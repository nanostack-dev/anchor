package ct_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/mailpit"
)

const (
	codeTemplateWithoutToken = "ORGANIZATION_INVITATION_SETTINGS_ACCEPT_URL_TEMPLATE_WITHOUT_TOKEN"
	codeUnknownTemplate      = "ORGANIZATION_INVITATION_SETTINGS_EMAIL_TEMPLATE_NOT_FOUND"
)

func TestInvitationSettings_ANewProductReadsTheDefaults(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	settings := w.product.InvitationSettings().Get()

	assert.Equal(t, ct.Product, settings.InvitationDelivery)
	assert.Nil(t, settings.EmailTemplateId)
	assert.Nil(t, settings.AcceptUrlTemplate)
	assert.EqualValues(t, defaultExpiry/time.Second, settings.DefaultExpirySeconds)
}

func TestInvitationSettings_APlatformUserChangesTheDefaultExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	client := w.product.InvitationSettings()

	updated := client.Update(settingsBody(ct.Product, nil, nil, 48*time.Hour))

	assert.EqualValues(t, 48*3600, updated.DefaultExpirySeconds)
	assert.EqualValues(t, 48*3600, client.Get().DefaultExpirySeconds)
}

func TestInvitationSettings_APlatformUserChangesTheAcceptURLTemplate(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	client := w.product.InvitationSettings()
	template := "https://other.example.com/join/{token}/now"

	updated := client.Update(settingsBody(ct.Product, nil, &template, defaultExpiry))

	require.NotNil(t, updated.AcceptUrlTemplate)
	assert.Equal(t, template, *updated.AcceptUrlTemplate)
	require.NotNil(t, client.Get().AcceptUrlTemplate)
	assert.Equal(t, template, *client.Get().AcceptUrlTemplate)
}

func TestInvitationSettings_APlatformUserChangesTheEmailTemplate(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	templateID := d.createTemplate(false)

	updated := d.settings.Update(settingsBody(ct.Product, &templateID, nil, defaultExpiry))

	require.NotNil(t, updated.EmailTemplateId)
	assert.Equal(t, templateID, *updated.EmailTemplateId)
	require.NotNil(t, d.settings.Get().EmailTemplateId)
	assert.Equal(t, templateID, *d.settings.Get().EmailTemplateId)
}

func TestInvitationSettings_APlatformUserChangesTheInvitationDelivery(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	mp := mailpit.Inbox(t)

	d.readyForAnchorDelivery(mp)
	assert.Equal(t, ct.Anchor, d.settings.Get().InvitationDelivery)

	current := d.settings.Get()
	back := d.settings.Update(
		settingsBody(ct.Product, current.EmailTemplateId, current.AcceptUrlTemplate, defaultExpiry),
	)
	assert.Equal(t, ct.Product, back.InvitationDelivery)
	assert.Equal(t, ct.Product, d.settings.Get().InvitationDelivery)
}

func TestInvitationSettings_AnAbsentEmailTemplateAndAcceptURLTemplateClearThem(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	templateID := d.createTemplate(false)
	d.settings.Update(settingsBody(ct.Product, &templateID, new(acceptURLTemplate), defaultExpiry))

	cleared := d.settings.UpdateRawBody(`{"invitation_delivery":"product","default_expiry_seconds":604800}`)

	require.Equal(t, http.StatusOK, cleared.StatusCode(), string(cleared.Body))
	assert.Nil(t, cleared.JSON200.EmailTemplateId)
	assert.Nil(t, cleared.JSON200.AcceptUrlTemplate)
	assert.Nil(t, d.settings.Get().EmailTemplateId)
	assert.Nil(t, d.settings.Get().AcceptUrlTemplate)
}

func TestInvitationSettings_RefuseAnAcceptURLTemplateWithoutTheTokenPlaceholder(t *testing.T) {
	t.Parallel()
	for _, delivery := range []ct.InvitationDelivery{ct.Product, ct.Anchor} {
		t.Run(string(delivery), func(t *testing.T) {
			t.Parallel()
			w := newWorld(t)

			resp := w.product.InvitationSettings().UpdateRaw(
				settingsBody(delivery, nil, new("https://app.example.com/accept"), defaultExpiry),
			)

			require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
			assert.Equal(t, codeTemplateWithoutToken, errorCode(t, resp.JSON400.Errors))
		})
	}
}

func TestInvitationSettings_RefuseAnAcceptURLTemplateLongerThan2048Characters(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	template := "https://app.example.com/" + strings.Repeat("a", 2048) + "?token={token}"

	resp := w.product.InvitationSettings().UpdateRaw(settingsBody(ct.Product, nil, &template, defaultExpiry))

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_RefuseAnEmailTemplateThatDoesNotExist(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRaw(
		settingsBody(ct.Product, new("etpl_unknown"), nil, defaultExpiry),
	)

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeUnknownTemplate, errorCode(t, resp.JSON400.Errors))
}

func TestInvitationSettings_RefuseTheEmailTemplateOfAnotherProduct(t *testing.T) {
	t.Parallel()
	other := newDeliveryWorld(t)
	foreignTemplateID := other.createTemplate(true)
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRaw(settingsBody(ct.Product, &foreignTemplateID, nil, defaultExpiry))

	require.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeUnknownTemplate, errorCode(t, resp.JSON400.Errors))
}

func TestInvitationSettings_RefuseADefaultExpiryBelowOneHour(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRaw(settingsBody(ct.Product, nil, nil, time.Hour-time.Second))

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_RefuseADefaultExpiryAboveNinetyDays(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRaw(settingsBody(ct.Product, nil, nil, 90*24*time.Hour+time.Second))

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_AcceptTheBoundsOfTheDefaultExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	client := w.product.InvitationSettings()

	shortest := client.Update(settingsBody(ct.Product, nil, nil, time.Hour))
	longest := client.Update(settingsBody(ct.Product, nil, nil, 90*24*time.Hour))

	assert.EqualValues(t, 3600, shortest.DefaultExpirySeconds)
	assert.EqualValues(t, 90*24*3600, longest.DefaultExpirySeconds)
}

func TestInvitationSettings_RefuseAnUnknownInvitationDelivery(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRawBody(
		`{"invitation_delivery":"carrier_pigeon","default_expiry_seconds":604800}`,
	)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_RefuseAMissingDefaultExpiry(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRawBody(`{"invitation_delivery":"product"}`)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_ARefusedChangeKeepsTheStoredSettings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	client := w.product.InvitationSettings()
	client.Update(settingsBody(ct.Product, nil, new(acceptURLTemplate), 48*time.Hour))

	refused := client.UpdateRaw(settingsBody(ct.Product, nil, new("https://app.example.com/none"), time.Hour))

	require.Equal(t, http.StatusBadRequest, refused.StatusCode())
	kept := client.Get()
	require.NotNil(t, kept.AcceptUrlTemplate)
	assert.Equal(t, acceptURLTemplate, *kept.AcceptUrlTemplate)
	assert.EqualValues(t, 48*3600, kept.DefaultExpirySeconds)
}

func TestInvitationSettings_RefuseAnchorDeliveryWhenTheSMTPIntegrationIsNotActive(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	templateID := d.createTemplate(true)

	resp := d.settings.UpdateRaw(settingsBody(ct.Anchor, &templateID, new(acceptURLTemplate), defaultExpiry))

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeSettingsUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionSMTP}, unmetConditions(t, resp.JSON409.Errors))
	assert.Contains(t, resp.JSON409.Errors[0].Message, "SMTP integration")
	assert.Equal(t, ct.Product, d.settings.Get().InvitationDelivery)
}

func TestInvitationSettings_RefuseAnchorDeliveryWhenTheSMTPIntegrationIsDisabled(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedMailpitSMTP(mailpit.Inbox(t))
	d.deactivateSMTP()
	templateID := d.createTemplate(true)

	resp := d.settings.UpdateRaw(settingsBody(ct.Anchor, &templateID, new(acceptURLTemplate), defaultExpiry))

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, []string{conditionSMTP}, unmetConditions(t, resp.JSON409.Errors))
}

func TestInvitationSettings_RefuseAnchorDeliveryWhenNoEmailTemplateIsChosen(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedMailpitSMTP(mailpit.Inbox(t))

	resp := d.settings.UpdateRaw(settingsBody(ct.Anchor, nil, new(acceptURLTemplate), defaultExpiry))

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeSettingsUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionTemplate}, unmetConditions(t, resp.JSON409.Errors))
	assert.Contains(t, resp.JSON409.Errors[0].Message, "email template")
}

func TestInvitationSettings_RefuseAnchorDeliveryWhenNoAcceptURLTemplateIsSet(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedMailpitSMTP(mailpit.Inbox(t))
	templateID := d.createTemplate(true)

	resp := d.settings.UpdateRaw(settingsBody(ct.Anchor, &templateID, nil, defaultExpiry))

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeSettingsUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionAcceptURL}, unmetConditions(t, resp.JSON409.Errors))
	assert.Contains(t, resp.JSON409.Errors[0].Message, "accept URL template")
}

func TestInvitationSettings_TheAnchorDeliveryRefusalNamesEveryFalseCondition(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().UpdateRaw(settingsBody(ct.Anchor, nil, nil, defaultExpiry))

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(
		t,
		[]string{conditionSMTP, conditionTemplate, conditionAcceptURL},
		unmetConditions(t, resp.JSON409.Errors),
	)
}

func TestInvitationSettings_AChangeEmitsNoProductEvent(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	sink := w.product.CaptureEvents()
	before := w.invite(uniqueEmail())
	sink.WaitFor(eventInvitationCreated, invitationFields(w, before.Id))
	eventsBefore := sink.Total()

	w.product.InvitationSettings().Update(settingsBody(ct.Product, nil, new(acceptURLTemplate), 48*time.Hour))
	marker := w.invite(uniqueEmail())
	sink.WaitFor(eventInvitationCreated, invitationFields(w, marker.Id))

	assert.Equal(t, eventsBefore+1, sink.Total())
}

func TestInvitationSettings_AProductAPIKeyCannotChangeThem(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	keyClient := w.product.InvitationSettings().As(w.product.AllScopeAPIKeyClient())

	resp := keyClient.UpdateRaw(settingsBody(ct.Product, nil, new(acceptURLTemplate), 48*time.Hour))

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
	assert.Nil(t, w.product.InvitationSettings().Get().AcceptUrlTemplate)
}

func TestInvitationSettings_AProductAPIKeyCannotReadThem(t *testing.T) {
	t.Parallel()
	w := newWorld(t)

	resp := w.product.InvitationSettings().As(w.product.AllScopeAPIKeyClient()).GetRaw()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode(), string(resp.Body))
}

func TestInvitationSettings_BelongToOneProduct(t *testing.T) {
	t.Parallel()
	changed := newWorld(t)
	untouched := newWorld(t)

	changed.product.InvitationSettings().Update(settingsBody(ct.Product, nil, new(acceptURLTemplate), 48*time.Hour))

	other := untouched.product.InvitationSettings().Get()
	assert.Nil(t, other.AcceptUrlTemplate)
	assert.EqualValues(t, defaultExpiry/time.Second, other.DefaultExpirySeconds)
}

func TestCreateInvitation_UsesTheDefaultExpiryOfTheSettings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.product.InvitationSettings().Update(settingsBody(ct.Product, nil, nil, 48*time.Hour))

	created := w.invite(uniqueEmail())

	assert.WithinDuration(t, time.Now().Add(48*time.Hour), created.ExpiresAt, time.Minute)
}

func TestCreateInvitation_AnExplicitExpiryWinsOverTheDefaultExpiryOfTheSettings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	w.product.InvitationSettings().Update(settingsBody(ct.Product, nil, nil, 48*time.Hour))
	expiresAt := time.Now().Add(5 * 24 * time.Hour).UTC().Truncate(time.Second)

	created := w.invitations.Create(w.organizationID, ct.CreateOrganizationInvitationJSONRequestBody{
		Email:     openapi_types.Email(uniqueEmail()),
		RoleId:    w.roleID,
		ExpiresAt: &expiresAt,
	})

	assert.WithinDuration(t, expiresAt, created.ExpiresAt, time.Second)
}

func TestResendInvitation_UsesTheDefaultExpiryOfTheSettings(t *testing.T) {
	t.Parallel()
	w := newWorld(t)
	created := w.invite(uniqueEmail())
	w.product.InvitationSettings().Update(settingsBody(ct.Product, nil, nil, 72*time.Hour))

	resent := w.invitations.Resend(w.organizationID, created.Id)

	assert.WithinDuration(t, time.Now().Add(72*time.Hour), resent.ExpiresAt, time.Minute)
}
