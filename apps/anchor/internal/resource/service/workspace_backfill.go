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

// WorkspaceBackfill runs ADR-0022 A6 phase 2: each product with workspaces
// gets a workspace type, each workspace a reference with its ID and name
// under its Organization, and each workspace_memberships row whose role is
// scoped to that type becomes a role assignment. It only fills what is
// missing, so it is safe on every start, and a reference the resource routes
// renamed, moved or gave roles to keeps them.
type WorkspaceBackfill struct {
	workspaceRepo orgrepository.WorkspaceRepository
	workspaces    Workspaces
	resources     repository.Repository
	resourceTypes rtservice.Service
	assignments   assignments.Service
	transactor    transactor.Transactor
	logger        zerolog.Logger
}

func NewWorkspaceBackfill(
	workspaceRepo orgrepository.WorkspaceRepository,
	workspaces Workspaces,
	resources repository.Repository,
	resourceTypes rtservice.Service,
	roleAssignments assignments.Service,
	tx transactor.Transactor,
	logger zerolog.Logger,
) *WorkspaceBackfill {
	return &WorkspaceBackfill{
		workspaceRepo: workspaceRepo,
		workspaces:    workspaces,
		resources:     resources,
		resourceTypes: resourceTypes,
		assignments:   roleAssignments,
		transactor:    tx,
		logger:        logger.With().Str("component", "workspace_backfill").Logger(),
	}
}

type workspaceBackfillParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Lock      *pglock.Client
	Backfill  *WorkspaceBackfill
}

// RegisterWorkspaceBackfill runs the backfill on every start, once across
// replicas. A workspace an earlier release writes during a rolling deploy is
// caught by the next start.
func RegisterWorkspaceBackfill(p workspaceBackfillParams) {
	p.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			acquired, err := p.Lock.TryWithLock(ctx, workspaceBackfillLockKey,
				func(lockCtx context.Context, _ *sql.Tx) error {
					return p.Backfill.Run(lockCtx)
				})
			if err != nil {
				p.Backfill.logger.Error().Err(err).Msg("workspace backfill failed")
				return nil
			}
			if !acquired {
				p.Backfill.logger.Debug().Msg("workspace backfill skipped: lock not acquired")
			}
			return nil
		},
	})
}

// Run migrates every product that has a workspace. A product that fails is
// logged, and the others still run.
func (b *WorkspaceBackfill) Run(ctx context.Context) error {
	productIDs, err := b.workspaceRepo.ProductIDsInternal(ctx)
	if err != nil {
		return err
	}
	var failed []error
	for _, productID := range productIDs {
		if err = b.RunProduct(ctx, productID); err != nil {
			b.logger.Error().Err(err).Str("product_id", productID).Msg("workspace backfill of a product failed")
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}

// workspaceTally counts what one Organization's migration did.
type workspaceTally struct {
	registered, moved, left int
}

// RunProduct migrates one product; tests call it directly.
func (b *WorkspaceBackfill) RunProduct(ctx context.Context, productID string) error {
	declared, err := b.workspaces.DeclareType(ctx, productID)
	if err != nil || !declared {
		return err
	}
	organizationIDs, err := b.workspaceRepo.OrganizationIDs(ctx, productID)
	if err != nil {
		return err
	}
	var total workspaceTally
	for _, organizationID := range organizationIDs {
		tally, orgErr := b.runOrganization(ctx, productID, organizationID)
		if orgErr != nil {
			return orgErr
		}
		total.registered += tally.registered
		total.moved += tally.moved
		total.left += tally.left
	}
	if total != (workspaceTally{}) {
		b.logger.Info().Str("product_id", productID).Int("registered", total.registered).
			Int("memberships_moved", total.moved).Int("memberships_left", total.left).
			Msg("workspaces migrated into the workspace resource type")
	}
	return nil
}

// runOrganization registers the Organization's missing references and moves
// its memberships in one transaction. It takes the locks of a resource write,
// in the same order (the Organization, the catalog, the Organization's
// resources), and reads the workspaces only after them: a workspace write
// waits for it, or it sees what that write committed.
func (b *WorkspaceBackfill) runOrganization(
	ctx context.Context, productID, organizationID string,
) (workspaceTally, error) {
	var tally workspaceTally
	err := b.transactor.InTx(ctx, func(txCtx context.Context) error {
		tally = workspaceTally{}
		exists, err := b.resources.HoldOrganization(txCtx, productID, organizationID)
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
		if err = b.resources.LockOrganization(txCtx, organizationID); err != nil {
			return err
		}
		byOrganization, err := b.workspaceRepo.ListByOrganizations(txCtx, productID, []string{organizationID})
		if err != nil {
			return err
		}
		for _, ws := range byOrganization[organizationID] {
			found, findErr := b.resources.Find(txCtx, organizationID, rt.ID, ws.ID)
			if findErr != nil {
				return findErr
			}
			if found.IsPresent() {
				continue
			}
			if err = b.workspaces.Put(txCtx, productID, organizationID, ws.ID, ws.Name); err != nil {
				return err
			}
			tally.registered++
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
	return tally, err
}

// moveMembership turns a workspace_memberships row into a role assignment on
// the workspace's reference, and removes the row, so the next start never
// assigns it again. A row whose role is not scoped to the workspace type, or
// that the assignment rules refuse otherwise, stays and is logged: workspace
// roles have been Organization-scoped (ADR-0022 G1), and a role assigned on a
// resource must be scoped to its type (D4). So does a row for a member who
// already holds a role there: that assignment is newer.
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
		b.logger.Warn().Str("product_id", productID).Str("organization_id", organizationID).
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
