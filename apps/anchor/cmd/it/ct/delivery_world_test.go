package ct_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"regexp"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	itdsl "anchor/cmd/it/shared/dsl"
	"anchor/cmd/it/shared/mailpit"
	domainintegration "anchor/internal/domain/integration"
	smtpprov "anchor/internal/integration/provider/smtp"
)

const (
	acceptURLPrefix   = "https://app.example.com/accept?token="
	acceptURLTemplate = acceptURLPrefix + "{token}"

	codeSettingsUnavailable   = "ORGANIZATION_INVITATION_SETTINGS_ANCHOR_DELIVERY_UNAVAILABLE"
	codeInvitationUnavailable = "ORGANIZATION_INVITATION_ANCHOR_DELIVERY_UNAVAILABLE"
	codeEmailSendFailed       = "ORGANIZATION_INVITATION_EMAIL_SEND_FAILED"

	conditionSMTP        = "smtp_integration_active"
	conditionTemplate    = "email_template_set"
	conditionAcceptURL   = "accept_url_template_set"
	sendFailureLogText   = "failed to send the invitation email"
	deliveryUnavailable  = "anchor delivery is unavailable: invitation call refused"
	noEmailObservationMs = 300
)

var acceptTokenInEmail = regexp.MustCompile(regexp.QuoteMeta(acceptURLPrefix) + `([A-Za-z0-9_]+)`)

type deliveryWorld struct {
	world
	tenantID      string
	settings      itdsl.InvitationSettingsClient
	integrationID string
}

func newDeliveryWorld(t *testing.T) deliveryWorld {
	t.Helper()
	state := itdsl.Given(t).
		Tenant(itdsl.TenantOpts{Alias: "t", Isolated: true}).
		Product(itdsl.ProductOpts{Alias: "p", TenantAlias: "t"}).
		Build()
	product := state.Product("p")
	return deliveryWorld{
		world:    newWorldOn(t, product),
		tenantID: state.Tenant("t").ID,
		settings: product.InvitationSettings(),
	}
}

func (d *deliveryWorld) seedSMTP(host string, port int) {
	d.t.Helper()
	configJSON, err := json.Marshal(smtpprov.Config{
		Host:        host,
		Port:        port,
		Encryption:  smtpprov.EncryptionNone,
		AuthMethod:  smtpprov.AuthMethodPlain,
		Username:    "test",
		Password:    "test",
		FromAddress: "noreply@tryanchor.dev",
		FromName:    "Anchor",
	})
	require.NoError(d.t, err)

	instance := domainintegration.Instance{
		PlatformTenantID: d.tenantID,
		ProductID:        d.product.ProductID,
		ProviderType:     domainintegration.ProviderTypeSMTP,
		ConfigJSON:       configJSON,
		ConfigVersion:    1,
		IsEnabled:        true,
		Status:           domainintegration.StatusActive,
	}
	instance.GenerateID()
	created, err := IntegrationRepo.Create(context.Background(), instance)
	require.NoError(d.t, err)
	d.integrationID = created.ID
}

func (d *deliveryWorld) seedMailpitSMTP(mp *mailpit.Mailpit) {
	d.t.Helper()
	d.seedSMTP(mp.SMTPHost, mp.SMTPPort)
}

func (d *deliveryWorld) seedUnreachableSMTP() {
	d.t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(d.t, err)
	address, ok := listener.Addr().(*net.TCPAddr)
	require.True(d.t, ok)
	require.NoError(d.t, listener.Close())
	d.seedSMTP(address.IP.String(), address.Port)
}

func (d *deliveryWorld) deactivateSMTP() {
	d.t.Helper()
	resp, err := d.product.OwnerAuthenticatedClient().UpdateIntegrationInstanceWithResponse(
		context.Background(), d.product.ProductID, d.integrationID,
		ct.UpdateIntegrationInstanceJSONRequestBody{IsEnabled: new(false)},
	)
	require.NoError(d.t, err)
	require.Equal(d.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
}

func (d *deliveryWorld) createTemplate(publish bool) string {
	d.t.Helper()
	client := d.product.OwnerAuthenticatedClient()
	created, err := client.CreateEmailTemplateWithResponse(
		context.Background(), d.product.ProductID,
		ct.CreateEmailTemplateJSONRequestBody{
			Slug:    "invitation-" + d.organizationID,
			Name:    "Invitation",
			Subject: "Join {{ .organization_name }}",
			BodyHtml: "<p>{{ .invitee_email }} joins {{ .organization_name }} as {{ .role_name }} " +
				"until {{ .expires_at }}.</p><p>{{ .accept_url }}</p>",
		},
	)
	require.NoError(d.t, err)
	require.Equal(d.t, http.StatusCreated, created.StatusCode(), string(created.Body))
	templateID := created.JSON201.Id

	if publish {
		published, publishErr := client.PublishEmailTemplateWithResponse(
			context.Background(), d.product.ProductID, templateID,
		)
		require.NoError(d.t, publishErr)
		require.Equal(d.t, http.StatusOK, published.StatusCode(), string(published.Body))
	}
	return templateID
}

func (d *deliveryWorld) deleteTemplate(templateID string) {
	d.t.Helper()
	resp, err := d.product.OwnerAuthenticatedClient().DeleteEmailTemplateWithResponse(
		context.Background(), d.product.ProductID, templateID,
	)
	require.NoError(d.t, err)
	require.Less(d.t, resp.StatusCode(), http.StatusMultipleChoices, string(resp.Body))
}

func settingsBody(
	delivery ct.InvitationDelivery, templateID *string, acceptURL *string, expiry time.Duration,
) ct.UpdateInvitationSettingsJSONRequestBody {
	return ct.UpdateInvitationSettingsJSONRequestBody{
		InvitationDelivery:   delivery,
		EmailTemplateId:      templateID,
		AcceptUrlTemplate:    acceptURL,
		DefaultExpirySeconds: int64(expiry / time.Second),
	}
}

func (d *deliveryWorld) chooseAnchorDelivery(templateID string) {
	d.t.Helper()
	d.settings.Update(settingsBody(ct.Anchor, &templateID, new(acceptURLTemplate), defaultExpiry))
}

// readyForAnchorDelivery configures a working SMTP relay, a published template
// and the Anchor delivery settings, and returns the template identifier.
func (d *deliveryWorld) readyForAnchorDelivery(mp *mailpit.Mailpit) string {
	d.t.Helper()
	d.seedMailpitSMTP(mp)
	templateID := d.createTemplate(true)
	d.chooseAnchorDelivery(templateID)
	return templateID
}

func (d *deliveryWorld) organizationName() string {
	d.t.Helper()
	return d.product.Organizations().Get(d.organizationID).Name
}

func (d *deliveryWorld) roleName() string {
	d.t.Helper()
	resp, err := d.product.OwnerAuthenticatedClient().GetProductRoleWithResponse(
		context.Background(), d.product.ProductID, d.roleID,
	)
	require.NoError(d.t, err)
	require.Equal(d.t, http.StatusOK, resp.StatusCode(), string(resp.Body))
	return resp.JSON200.Name
}

func tokenInEmail(t *testing.T, message mailpit.MessageBody) string {
	t.Helper()
	match := acceptTokenInEmail.FindStringSubmatch(message.HTML)
	require.Len(t, match, 2, "no accept link in the email: %s", message.HTML)
	return match[1]
}

func emailsTo(t *testing.T, mp *mailpit.Mailpit, address string) []mailpit.MessageSummary {
	t.Helper()
	var matching []mailpit.MessageSummary
	for _, message := range mp.Messages(t) {
		for _, recipient := range message.To {
			if recipient.Address == address {
				matching = append(matching, message)
			}
		}
	}
	return matching
}

func awaitEmailsTo(t *testing.T, mp *mailpit.Mailpit, address string, count int) []mailpit.MessageSummary {
	t.Helper()
	var matching []mailpit.MessageSummary
	require.Eventually(t, func() bool {
		matching = emailsTo(t, mp, address)
		return len(matching) >= count
	}, 10*time.Second, 100*time.Millisecond)
	return matching
}

func assertNoEmailTo(t *testing.T, mp *mailpit.Mailpit, address string) {
	t.Helper()
	time.Sleep(noEmailObservationMs * time.Millisecond)
	assert.Empty(t, emailsTo(t, mp, address))
}

func unmetConditions(t *testing.T, errs []ct.ApiError) []string {
	t.Helper()
	require.NotEmpty(t, errs)
	require.NotNil(t, errs[0].Metadata)
	raw, ok := (*errs[0].Metadata)["unmet_conditions"].([]any)
	require.True(t, ok, "unmet_conditions is missing from the error metadata")
	conditions := make([]string, 0, len(raw))
	for _, condition := range raw {
		name, isString := condition.(string)
		require.True(t, isString)
		conditions = append(conditions, name)
	}
	return conditions
}
