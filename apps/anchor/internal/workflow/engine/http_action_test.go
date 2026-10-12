package engine_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/workflow"
	"anchor/internal/events"
	"anchor/internal/workflow/engine"
)

func TestIsPublicAddress_RefusesEveryNonPublicRange(t *testing.T) {
	for _, address := range []string{
		"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254",
		"100.100.100.100", "198.18.0.1", "0.0.0.0", "::1", "fe80::1", "fd00::1",
		"64:ff9b::a9fe:a9fe", "::ffff:127.0.0.1", "240.0.0.1",
	} {
		assert.False(t, engine.IsPublicAddress(net.ParseIP(address)), address)
	}
}

func TestIsPublicAddress_AcceptsPublicAddresses(t *testing.T) {
	for _, address := range []string{"93.184.215.14", "1.1.1.1", "2606:4700:4700::1111"} {
		assert.True(t, engine.IsPublicAddress(net.ParseIP(address)), address)
	}
}

type fixedSecret struct{}

func (fixedSecret) DeliveryTarget(context.Context, string) (events.DeliveryTarget, bool, error) {
	return events.DeliveryTarget{Secret: "whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw"}, true, nil
}

func callStep(t *testing.T, url string) workflow.StepResult {
	t.Helper()
	catalog := events.NewCatalog(events.CatalogParams{Registrations: []events.Registration{
		events.RegisterInternal(events.GroupOrganizations,
			events.Definition{Type: events.OrganizationCreated, Name: "Organization created"}),
	}})
	eng := engine.New(engine.Services{Caller: engine.NewHTTPCaller(fixedSecret{}, false)}, catalog)
	run := eng.Execute(context.Background(), engine.Execution{
		Workflow: workflow.Workflow{ID: "wf_1", ProductID: "prd_1", Definition: workflow.Definition{
			Steps: []workflow.Step{
				{ID: "call", Action: engine.ActionHTTPRequest, Params: map[string]string{"url": url}},
			},
		}},
		EventData: map[string]string{},
		Trigger:   workflow.RunTriggerManual,
	})
	require.Len(t, run.Steps, 1)
	return run.Steps[0]
}

func TestCallYourBackend_RefusesPlainHTTPUnlessPrivateTargetsAreAllowed(t *testing.T) {
	result := callStep(t, "http://example.com/hook")

	assert.Equal(t, workflow.StepStatusFailed, result.Status)
	assert.Contains(t, result.Error, "must use HTTPS")
}

func TestCallYourBackend_RefusesALoopbackAddressUnlessPrivateTargetsAreAllowed(t *testing.T) {
	result := callStep(t, "https://127.0.0.1:9/hook")

	assert.Equal(t, workflow.StepStatusFailed, result.Status)
	assert.Contains(t, result.Error, "private, loopback or link-local")
}
