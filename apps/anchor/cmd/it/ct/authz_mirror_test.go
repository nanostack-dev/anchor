package ct_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"golang.org/x/sync/errgroup"

	authzengine "anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	authzrepository "anchor/internal/authz/repository"
	authzservice "anchor/internal/authz/service"
)

const (
	mirrorCheckBatch   = 50
	mirrorWorkers      = 8
	mirrorShownLimit   = 10
	mirrorUnknownStore = -1
)

var (
	authzEngines authzengine.Provider
	authzRepo    authzrepository.Repository
	authzSweeper *authzservice.Sweeper
)

// holder is a member or an active, unexpired Organization API key, and what
// Anchor's tables say it holds.
type holder struct {
	organizationID string
	subject        string
	held           []string
}

// checkAuthzMirror compares, for every Product the suite gave a model, what
// each member and API key holds in Anchor's tables with OpenFGA's answer for
// every permission of the Product. Decisions come from OpenFGA (ADR-0022 G4,
// PR 7), and most writes in the suite are never followed by one, so this is
// where their drift would show.
func checkAuthzMirror(ctx context.Context) error {
	authzSweeper.Sweep(ctx)
	byProduct, err := mirrorHolders(ctx)
	if err != nil {
		return err
	}
	var (
		mu          sync.Mutex
		differences []string
		compared    int
		skipped     int
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(mirrorWorkers)
	for productID, holders := range byProduct {
		group.Go(func() error {
			found, count, productErr := compareProduct(groupCtx, productID, holders)
			mu.Lock()
			defer mu.Unlock()
			differences = append(differences, found...)
			if count == mirrorUnknownStore {
				skipped++
			}
			compared += max(count, 0)
			return productErr
		})
	}
	if err = group.Wait(); err != nil {
		return err
	}
	if len(differences) > 0 {
		slices.Sort(differences)
		return fmt.Errorf("OpenFGA differs from Anchor's tables on %d permissions, first ones:\n%s",
			len(differences), strings.Join(differences[:min(len(differences), mirrorShownLimit)], "\n"))
	}
	if compared == 0 {
		return errors.New("the mirror check compared no permission")
	}
	fmt.Fprintf(os.Stderr, "authz mirror: %d permissions agree across %d products (%d without a model)\n",
		compared, len(byProduct)-skipped, skipped)
	return nil
}

// mirrorHolders reads every member's role permissions and every active key's
// permissions, by Product.
func mirrorHolders(ctx context.Context) (map[string][]holder, error) {
	byProduct := map[string][]holder{}
	members, err := testDB.QueryContext(ctx, `SELECT o.product_id, m.organization_id, m.product_user_id
		FROM organization_memberships m JOIN organizations o ON o.id = m.organization_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = members.Close() }()
	type member struct{ productID, organizationID, productUserID string }
	var all []member
	for members.Next() {
		var row member
		if err = members.Scan(&row.productID, &row.organizationID, &row.productUserID); err != nil {
			return nil, err
		}
		all = append(all, row)
	}
	if err = members.Err(); err != nil {
		return nil, err
	}
	for _, row := range all {
		found, findErr := OrgMemberRepo.FindByProductUserIDAndOrgID(
			ctx, row.productID, row.productUserID, row.organizationID, true)
		if findErr != nil {
			return nil, findErr
		}
		if membership, ok := found.Get(); ok {
			byProduct[row.productID] = append(byProduct[row.productID], holder{
				organizationID: row.organizationID, subject: model.User(row.productUserID),
				held: membership.RolePermissions,
			})
		}
	}
	keys, err := testDB.QueryContext(ctx, `SELECT o.product_id, k.organization_id, k.id,
			COALESCE(array_to_string(array_agg(p.permission_name) FILTER (WHERE p.permission_name IS NOT NULL), ','), '')
		FROM organization_api_keys k
		JOIN organizations o ON o.id = k.organization_id
		LEFT JOIN organization_api_key_permissions p ON p.api_key_id = k.id
		WHERE k.status = 'ACTIVE' AND (k.expires_at IS NULL OR k.expires_at > NOW())
		GROUP BY o.product_id, k.organization_id, k.id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = keys.Close() }()
	for keys.Next() {
		var productID, organizationID, keyID, permissions string
		if err = keys.Scan(&productID, &organizationID, &keyID, &permissions); err != nil {
			return nil, err
		}
		byProduct[productID] = append(byProduct[productID], holder{
			organizationID: organizationID, subject: model.APIKey(keyID),
			held: strings.FieldsFunc(permissions, func(r rune) bool { return r == ',' }),
		})
	}
	return byProduct, keys.Err()
}

// compareProduct answers every permission of the Product for each holder. A
// Product the suite never gave a model is skipped: its first decision builds
// it from the tables.
func compareProduct(ctx context.Context, productID string, holders []holder) ([]string, int, error) {
	found, err := authzRepo.FindStore(ctx, productID)
	if err != nil {
		return nil, 0, err
	}
	stored, ok := found.Get()
	if !ok || stored.ModelID == "" {
		return nil, mirrorUnknownStore, nil
	}
	scopes, err := authzRepo.Permissions(ctx, productID)
	if err != nil {
		return nil, 0, err
	}
	openfga, err := authzEngines.Get()
	if err != nil {
		return nil, 0, err
	}
	var differences []string
	compared := 0
	for _, h := range holders {
		for start := 0; start < len(scopes); start += mirrorCheckBatch {
			batch := scopes[start:min(start+mirrorCheckBatch, len(scopes))]
			checks := make([]*openfgav1.BatchCheckItem, len(batch))
			for i, scope := range batch {
				checks[i] = &openfgav1.BatchCheckItem{
					CorrelationId: strconv.Itoa(i),
					TupleKey: &openfgav1.CheckRequestTupleKey{
						User: h.subject, Relation: model.CanRelation(scope.Name),
						Object: model.Organization(h.organizationID),
					},
				}
			}
			resp, checkErr := openfga.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
				StoreId: stored.StoreID, AuthorizationModelId: stored.ModelID, Checks: checks,
			})
			if checkErr != nil {
				return nil, 0, fmt.Errorf("mirror check for product %s: %w", productID, checkErr)
			}
			for i, scope := range batch {
				result := resp.GetResult()[strconv.Itoa(i)]
				if result.GetError() != nil {
					return nil, 0, fmt.Errorf("mirror check for product %s, %s: %s",
						productID, scope.Name, result.GetError().GetMessage())
				}
				compared++
				want := slices.Contains(h.held, scope.Name)
				if result.GetAllowed() != want {
					differences = append(
						differences,
						fmt.Sprintf("  product %s, organization %s, %s, %s: tables %t, OpenFGA %t",
							productID, h.organizationID, h.subject, scope.Name, want, result.GetAllowed()),
					)
				}
			}
		}
	}
	return differences, compared, nil
}
