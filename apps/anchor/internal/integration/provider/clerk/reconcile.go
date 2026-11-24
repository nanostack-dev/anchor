package clerk

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	clerkapi "github.com/clerk/clerk-sdk-go/v2"
	"github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/integration/provider"
)

const (
	clerkUsersOrderBy               = "+created_at"
	reconcileDeleteMinCap           = int64(25)
	reconcileDeleteCapDivisor       = int64(10)
	reconcileMaxConfirmationsPerRun = 50
)

// ReconcileResult holds the full user snapshot fetched from Clerk along with
// the timestamp taken before page 1 so downstream deletion diffing only
// considers rows synced before the snapshot began.
type ReconcileResult struct {
	Upserts       []provider.Command
	SeenIDs       map[string]struct{}
	SnapshotStart time.Time
}

// ReconcileDeletionPlan is the output of diffing Clerk-stamped product users
// against a complete reconcile snapshot and confirming missing users via 404.
type ReconcileDeletionPlan struct {
	Deletes        []provider.Command
	AbortedReason  string
	AbortRun       bool
	StampedCount   int64
	CandidateCount int
	ConfirmedCount int
	DeferredCount  int
}

// Reconcile fetches all users from Clerk and emits UPSERT_USER commands plus
// the complete set of seen external IDs. Any page error or early empty page
// fails the run so callers never run partial deletion diffs.
func (p *Provider) Reconcile(
	ctx context.Context,
	configJSON []byte,
) (ReconcileResult, error) {
	cfg, err := p.resolveConfig(configJSON)
	if err != nil {
		return ReconcileResult{}, err
	}

	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		p.logger.InfoContext(ctx, "clerk api key not configured, reconciliation skipped")
		return ReconcileResult{}, nil
	}

	snapshotStart := time.Now().UTC()
	users, err := p.fetchAllUsers(ctx, apiKey)
	if err != nil {
		return ReconcileResult{}, err
	}

	seen := make(map[string]struct{}, len(users))
	commands := make([]provider.Command, 0, len(users))
	for _, u := range users {
		if u == nil || strings.TrimSpace(u.ID) == "" {
			continue
		}
		seen[u.ID] = struct{}{}
		email, verified := extractPrimaryEmailFromSDKUser(u)
		var eventAt *time.Time
		if u.UpdatedAt > 0 {
			t := time.UnixMilli(u.UpdatedAt).UTC()
			eventAt = &t
		}
		commands = append(commands, provider.Command{
			Type: CommandUpsertUser,
			Data: UpsertUserData{
				ExternalID:    u.ID,
				Email:         email,
				EmailVerified: verified,
				Name:          buildFullName(u.FirstName, u.LastName),
				EventAt:       eventAt,
				Source:        SyncSourceReconcile,
			},
		})
	}

	return ReconcileResult{
		Upserts:       commands,
		SeenIDs:       seen,
		SnapshotStart: snapshotStart,
	}, nil
}

// PlanReconcileDeletions compares Clerk-stamped product users created and
// last synced before res.SnapshotStart against res.SeenIDs, applies safety
// caps against misconfiguration or mass deletion, and confirms each candidate
// with GET /v1/users/{id} requiring an explicit HTTP 404 before emitting a
// DELETE_USER command.
func (p *Provider) PlanReconcileDeletions(
	ctx context.Context,
	productID string,
	configJSON []byte,
	res ReconcileResult,
) (ReconcileDeletionPlan, error) {
	if p.productUserRepo == nil || res.SnapshotStart.IsZero() {
		return ReconcileDeletionPlan{}, nil
	}

	cfg, err := p.resolveConfig(configJSON)
	if err != nil {
		return ReconcileDeletionPlan{}, err
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return ReconcileDeletionPlan{}, nil
	}

	stampedCount, err := p.productUserRepo.CountClerkStampedUsers(ctx, productID)
	if err != nil {
		return ReconcileDeletionPlan{}, fmt.Errorf("count clerk stamped users: %w", err)
	}
	if stampedCount == 0 {
		return ReconcileDeletionPlan{}, nil
	}

	// Guard 1: an empty Clerk user list while stamped users exist almost
	// certainly indicates a wrong API key or different Clerk instance. Abort the
	// entire run (deletes and upserts) so nothing mutates against the wrong app.
	if len(res.SeenIDs) == 0 {
		return ReconcileDeletionPlan{
			AbortedReason: "empty_clerk_user_list",
			AbortRun:      true,
			StampedCount:  stampedCount,
		}, nil
	}

	beforeIDs, err := p.productUserRepo.ListClerkStampedExternalIDsBefore(ctx, productID, res.SnapshotStart)
	if err != nil {
		return ReconcileDeletionPlan{}, fmt.Errorf("list clerk deletion candidates: %w", err)
	}

	candidates := make([]string, 0)
	overlap := 0
	for _, id := range beforeIDs {
		if _, ok := res.SeenIDs[id]; ok {
			overlap++
		} else {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return ReconcileDeletionPlan{StampedCount: stampedCount}, nil
	}

	// Guard 2: if none of the previously synced users appear in Clerk's list,
	// the API key likely points to a different Clerk application (e.g. a dev/prod
	// key swap on a small product below the absolute volume cap). Abort the
	// entire run (including upserts) so wrong-app users are not inserted and
	// cannot create false overlap on the next reconcile tick.
	if len(beforeIDs) > 0 && overlap == 0 {
		return ReconcileDeletionPlan{
			AbortedReason:  "no_overlap_with_existing_users",
			AbortRun:       true,
			StampedCount:   stampedCount,
			CandidateCount: len(candidates),
		}, nil
	}

	// Guard 3: abort all deletions if the candidate set exceeds max(25, 10% of stamped users).
	volumeCap := max(reconcileDeleteMinCap, stampedCount/reconcileDeleteCapDivisor)
	if int64(len(candidates)) > volumeCap {
		return ReconcileDeletionPlan{
			AbortedReason:  "deletion_volume_cap_exceeded",
			StampedCount:   stampedCount,
			CandidateCount: len(candidates),
		}, nil
	}

	toCheck := candidates
	deferred := 0
	if len(toCheck) > reconcileMaxConfirmationsPerRun {
		deferred = len(toCheck) - reconcileMaxConfirmationsPerRun
		toCheck = toCheck[:reconcileMaxConfirmationsPerRun]
	}

	deletes := p.confirmDeletedUsers(ctx, apiKey, toCheck)
	return ReconcileDeletionPlan{
		Deletes:        deletes,
		StampedCount:   stampedCount,
		CandidateCount: len(candidates),
		ConfirmedCount: len(deletes),
		DeferredCount:  deferred,
	}, nil
}

func (p *Provider) confirmDeletedUsers(
	ctx context.Context,
	apiKey string,
	candidateIDs []string,
) []provider.Command {
	usersClient := p.newClerkUsersClient(apiKey)
	deletes := make([]provider.Command, 0, len(candidateIDs))
	for _, externalID := range candidateIDs {
		_, err := usersClient.Get(ctx, externalID)
		if err == nil {
			// User still exists in Clerk (e.g. skipped across offset pages).
			continue
		}
		if apiErr, ok := errors.AsType[*clerkapi.APIErrorResponse](
			err,
		); ok &&
			apiErr.HTTPStatusCode == http.StatusNotFound {
			confirmedAt := time.Now().UTC()
			deletes = append(deletes, provider.Command{
				Type: CommandDeleteUser,
				Data: DeleteUserData{
					ExternalID: externalID,
					EventAt:    &confirmedAt,
				},
			})
			continue
		}
		// Any 5xx, 429, or network timeout leaves the user untouched.
		p.logger.WarnContext(ctx, "clerk deletion confirmation did not return 404; keeping user",
			slog.String(clerkExternalIDKey, externalID),
			log.Err(err),
		)
	}
	return deletes
}

func (p *Provider) newClerkUsersClient(apiKey string) *user.Client {
	return user.NewClient(&clerkapi.ClientConfig{
		Key:        &apiKey,
		HTTPClient: p.httpClient,
		URL:        &p.baseURL,
	})
}

func (p *Provider) fetchAllUsers(ctx context.Context, apiKey string) ([]*clerkapi.User, error) {
	usersListClient := p.newClerkUsersClient(apiKey)

	users := make([]*clerkapi.User, 0)
	offset := int64(0)
	limit := int64(clerkUsersPageSize)
	orderBy := clerkUsersOrderBy

	for {
		params := &user.ListParams{OrderBy: &orderBy}
		params.Limit = &limit
		params.Offset = &offset
		page, err := usersListClient.List(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("failed to list clerk users: %w", err)
		}

		if len(page.Users) == 0 {
			if offset < page.TotalCount {
				return nil, fmt.Errorf(
					"clerk users list returned empty page early at offset %d of %d",
					offset, page.TotalCount,
				)
			}
			break
		}

		users = append(users, page.Users...)
		offset += int64(len(page.Users))

		if offset >= page.TotalCount {
			break
		}
	}

	return users, nil
}

func extractPrimaryEmailFromSDKUser(u *clerkapi.User) (string, bool) {
	if u == nil {
		return "", false
	}

	if u.PrimaryEmailAddressID != nil {
		primary := functional.Slice(u.EmailAddresses).FindFirst(func(ea *clerkapi.EmailAddress) bool {
			return ea != nil && ea.ID == *u.PrimaryEmailAddressID
		})
		if primary.IsPresent() {
			ea := primary.Value()
			return ea.EmailAddress, ea.Verification != nil && ea.Verification.Status == clerkEmailVerifiedStatus
		}
	}

	if len(u.EmailAddresses) > 0 && u.EmailAddresses[0] != nil {
		ea := u.EmailAddresses[0]
		return ea.EmailAddress, ea.Verification != nil && ea.Verification.Status == clerkEmailVerifiedStatus
	}

	return "", false
}
