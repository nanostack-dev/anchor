package ct_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	authzstore "anchor/internal/authz/store"
)

const (
	syncCheckWorkers = 8
	syncShownLimit   = 10
	syncUnknownStore = -1
)

var authzStores authzstore.Stores

// holder is a member or an active, unexpired Organization API key, and what
// Anchor's tables say it holds.
type holder struct {
	organizationID string
	subject        string
	held           []string
}

// checkAuthzSync compares, for every Product the suite gave a rebuilt store,
// what each member and API key holds in Anchor's tables with OpenFGA's answer
// for every permission of the Product. It runs before any sweep, so it checks
// what the writes' own replays left. Decisions come from OpenFGA (ADR-0022 G4,
// PR 7), and most writes in the suite are never followed by one, so this is
// where their drift would show.
func checkAuthzSync(ctx context.Context) error {
	byProduct, err := syncHolders(ctx)
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
	group.SetLimit(syncCheckWorkers)
	for productID, holders := range byProduct {
		group.Go(func() error {
			found, count, productErr := compareProduct(groupCtx, productID, holders)
			mu.Lock()
			defer mu.Unlock()
			differences = append(differences, found...)
			if count == syncUnknownStore {
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
			len(differences), strings.Join(differences[:min(len(differences), syncShownLimit)], "\n"))
	}
	if compared == 0 {
		return errors.New("the sync check compared no permission")
	}
	fmt.Fprintf(os.Stderr, "authz sync: %d permissions agree across %d products (%d without a model)\n",
		compared, len(byProduct)-skipped, skipped)
	return nil
}

// syncHolders reads every member's role permissions and every active key's
// permissions, by Product.
func syncHolders(ctx context.Context) (map[string][]holder, error) {
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
	found, err := authzStores.Open(ctx, productID)
	if err != nil {
		return nil, 0, err
	}
	opened, ok := found.Get()
	if !ok {
		return nil, syncUnknownStore, nil
	}
	permissions, err := productPermissions(ctx, productID)
	if err != nil {
		return nil, 0, err
	}
	var differences []string
	compared := 0
	for _, h := range holders {
		checks := make([]fga.Tuple, len(permissions))
		for i, permission := range permissions {
			checks[i] = fga.Tuple{
				Object: model.Organization(h.organizationID), Relation: model.CanRelation(permission), User: h.subject,
			}
		}
		answers, checkErr := opened.Holds(ctx, checks)
		if checkErr != nil {
			return nil, 0, fmt.Errorf("sync check for product %s: %w", productID, checkErr)
		}
		for i, permission := range permissions {
			compared++
			want := slices.Contains(h.held, permission)
			if answers[i] != want {
				differences = append(differences, fmt.Sprintf(
					"  product %s, organization %s, %s, %s: tables %t, OpenFGA %t",
					productID, h.organizationID, h.subject, permission, want, answers[i]))
			}
		}
	}
	return differences, compared, nil
}

func productPermissions(ctx context.Context, productID string) ([]string, error) {
	rows, err := testDB.QueryContext(ctx, `SELECT name FROM product_permissions WHERE product_id = $1`, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}
