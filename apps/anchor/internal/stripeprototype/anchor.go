package stripeprototype

import (
	"context"
	"net"
	"strings"
)

type AnchorGateway interface {
	Snapshot(ctx context.Context) (AnchorSnapshot, error)
	ApplyTemplate(ctx context.Context, organizationID, templateID string) error
}

type AnchorSnapshot struct {
	Product       Product
	Templates     []Template
	Organizations []AnchorOrganization
}

type AnchorOrganization struct {
	ID            string
	Name          string
	TemplateID    string
	LicenseValues map[string]any
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func validAnchorID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	return strings.IndexFunc(id, func(character rune) bool {
		valid := (character >= 'A' && character <= 'Z') ||
			(character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') || character == '_' || character == '-'
		return !valid
	}) == -1
}
