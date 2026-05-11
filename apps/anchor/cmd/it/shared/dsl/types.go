package itdsl

import (
	nanostackClient "github.com/nanostack-dev/anchor/clients/go"
)

// Tenant is a DSL-level tenant runtime context.
type Tenant struct {
	ID           string
	OwnerUser    *PlatformUser
	OwnerClient  *nanostackClient.ClientWithResponses
	NoAuthClient *nanostackClient.ClientWithResponses
	ServerURL    string
}

// PlatformUser is a DSL-level platform user runtime context.
type PlatformUser struct {
	ID                  string
	UserID              string
	Email               string
	Password            string
	AccessToken         string
	RefreshToken        string
	AuthenticatedClient *nanostackClient.ClientWithResponses
	TenantAlias         string
}
