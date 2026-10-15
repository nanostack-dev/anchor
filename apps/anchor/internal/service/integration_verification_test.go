//nolint:testpackage // Exercises private verification-run coordination with deterministic repository and provider seams.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/integration"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
)

type verificationRepository struct {
	repository.IntegrationInstanceRepository
	mu            sync.Mutex
	instance      integration.Instance
	persistError  error
	beforePersist func()
}

func (r *verificationRepository) FindByIDInternal(
	ctx context.Context,
	_ string,
) (functional.Option[integration.Instance], error) {
	if err := ctx.Err(); err != nil {
		return functional.None[integration.Instance](), err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return functional.Some(r.instance), nil
}

func (r *verificationRepository) CompareAndSetVerificationStatusInternal(
	_ context.Context, snapshot integration.Instance, status integration.Status, lastError *string,
) (functional.Option[integration.Instance], error) {
	if r.beforePersist != nil {
		r.beforePersist()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.persistError != nil {
		return functional.None[integration.Instance](), r.persistError
	}
	current := r.instance
	if current.ID != snapshot.ID || current.PlatformTenantID != snapshot.PlatformTenantID ||
		current.ProductID != snapshot.ProductID || current.ConfigVersion != snapshot.ConfigVersion ||
		current.IsEnabled != snapshot.IsEnabled || !bytes.Equal(current.ConfigJSON, snapshot.ConfigJSON) {
		return functional.None[integration.Instance](), nil
	}
	r.instance.Status, r.instance.LastError = status, lastError
	return functional.Some(r.instance), nil
}

func (r *verificationRepository) change(update func(*integration.Instance)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	update(&r.instance)
}

func (r *verificationRepository) current() integration.Instance {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.instance
}

type verificationProvider struct {
	provider.Provider
	verify func(context.Context, *integration.Instance) error
}

func (*verificationProvider) Type() string { return "STRIPE" }
func (p *verificationProvider) VerifyConnection(ctx context.Context, instance *integration.Instance) error {
	return p.verify(ctx, instance)
}

func verificationFixture() integration.Instance {
	return integration.Instance{ID: "iin_fixture", PlatformTenantID: "tenant_fixture", ProductID: "prd_fixture",
		ProviderType: integration.ProviderTypeStripe, ConfigJSON: json.RawMessage(`{"account_id":"acct_Old"}`),
		ConfigVersion: 1, IsEnabled: true, Status: integration.StatusConfiguring}
}

func verificationService(repo *verificationRepository, p *verificationProvider) *integrationService {
	return &integrationService{instanceRepo: repo,
		registry:         provider.NewRegistry(provider.RegistryParams{Providers: []provider.Provider{p}}),
		verificationRuns: map[string]*verificationRun{}, logger: zerolog.Nop()}
}

func TestInterruptedConnectionVerificationRecoversSynchronously(t *testing.T) {
	t.Parallel()
	repo := &verificationRepository{instance: verificationFixture()}
	calls := 0
	p := &verificationProvider{verify: func(ctx context.Context, _ *integration.Instance) error {
		calls++
		_, bounded := ctx.Deadline()
		require.True(t, bounded)
		return nil
	}}
	svc := verificationService(repo, p)
	require.NoError(t, svc.VerifyAndActivateInternal(t.Context(), repo.instance.ID))
	require.Equal(t, 1, calls)
	require.Equal(t, integration.StatusActive, repo.current().Status)
	require.Empty(t, svc.verificationRuns)
}

func TestVerificationRecoveryLeavesCurrentProcessRunAlone(t *testing.T) {
	t.Parallel()
	repo := &verificationRepository{instance: verificationFixture()}
	p := &verificationProvider{verify: func(context.Context, *integration.Instance) error {
		t.Fatal("recovery must not duplicate current verification")
		return nil
	}}
	svc := verificationService(repo, p)
	_, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	run := &verificationRun{cancel: cancel}
	svc.verificationRuns[repo.instance.ID] = run
	require.NoError(t, svc.VerifyAndActivateInternal(t.Context(), repo.instance.ID))
	require.Same(t, run, svc.verificationRuns[repo.instance.ID])
	require.Equal(t, integration.StatusConfiguring, repo.current().Status)
}

func TestConcurrentSaveSupersedesRecoveredVerification(t *testing.T) {
	t.Parallel()
	repo := &verificationRepository{instance: verificationFixture()}
	started := make(chan struct{})
	newConfig := json.RawMessage(`{"account_id":"acct_New"}`)
	p := &verificationProvider{verify: func(ctx context.Context, instance *integration.Instance) error {
		if bytes.Equal(instance.ConfigJSON, newConfig) {
			return nil
		}
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}
	svc := verificationService(repo, p)
	done := make(chan error, 1)
	go func() { done <- svc.VerifyAndActivateInternal(t.Context(), "iin_fixture") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("recovery did not start")
	}
	repo.change(func(instance *integration.Instance) { instance.ConfigJSON = newConfig })
	svc.startVerifyAndActivate(zerolog.Nop(), repo.current(), p)
	require.NoError(t, <-done)
	require.Eventually(
		t,
		func() bool { return repo.current().Status == integration.StatusActive },
		time.Second,
		time.Millisecond,
	)
	require.JSONEq(t, string(newConfig), string(repo.current().ConfigJSON))
}

func TestRecoveredVerificationCannotOverwriteChangedConfigVersionOrPause(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(*integration.Instance)
	}{
		{"config changed", func(instance *integration.Instance) {
			instance.ConfigJSON = json.RawMessage(`{"account_id":"acct_New"}`)
		}},
		{"schema changed", func(instance *integration.Instance) { instance.ConfigVersion++ }},
		{"paused", func(instance *integration.Instance) {
			instance.IsEnabled = false
			instance.Status = integration.StatusInactive
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			repo := &verificationRepository{instance: verificationFixture()}
			p := &verificationProvider{verify: func(context.Context, *integration.Instance) error { return nil }}
			repo.beforePersist = func() { repo.change(test.change) }
			svc := verificationService(repo, p)
			require.NoError(t, svc.VerifyAndActivateInternal(t.Context(), "iin_fixture"))
			require.NotEqual(t, integration.StatusActive, repo.current().Status)
			require.Nil(t, repo.current().LastError)
		})
	}
}

func TestVerificationRecoveryPersistsFailureAndRetriesPersistenceErrors(t *testing.T) {
	t.Parallel()
	repo := &verificationRepository{instance: verificationFixture()}
	failed := errors.New("sandbox authentication failed")
	p := &verificationProvider{verify: func(context.Context, *integration.Instance) error { return failed }}
	svc := verificationService(repo, p)
	require.NoError(t, svc.VerifyAndActivateInternal(t.Context(), "iin_fixture"))
	require.Equal(t, integration.StatusError, repo.current().Status)
	require.Equal(t, failed.Error(), *repo.current().LastError)
	repo.change(func(instance *integration.Instance) { instance.Status = integration.StatusConfiguring })
	repo.persistError = errors.New("database unavailable")
	require.ErrorIs(t, svc.VerifyAndActivateInternal(t.Context(), "iin_fixture"), repo.persistError)
	require.Equal(t, integration.StatusConfiguring, repo.current().Status)
	require.Empty(t, svc.verificationRuns)
}
