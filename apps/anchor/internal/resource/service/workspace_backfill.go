package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/pglock"
	"github.com/rs/zerolog"
	"go.uber.org/fx"

	"anchor/internal/domain/resourcetype"
	"anchor/internal/domain/roleassignment"
	"anchor/internal/domain/workspace"
	orgrepository "anchor/internal/repository"
	"anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
	assignments "anchor/internal/roleassignment/service"
)

const workspaceBackfillLockKey = "resources:workspace-backfill"

// WriteReplay replays the relationship-store marks of a write made outside a
// request once its transaction commits, as a request's replay does (ADR-0022
// E5). The authz module provides it.
type WriteReplay interface {
	// Collect returns a context that collects the marks of the writes made
	// in it, and the function that replays them after the commit.
	Collect(ctx context.Context) (context.Context, func(context.Context))
}

// WorkspaceBackfill runs ADR-0022 A6 phase 2: each product with workspaces
// gets a workspace type, each workspace a marked resource with its ID and
// name under its Organization, and each workspace_memberships row whose role
// is scoped to that type becomes a role assignment. It also deletes a marked
// resource whose workspace an earlier release deleted. It only fills what is
// missing, so it is safe on every start, and a resource the resource routes
// renamed, moved or gave roles to keeps them.
type WorkspaceBackfill struct {
	workspaceRepo orgrepository.WorkspaceRepository
	workspaces    Workspaces
	resources     Service
	repo          repository.Repository
	resourceTypes rtservice.Service
	assignments   assignments.Service
	replay        WriteReplay
	transactor    transactor.Transactor
	logger        zerolog.Logger
}

type WorkspaceBackfillParams struct {
	fx.In
	WorkspaceRepo orgrepository.WorkspaceRepository
	Workspaces    Workspaces
	Resources     Service
	Repo          repository.Repository
	ResourceTypes rtservice.Service
	Assignments   assignments.Service
	Replay        WriteReplay
	Transactor    transactor.Transactor
	Logger        zerolog.Logger
}

func NewWorkspaceBackfill(p WorkspaceBackfillParams) *WorkspaceBackfill {
	return &WorkspaceBackfill{
		workspaceRepo: p.WorkspaceRepo,
		workspaces:    p.Workspaces,
		resources:     p.Resources,
		repo:          p.Repo,
		resourceTypes: p.ResourceTypes,
		assignments:   p.Assignments,
		replay:        p.Replay,
		transactor:    p.Transactor,
		logger:        p.Logger.With().Str("component", "workspace_backfill").Logger(),
	}
}

type workspaceBackfillParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Lock      *pglock.Client
	Backfill  *WorkspaceBackfill
}

// RegisterWorkspaceBackfill runs the backfill on every start, once across
// replicas, beside the start so it never holds it up. A workspace an earlier
// release writes during a rolling deploy is caught by the next start.
func RegisterWorkspaceBackfill(p workspaceBackfillParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			runCtx := context.WithoutCancel(ctx)
			go func() {
				acquired, err := p.Lock.TryWithLock(runCtx, workspaceBackfillLockKey,
					func(lockCtx context.Context, _ *sql.Tx) error {
						return p.Backfill.Run(lockCtx)
					})
				if err != nil {
					p.Backfill.logger.Error().Err(err).Msg("workspace backfill failed")
					return
				}
				if !acquired {
					p.Backfill.logger.Debug().Msg("workspace backfill skipped: lock not acquired")
				}
			}()
			return nil
		},
	})
}

// Run migrates every Organization the work list names. A product or an
// Organization that fails is logged, and the others still run.
func (b *WorkspaceBackfill) Run(ctx context.Context) error {
	return b.run(ctx, "")
}

// RunProduct migrates one product; tests call it directly.
func (b *WorkspaceBackfill) RunProduct(ctx context.Context, productID string) error {
	if productID == "" {
		return errors.New("workspace backfill: no product")
	}
	return b.run(ctx, productID)
}

func (b *WorkspaceBackfill) run(ctx context.Context, productID string) error {
	pending, err := b.repo.PendingWorkspaceOrganizations(ctx, productID)
	if err != nil {
		return err
	}
	byProduct := map[string][]string{}
	var products []string
	for _, row := range pending {
		if _, seen := byProduct[row.ProductID]; !seen {
			products = append(products, row.ProductID)
		}
		byProduct[row.ProductID] = append(byProduct[row.ProductID], row.OrganizationID)
	}
	var failed []error
	for _, id := range products {
		if productErr := b.runProduct(ctx, id, byProduct[id]); productErr != nil {
			failed = append(failed, productErr)
		}
	}
	return errors.Join(failed...)
}

// workspaceTally counts what one Organization's migration did.
type workspaceTally struct {
	registered, pruned, moved, left int
}

func (t *workspaceTally) add(other workspaceTally) {
	t.registered += other.registered
	t.pruned += other.pruned
	t.moved += other.moved
	t.left += other.left
}

func (b *WorkspaceBackfill) runProduct(ctx context.Context, productID string, organizationIDs []string) error {
	declared, err := b.workspaces.DeclareType(ctx, productID)
	if err != nil {
		b.logger.Error().Err(err).Str("product_id", productID).Msg("workspace backfill of a product failed")
		return err
	}
	if !declared {
		return nil
	}
	var total workspaceTally
	var failed []error
	for _, organizationID := range organizationIDs {
		tally, orgErr := b.runOrganization(ctx, productID, organizationID)
		if orgErr != nil {
			b.logger.Error().Err(orgErr).Str("product_id", productID).Str("organization_id", organizationID).
				Msg("workspace backfill of an organization failed")
			failed = append(failed, orgErr)
			continue
		}
		total.add(tally)
	}
	if total.registered+total.pruned+total.moved > 0 {
		b.logger.Info().Str("product_id", productID).Int("registered", total.registered).
			Int("pruned", total.pruned).Int("memberships_moved", total.moved).
			Msg("workspaces migrated into the workspace resource type")
	}
	if total.left > 0 {
		b.logger.Warn().Str("product_id", productID).Int("memberships_left", total.left).
			Msg("workspace memberships left in workspace_memberships: their roles aren't scoped to workspace")
	}
	return errors.Join(failed...)
}

// runOrganization migrates one Organization in one transaction, then replays
// its marks, so the store holds its resources before a decision reads them.
// It takes the locks of a resource write, in the same order (the
// Organization, the catalog, the Organization's resources), and reads the
// workspaces only after them: a workspace write waits for it, or it sees what
// that write committed.
func (b *WorkspaceBackfill) runOrganization(
	ctx context.Context, productID, organizationID string,
) (workspaceTally, error) {
	var tally workspaceTally
	collectCtx, replay := b.replay.Collect(ctx)
	err := b.transactor.InTx(collectCtx, func(txCtx context.Context) error {
		tally = workspaceTally{}
		exists, err := b.repo.OrganizationExists(transactor.ForKeyShare(txCtx), productID, organizationID)
		if err != nil || !exists {
			return err
		}
		tree, err := b.resourceTypes.LockedTree(txCtx, productID)
		if err != nil {
			return err
		}
		rt, declared := tree.BySlug(resourcetype.WorkspaceSlug).Get()
		if !declared {
			return nil
		}
		if err = b.repo.LockOrganization(txCtx, organizationID); err != nil {
			return err
		}
		if tally.pruned, err = b.prune(txCtx, productID, organizationID); err != nil {
			return err
		}
		if tally.registered, err = b.register(txCtx, productID, organizationID, rt.ID); err != nil {
			return err
		}
		memberships, err := b.workspaceRepo.ListMemberships(txCtx, productID, organizationID)
		if err != nil {
			return err
		}
		for _, membership := range memberships {
			moved, moveErr := b.moveMembership(txCtx, productID, organizationID, membership)
			if moveErr != nil {
				return moveErr
			}
			if moved {
				tally.moved++
			} else {
				tally.left++
			}
		}
		return nil
	})
	if err == nil {
		replay(ctx)
	}
	return tally, err
}

// prune deletes each marked resource whose workspace an earlier release
// deleted, with its subtree and the events of its role assignments.
func (b *WorkspaceBackfill) prune(txCtx context.Context, productID, organizationID string) (int, error) {
	deleted, err := b.repo.DeletedWorkspaces(txCtx, organizationID)
	if err != nil {
		return 0, err
	}
	for _, workspaceID := range deleted {
		if err = b.resources.Delete(
			txCtx, productID, organizationID, resourcetype.WorkspaceSlug, workspaceID, true,
		); err != nil {
			return 0, err
		}
	}
	return len(deleted), nil
}

// register marks the resources a workspace exists for, then registers each
// workspace no resource of the type stands for yet.
func (b *WorkspaceBackfill) register(txCtx context.Context, productID, organizationID, typeID string) (int, error) {
	if err := b.repo.MarkRegisteredWorkspaces(txCtx, organizationID, typeID); err != nil {
		return 0, err
	}
	missing, err := b.repo.UnregisteredWorkspaces(txCtx, organizationID, typeID)
	if err != nil {
		return 0, err
	}
	for _, ws := range missing {
		if err = b.workspaces.Put(txCtx, productID, organizationID, ws.ID, ws.Name); err != nil {
			return 0, err
		}
	}
	return len(missing), nil
}

// moveMembership turns a workspace_memberships row into a role assignment on
// the workspace's resource, and removes the row, so the next start never
// assigns it again. A row whose role is not scoped to the workspace type, or
// that the assignment rules refuse otherwise, stays: workspace roles have
// been Organization-scoped (ADR-0022 G1), and a role assigned on a resource
// must be scoped to its type (D4). So does a row for a member who already
// holds a role there: that assignment is newer. Each kept row logs at debug;
// the product logs their count.
func (b *WorkspaceBackfill) moveMembership(
	txCtx context.Context, productID, organizationID string, membership workspace.Membership,
) (bool, error) {
	target := roleassignment.Target{
		ProductID:      productID,
		OrganizationID: organizationID,
		ResourceType:   resourcetype.WorkspaceSlug,
		ResourceID:     membership.WorkspaceID,
		ProductUserID:  membership.ProductUserID,
	}
	leave := func(code string) (bool, error) {
		b.logger.Debug().Str("product_id", productID).Str("organization_id", organizationID).
			Str("workspace_id", membership.WorkspaceID).Str("product_user_id", membership.ProductUserID).
			Str("role_id", membership.RoleID).Str("code", code).
			Msg("workspace membership left in workspace_memberships")
		return false, nil
	}
	_, err := b.assignments.Get(txCtx, target)
	if err == nil {
		return leave("ROLE_ALREADY_ASSIGNED")
	}
	if _, refused := refusal(err); !refused {
		return false, err
	}
	_, err = b.assignments.Put(txCtx, roleassignment.PutInput{Target: target, RoleIDs: []string{membership.RoleID}})
	if refused, ok := refusal(err); ok {
		return leave(codeOf(refused))
	}
	if err != nil {
		return false, err
	}
	return true, b.workspaceRepo.DeleteMembership(txCtx, productID, organizationID, membership)
}
