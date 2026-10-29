package repository

import (
	"context"
	"fmt"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
)

// The workspace mirror (ADR-0022 A6, phase 2) reads the built-in workspaces
// beside the resources that mirror them, until both go in the cleanup (G6).
// A mirrored resource is marked in workspace_resources and named by its
// workspace's ID.

// WorkspaceMirrors is the part of the registry the workspace mirror uses.
type WorkspaceMirrors interface {
	// MarkWorkspace marks the resource as the mirror of its workspace.
	MarkWorkspace(ctx context.Context, rowID string) error
	// MirrorsDeletedWorkspace reports whether the resource is marked and its
	// workspace is gone.
	MirrorsDeletedWorkspace(ctx context.Context, rowID string) (bool, error)
	// ChainMirrorsDeletedWorkspace reports whether the resource or any
	// resource above it is marked and its workspace is gone.
	ChainMirrorsDeletedWorkspace(ctx context.Context, organizationID, rowID string) (bool, error)
}

func (r *repositoryImpl) MarkWorkspace(ctx context.Context, rowID string) error {
	_, err := transactor.Executor(ctx, r.db).ExecContext(ctx,
		`INSERT INTO workspace_resources (resource_id) VALUES ($1) ON CONFLICT DO NOTHING`, rowID)
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

// chainMirrorsDeletedWorkspaceQuery walks up from the resource; a stored
// chain never loops and is at most resource.MaxDepth long.
const chainMirrorsDeletedWorkspaceQuery = `
WITH RECURSIVE chain AS (
    SELECT id, parent_resource_id, external_id, organization_id
    FROM resources
    WHERE id = $1 AND organization_id = $2
    UNION ALL
    SELECT r.id, r.parent_resource_id, r.external_id, r.organization_id
    FROM resources r
    JOIN chain ON r.id = chain.parent_resource_id
)
SELECT EXISTS (
    SELECT 1 FROM chain
    JOIN workspace_resources m ON m.resource_id = chain.id
    WHERE NOT EXISTS (
        SELECT 1 FROM workspaces w WHERE w.id = chain.external_id AND w.organization_id = chain.organization_id
    )
)`

func (r *repositoryImpl) ChainMirrorsDeletedWorkspace(ctx context.Context, organizationID, rowID string) (bool, error) {
	rows, err := transactor.Executor(ctx, r.db).
		QueryContext(ctx, chainMirrorsDeletedWorkspaceQuery, rowID, organizationID)
	if err != nil {
		return false, fmt.Errorf("read workspace mirrors above a resource: %w", err)
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
