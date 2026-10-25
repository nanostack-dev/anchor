// Package service resolves the role a request names inside an Organization
// and keeps every Organization's copy of a role template in step with it.
package service

import (
	"context"

	"anchor/internal/domain/organizationrole"
	"anchor/internal/organizationrole/repository"
)

type Service interface {
	// Resolve finds the role a request body named inside an Organization:
	// its copy of a template when it has one, or else the template itself.
	Resolve(txCtx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error)
	// SyncTemplate brings every copy of a template in step with it.
	SyncTemplate(txCtx context.Context, productID, templateID string) error
}

type service struct {
	repo repository.Repository
}

func NewService(repo repository.Repository) Service {
	return &service{repo: repo}
}

func (s *service) Resolve(
	txCtx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	found, err := s.repo.Resolve(txCtx, productID, organizationID, roleID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	return found.ToResult(organizationrole.NewBodyRoleNotFoundError(roleID)).Value()
}

func (s *service) SyncTemplate(txCtx context.Context, productID, templateID string) error {
	return s.repo.SyncCopies(txCtx, productID, templateID)
}
