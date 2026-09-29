package ct_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	ct "github.com/nanostack-dev/anchor/clients/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/cmd/it/shared/logcapture"
	"anchor/cmd/it/shared/mailpit"
)

func TestCreateInvitation_UnderAnchorDeliverySendsOneEmailToTheInvitationAddress(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)
	address := uniqueEmail()

	created := d.invite(address)

	sent := awaitEmailsTo(t, mp, address, 1)
	require.Len(t, sent, 1)
	message := mp.MessageByID(t, sent[0].ID)
	assert.Equal(t, created.Token, tokenInEmail(t, message))
}

func TestCreateInvitation_UnderAnchorDeliveryStillReturnsTheToken(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)

	created := d.invite(uniqueEmail())

	assert.Contains(t, created.Token, "anchor_inv_")
	assert.Equal(t, created.Id, d.invitations.Lookup(created.Token).Id)
}

func TestCreateInvitation_UnderAnchorDeliveryFillsTheTemplateVariables(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)
	address := uniqueEmail()

	created := d.invite(address)

	sent := awaitEmailsTo(t, mp, address, 1)
	message := mp.MessageByID(t, sent[0].ID)
	assert.Contains(t, message.Subject, d.organizationName())
	assert.Contains(t, message.HTML, address)
	assert.Contains(t, message.HTML, d.organizationName())
	assert.Contains(t, message.HTML, d.roleName())
	assert.Contains(t, message.HTML, created.ExpiresAt.UTC().Format(time.RFC3339))
	assert.Contains(t, message.HTML, acceptURLPrefix+created.Token)
}

func TestResendInvitation_UnderAnchorDeliverySendsOneEmailWithTheNewToken(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)
	address := uniqueEmail()
	created := d.invite(address)
	awaitEmailsTo(t, mp, address, 1)

	resent := d.invitations.Resend(d.organizationID, created.Id)

	sent := awaitEmailsTo(t, mp, address, 2)
	require.Len(t, sent, 2)
	tokensInEmails := map[string]bool{}
	for _, message := range sent {
		tokensInEmails[tokenInEmail(t, mp.MessageByID(t, message.ID))] = true
	}
	assert.Equal(t, map[string]bool{created.Token: true, resent.Token: true}, tokensInEmails)
	assert.NotEqual(t, created.Token, resent.Token)
	assert.Equal(t, created.Id, d.invitations.Lookup(resent.Token).Id)
}

func TestCreateInvitation_UnderProductDeliverySendsNoEmail(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	templateID := d.readyForAnchorDelivery(mp)
	d.settings.Update(settingsBody(ct.Product, &templateID, new(acceptURLTemplate), defaultExpiry))
	address := uniqueEmail()

	created := d.invite(address)

	assert.Contains(t, created.Token, "anchor_inv_")
	assertNoEmailTo(t, mp, address)
}

func TestResendInvitation_UnderProductDeliverySendsNoEmail(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	templateID := d.readyForAnchorDelivery(mp)
	d.settings.Update(settingsBody(ct.Product, &templateID, new(acceptURLTemplate), defaultExpiry))
	address := uniqueEmail()
	created := d.invite(address)

	resent := d.invitations.Resend(d.organizationID, created.Id)

	assert.Contains(t, resent.Token, "anchor_inv_")
	assertNoEmailTo(t, mp, address)
}

func TestCreateInvitation_UnderAnchorDeliveryWithAFailingSendReturnsAnErrorAndLeavesNoInvitation(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedUnreachableSMTP()
	d.chooseAnchorDelivery(d.createTemplate(true))

	resp := d.inviteRaw(uniqueEmail())

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeEmailSendFailed, errorCode(t, resp.JSON500.Errors))
	assert.NotContains(t, string(resp.Body), "connection refused")
	assert.Empty(t, d.invitations.Search(d.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}))
	entry := waitForSendFailureLog(t, d)
	assert.Equal(t, "error", entry.String("level"))
	assert.NotEmpty(t, entry.String("invitation_id"))
	assert.NotContains(t, entry.Raw, "anchor_inv_")
	assert.NotContains(t, entry.Raw, acceptURLPrefix)
}

func TestCreateInvitation_UnderAnchorDeliveryLeavesRoomToInviteAgainAfterAFailingSend(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedUnreachableSMTP()
	templateID := d.createTemplate(true)
	d.chooseAnchorDelivery(templateID)
	address := uniqueEmail()
	require.Equal(t, http.StatusInternalServerError, d.inviteRaw(address).StatusCode())

	d.settings.Update(settingsBody(ct.Product, &templateID, new(acceptURLTemplate), defaultExpiry))
	created := d.invite(address)

	assert.Equal(t, ct.Pending, created.Status)
}

func TestResendInvitation_UnderAnchorDeliveryWithAFailingSendReturnsAnErrorAndKeepsTheOldToken(t *testing.T) {
	t.Parallel()
	d := newDeliveryWorld(t)
	d.seedUnreachableSMTP()
	templateID := d.createTemplate(true)
	created := d.invite(uniqueEmail())
	d.settings.Update(settingsBody(ct.Anchor, &templateID, new(acceptURLTemplate), 72*time.Hour))

	resp := d.invitations.ResendRaw(d.organizationID, created.Id)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeEmailSendFailed, errorCode(t, resp.JSON500.Errors))
	assert.Equal(t, created.Id, d.invitations.Lookup(created.Token).Id)
	kept := d.invitations.Get(d.organizationID, created.Id)
	assert.WithinDuration(t, created.ExpiresAt, kept.ExpiresAt, time.Second)
	entry := waitForSendFailureLog(t, d)
	assert.Equal(t, created.Id, entry.String("invitation_id"))
	assert.NotContains(t, entry.Raw, created.Token)
}

func TestCreateInvitation_UnderAnchorDeliveryFailsWhenTheTemplateHasNoPublishedVersion(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.seedMailpitSMTP(mp)
	d.chooseAnchorDelivery(d.createTemplate(false))
	address := uniqueEmail()

	resp := d.inviteRaw(address)

	require.Equal(t, http.StatusInternalServerError, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeEmailSendFailed, errorCode(t, resp.JSON500.Errors))
	assert.Empty(t, d.invitations.Search(d.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}))
	assertNoEmailTo(t, mp, address)
}

func TestCreateInvitation_RefusedWhenTheSMTPIntegrationStoppedBeingActive(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)
	d.deactivateSMTP()
	address := uniqueEmail()

	resp := d.inviteRaw(address)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeInvitationUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionSMTP}, unmetConditions(t, resp.JSON409.Errors))
	assert.Empty(t, d.invitations.Search(d.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}))
	assertNoEmailTo(t, mp, address)
	waitForUnavailableLog(t, d)
	assert.Equal(t, ct.Anchor, d.settings.Get().InvitationDelivery)
}

func TestCreateInvitation_RefusedWhenTheEmailTemplateWasDeleted(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.deleteTemplate(d.readyForAnchorDelivery(mp))

	resp := d.inviteRaw(uniqueEmail())

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeInvitationUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionTemplate}, unmetConditions(t, resp.JSON409.Errors))
	assert.Empty(t, d.invitations.Search(d.organizationID, ct.SearchOrganizationInvitationsJSONRequestBody{}))
	settings := d.settings.Get()
	assert.Equal(t, ct.Anchor, settings.InvitationDelivery)
	assert.Nil(t, settings.EmailTemplateId)
}

func TestResendInvitation_RefusedWhenTheSMTPIntegrationStoppedBeingActive(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	d.readyForAnchorDelivery(mp)
	address := uniqueEmail()
	created := d.invite(address)
	awaitEmailsTo(t, mp, address, 1)
	d.deactivateSMTP()

	resp := d.invitations.ResendRaw(d.organizationID, created.Id)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, codeInvitationUnavailable, errorCode(t, resp.JSON409.Errors))
	assert.Equal(t, []string{conditionSMTP}, unmetConditions(t, resp.JSON409.Errors))
	assert.Equal(t, created.Id, d.invitations.Lookup(created.Token).Id)
	assert.Len(t, emailsTo(t, mp, address), 1)
	waitForUnavailableLog(t, d)
}

func TestResendInvitation_RefusedWhenTheEmailTemplateWasDeleted(t *testing.T) {
	t.Parallel()
	mp := mailpit.Inbox(t)
	d := newDeliveryWorld(t)
	templateID := d.readyForAnchorDelivery(mp)
	created := d.invite(uniqueEmail())
	d.deleteTemplate(templateID)

	resp := d.invitations.ResendRaw(d.organizationID, created.Id)

	require.Equal(t, http.StatusConflict, resp.StatusCode(), string(resp.Body))
	assert.Equal(t, []string{conditionTemplate}, unmetConditions(t, resp.JSON409.Errors))
	assert.Equal(t, created.Id, d.invitations.Lookup(created.Token).Id)
}

func waitForSendFailureLog(t *testing.T, d deliveryWorld) logcapture.Entry {
	t.Helper()
	return logcapture.WaitFor(t, sendFailureLogText, func(entry logcapture.Entry) bool {
		return strings.Contains(entry.String("message"), sendFailureLogText) &&
			entry.String("product_id") == d.product.ProductID &&
			entry.String("organization_id") == d.organizationID
	})
}

func waitForUnavailableLog(t *testing.T, d deliveryWorld) {
	t.Helper()
	logcapture.WaitFor(t, deliveryUnavailable, func(entry logcapture.Entry) bool {
		return entry.String("message") == deliveryUnavailable &&
			entry.String("product_id") == d.product.ProductID &&
			entry.String("organization_id") == d.organizationID
	})
}
