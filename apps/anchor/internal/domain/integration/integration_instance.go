package integration

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/ids"
)

type Status string

const (
	StatusActive      Status = "ACTIVE"
	StatusInactive    Status = "INACTIVE"
	StatusConfiguring Status = "CONFIGURING"
	StatusError       Status = "ERROR"
)

const DefaultConfiguringReason = "Not configured yet. Complete provider setup before enabling ingestion."

type ProviderType string

const (
	ProviderTypeClerk ProviderType = "CLERK"
	ProviderTypeSMTP  ProviderType = "SMTP"
)

type Instance struct {
	ID               string
	PlatformTenantID string
	ProductID        string
	ProviderType     ProviderType
	WebhookSecret    *string
	ConfigJSON       json.RawMessage
	ConfigVersion    int32
	IsEnabled        bool
	LastError        *string
	Status           Status
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// CanIngest reports whether webhook ingestion and downstream resource updates are
// allowed for this integration instance under the current lifecycle state.
func (i *Instance) CanIngest() bool {
	if !i.IsEnabled {
		return false
	}

	if i.WebhookSecret == nil || strings.TrimSpace(*i.WebhookSecret) == "" {
		return false
	}

	switch i.Status {
	case StatusConfiguring, StatusInactive, StatusError:
		return false
	case StatusActive:
		return true
	default:
		return false
	}
}

// GenerateID sets the instance's ID to a new prefixed KSUID.
func (i *Instance) GenerateID() {
	i.ID = ids.MustNew("iin")
}

// IngestionBlockReason provides a stable reason string when webhook processing
// is blocked by lifecycle state.
func (i *Instance) IngestionBlockReason() string {
	if !i.IsEnabled {
		return "disabled"
	}

	if i.WebhookSecret == nil || strings.TrimSpace(*i.WebhookSecret) == "" {
		return "missing_webhook_secret"
	}

	switch i.Status {
	case StatusActive:
		return "active"
	case StatusConfiguring:
		return "configuring"
	case StatusInactive:
		return "inactive"
	case StatusError:
		if i.LastError != nil && *i.LastError != "" {
			return *i.LastError
		}
		return "error"
	default:
		return "unknown_state"
	}
}
