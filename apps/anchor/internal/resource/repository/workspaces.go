package repository

import (
	"context"
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"

	"anchor/internal/domain/resourcetype"
	"anchor/internal/domain/workspace"
)

// The workspace mirror (ADR-0022 A6, phase 2) reads the built-in workspaces
// beside the resources that mirror them, until both go in the cleanup (G6).
// A mirrored resource is marked in workspace_resources and named by its
// workspace's ID.

// WorkspaceMirrors is the part of the registry the workspace mirror uses.
type WorkspaceMirrors interface {
	// MarkWorkspace marks the resource as the mirror of its workspace.
	MarkWorkspace(ctx context.Context, rowID string) error
	// MarkRegisteredWorkspaces marks every resource of the type in the
	// Organization that a workspace of the same ID exists for.
	MarkRegisteredWorkspaces(ctx context.Context, organizationID, typeID string) error
	// MirrorsDeletedWorkspace reports whether the resource is marked and its
	// workspace is gone.
	MirrorsDeletedWorkspace(ctx context.Context, rowID string) (bool, error)
	// DeletedWorkspaces lists the workspace IDs of the Organization's marked
	// resources whose workspace is gone.
	DeletedWorkspaces(ctx context.Context, organizationID string) ([]string, error)
	// UnregisteredWorkspaces lists the Organization's workspaces that no
	// resource of the type mirrors yet, in one anti-join.
	UnregisteredWorkspaces(ctx context.Context, organizationID, typeID string) ([]workspace.Workspace, error)
	// PendingWorkspaceOrganizations lists the Organizations that the
	// migration has work in: a workspace with no marked resource, a marked
	// resource whose workspace is gone, or a workspace_memberships row. An
	// empty productID lists them across products.
	PendingWorkspaceOrganizations(ctx context.Context, productID string) ([]WorkspaceOrganization, error)
}

// WorkspaceOrganization is an Organization of a Product.
type WorkspaceOrganization struct {
	ProductID      string
	OrganizationID string
}

func (r *repositoryImpl) MarkWorkspace(ctx context.Context, rowID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx,
		`INSERT INTO workspace_resources (resource_id) VALUES ($1) ON CONFLICT DO NOTHING`, rowID)
	return err
}

func (r *repositoryImpl) MarkRegisteredWorkspaces(ctx context.Context, organizationID, typeID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx, `
INSERT INTO workspace_resources (resource_id)
SELECT r.id FROM resources r
JOIN workspaces w ON w.id = r.external_id AND w.organization_id = r.organization_id
WHERE r.organization_id = $1 AND r.resource_type_id = $2
ON CONFLICT DO NOTHING`, organizationID, typeID)
	return err
}

func (r *repositoryImpl) MirrorsDeletedWorkspace(ctx context.Context, rowID string) (bool, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, `
SELECT EXISTS (
    SELECT 1 FROM workspace_resources m
    JOIN resources r ON r.id = m.resource_id
    WHERE m.resource_id = $1 AND NOT EXISTS (
        SELECT 1 FROM workspaces w WHERE w.id = r.external_id AND w.organization_id = r.organization_id
    )
)`, rowID)
	if err != nil {
		return false, fmt.Errorf("read workspace mirror: %w", err)
	}
	defer func() { _ = rows.Close() }()
	gone := false
	if rows.Next() {
		if err = rows.Scan(&gone); err != nil {
			return false, err
		}
	}
	return gone, rows.Err()
}

func (r *repositoryImpl) DeletedWorkspaces(ctx context.Context, organizationID string) ([]string, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, `
SELECT r.external_id FROM workspace_resources m
JOIN resources r ON r.id = m.resource_id
WHERE r.organization_id = $1 AND NOT EXISTS (
    SELECT 1 FROM workspaces w WHERE w.id = r.external_id AND w.organization_id = r.organization_id
)
ORDER BY r.external_id`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("read deleted workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *repositoryImpl) UnregisteredWorkspaces(
	ctx context.Context, organizationID, typeID string,
) ([]workspace.Workspace, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, `
SELECT w.id, w.name FROM workspaces w
WHERE w.organization_id = $1 AND NOT EXISTS (
    SELECT 1 FROM resources r
    WHERE r.organization_id = w.organization_id AND r.resource_type_id = $2 AND r.external_id = w.id
)
ORDER BY w.created_at, w.id`, organizationID, typeID)
	if err != nil {
		return nil, fmt.Errorf("read unregistered workspaces: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var found []workspace.Workspace
	for rows.Next() {
		ws := workspace.Workspace{OrganizationID: organizationID}
		if err = rows.Scan(&ws.ID, &ws.Name); err != nil {
			return nil, err
		}
		found = append(found, ws)
	}
	return found, rows.Err()
}

// pendingWorkspacesQuery is the migration's work list. Its first branch is the
// anti-join of every start: a workspace with a marked resource of the
// workspace type costs one index probe and adds nothing.
const pendingWorkspacesQuery = `
SELECT o.product_id, w.organization_id
FROM workspaces w
JOIN organizations o ON o.id = w.organization_id
WHERE ($1 = '' OR o.product_id = $1) AND NOT EXISTS (
    SELECT 1 FROM resources r
    JOIN product_resource_types t ON t.id = r.resource_type_id AND t.slug = $2
    JOIN workspace_resources m ON m.resource_id = r.id
    WHERE r.organization_id = w.organization_id AND r.external_id = w.id
)
UNION
SELECT r.product_id, r.organization_id
FROM workspace_resources m
JOIN resources r ON r.id = m.resource_id
WHERE ($1 = '' OR r.product_id = $1) AND NOT EXISTS (
    SELECT 1 FROM workspaces w WHERE w.id = r.external_id AND w.organization_id = r.organization_id
)
UNION
SELECT o.product_id, w.organization_id
FROM workspace_memberships wm
JOIN workspaces w ON w.id = wm.workspace_id
JOIN organizations o ON o.id = w.organization_id
WHERE ($1 = '' OR o.product_id = $1)
ORDER BY 1, 2`

func (r *repositoryImpl) PendingWorkspaceOrganizations(
	ctx context.Context, productID string,
) ([]WorkspaceOrganization, error) {
	rows, err := transactor.Executor(ctx, r.db).QueryContext(ctx, pendingWorkspacesQuery,
		productID, resourcetype.WorkspaceSlug)
	if err != nil {
		return nil, fmt.Errorf("read pending workspace organizations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var pending []WorkspaceOrganization
	for rows.Next() {
		var row WorkspaceOrganization
		if err = rows.Scan(&row.ProductID, &row.OrganizationID); err != nil {
			return nil, err
		}
		pending = append(pending, row)
	}
	return pending, rows.Err()
}
