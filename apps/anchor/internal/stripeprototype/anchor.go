package stripeprototype

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

const anchorRequestTimeout = 20 * time.Second

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

type AnchorGatewayOption func(*anchorGateway)

func WithAnchorProductName(name string) AnchorGatewayOption {
	return func(gateway *anchorGateway) {
		gateway.product.Name = name
	}
}

type anchorGateway struct {
	baseURL string
	product Product
	apiKey  string
	client  *http.Client
}

type anchorHTTPError struct {
	statusCode int
}

func (e *anchorHTTPError) Error() string {
	return fmt.Sprintf("anchor request failed with HTTP %d", e.statusCode)
}

func NewAnchorGateway(baseURL, productID, apiKey string, options ...AnchorGatewayOption) (AnchorGateway, error) {
	in := struct {
		BaseURL   string `validate:"required,notblank"`
		ProductID string `validate:"required,notblank,max=128"`
		APIKey    string `validate:"required,notblank"`
	}{baseURL, productID, apiKey}
	if err := validate.ValidateStruct(in); err != nil || !validAnchorID(productID) {
		return nil, errors.New("anchor URL, product identifier, and product API key are required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") || !loopbackHost(parsed.Hostname()) {
		return nil, errors.New("anchor URL must be an HTTP loopback origin without credentials or a path")
	}

	gateway := &anchorGateway{
		baseURL: strings.TrimSuffix(parsed.String(), "/"),
		product: Product{ID: productID, Name: productID},
		apiKey:  apiKey,
		client: &http.Client{
			Timeout: anchorRequestTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return errors.New("anchor redirects are disabled")
			},
		},
	}
	for _, option := range options {
		option(gateway)
	}
	return gateway, nil
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

type anchorTemplateResponse struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Status string         `json:"status"`
	Values map[string]any `json:"values"`
}

type anchorLicenseResponse struct {
	TemplateID string         `json:"template_id"`
	Values     map[string]any `json:"values"`
}

type anchorOrganizationResponse struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	License *anchorLicenseResponse `json:"license"`
}

func (a *anchorGateway) Snapshot(ctx context.Context) (AnchorSnapshot, error) {
	var templateList struct {
		Items []anchorTemplateResponse `json:"items"`
	}
	if err := a.request(ctx, http.MethodGet, "/licensing/templates", nil, &templateList); err != nil {
		return AnchorSnapshot{}, err
	}
	templates := functional.Slice(templateList.Items).Map(func(item anchorTemplateResponse) Template {
		values := item.Values
		if values == nil {
			values = map[string]any{}
		}
		return Template{ID: item.ID, Name: item.Name, Values: values, Archived: item.Status == "ARCHIVED"}
	})
	if templates == nil {
		templates = []Template{}
	}

	organizations := []AnchorOrganization{}
	const pageSize = 100
	for offset := 0; ; offset += pageSize {
		var page struct {
			Items []anchorOrganizationResponse `json:"items"`
			Total int                          `json:"total"`
		}
		input := map[string]any{"pagination": map[string]int{"limit": pageSize, "offset": offset}}
		if err := a.request(ctx, http.MethodPost, "/organizations/search?include=license", input, &page); err != nil {
			return AnchorSnapshot{}, err
		}
		organizations = append(organizations,
			functional.Slice(page.Items).Map(func(item anchorOrganizationResponse) AnchorOrganization {
				organization := AnchorOrganization{ID: item.ID, Name: item.Name, LicenseValues: map[string]any{}}
				if item.License != nil {
					organization.TemplateID = item.License.TemplateID
					if item.License.Values != nil {
						organization.LicenseValues = item.License.Values
					}
				}
				return organization
			})...,
		)
		if len(page.Items) < pageSize || offset+len(page.Items) >= page.Total {
			break
		}
	}
	return AnchorSnapshot{Product: a.product, Templates: templates, Organizations: organizations}, nil
}

func (a *anchorGateway) ApplyTemplate(ctx context.Context, organizationID, templateID string) error {
	in := struct {
		OrganizationID string `validate:"required,notblank,max=128"`
		TemplateID     string `validate:"required,notblank,max=128"`
	}{organizationID, templateID}
	if err := validate.ValidateStruct(in); err != nil || !validAnchorID(organizationID) || !validAnchorID(templateID) {
		return errors.New("organization and template identifiers are required")
	}

	var held anchorLicenseResponse
	err := a.request(ctx, http.MethodGet, "/organizations/"+organizationID+"/license", nil, &held)
	if err == nil && held.TemplateID == templateID {
		return nil
	}
	if err != nil {
		var httpError *anchorHTTPError
		if !errors.As(err, &httpError) || httpError.statusCode != http.StatusNotFound {
			return err
		}
	}
	input := map[string]any{
		"template_id":      templateID,
		"organization_ids": []string{organizationID},
		"on_difference":    "CARRY_FORWARD",
	}
	var receipt struct {
		TemplateID string `json:"template_id"`
		Failed     int    `json:"failed"`
		Results    []struct {
			OrganizationID string          `json:"organization_id"`
			Outcome        string          `json:"outcome"`
			Error          json.RawMessage `json:"error"`
		} `json:"results"`
	}
	if migrationErr := a.request(
		ctx,
		http.MethodPost,
		"/licensing/organization-licenses/migrate",
		input,
		&receipt,
	); migrationErr != nil {
		return migrationErr
	}
	if receipt.TemplateID != templateID || receipt.Failed != 0 || len(receipt.Results) != 1 {
		return errors.New("anchor did not confirm the organization license migration")
	}
	result := receipt.Results[0]
	if result.OrganizationID != organizationID ||
		(result.Outcome != "CHANGED" && result.Outcome != "UNCHANGED") ||
		(len(result.Error) != 0 && string(result.Error) != "null") {
		return errors.New("anchor did not apply the organization license migration")
	}
	return nil
}

func (a *anchorGateway) request(ctx context.Context, method, resource string, body, output any) error {
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			return errors.New("cannot encode Anchor request")
		}
	}
	//nolint:gosec // NewAnchorGateway permits loopback origins only and validates product and organization IDs.
	request, err := http.NewRequestWithContext(ctx, method,
		a.baseURL+"/v1/products/"+a.product.ID+resource, bytes.NewReader(encoded))
	if err != nil {
		return errors.New("cannot create Anchor request")
	}
	request.Header.Set("X-Product-Api-Key", a.apiKey)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	//nolint:gosec // NewAnchorGateway permits loopback origins only and disables redirects.
	response, err := a.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("cannot reach the local Anchor API")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &anchorHTTPError{statusCode: response.StatusCode}
	}
	if output == nil {
		return nil
	}
	const maxAnchorResponseBytes = 8 << 20
	if decodeErr := json.NewDecoder(io.LimitReader(response.Body, maxAnchorResponseBytes)).
		Decode(output); decodeErr != nil {
		return errors.New("the local Anchor API returned an invalid response")
	}
	return nil
}
