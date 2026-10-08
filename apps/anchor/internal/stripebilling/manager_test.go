//nolint:testpackage // Tests verify private tenant-scoped repository calls before any billing operation.
package stripebilling

import (
	"context"
	"errors"
	"testing"
	"time"

	"anchor/internal/domain/integration"
	"anchor/internal/domain/product"
	"anchor/internal/integration/provider"
	"anchor/internal/repository"
	"anchor/internal/service"
	billing "anchor/internal/stripebilling/billing"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type scopedProducts struct {
	service.ProductService
	err                 error
	tenantID, productID string
}

func (p *scopedProducts) Get(_ context.Context, in product.GetProductInput) (product.Product, error) {
	p.tenantID = in.TenantID
	p.productID = in.ProductID
	return product.Product{ID: in.ProductID}, p.err
}

type scopedInstances struct {
	repository.IntegrationInstanceRepository
	tenantID, productID string
	calls               int
	instance            functional.Option[integration.Instance]
	list                []integration.Instance
}

func (r *scopedInstances) FindByProductAndProvider(
	_ context.Context,
	tenantID, productID, providerType string,
) (functional.Option[integration.Instance], error) {
	r.calls++
	r.tenantID = tenantID
	r.productID = productID
	if providerType != "STRIPE" {
		return functional.None[integration.Instance](), errors.New("wrong provider")
	}
	return r.instance, nil
}

func TestBillingAuthorizesProductBeforeReadingIntegration(t *testing.T) {
	t.Parallel()
	denied := errors.New("product does not belong to tenant")
	products := &scopedProducts{err: denied}
	instances := &scopedInstances{}
	manager := NewManager(Params{Products: products, Instances: instances})
	err := manager.WithService(
		context.Background(),
		"tenant-other",
		"product-owned",
		func(context.Context, *billing.Service) error { t.Fatal("billing action must not execute"); return nil },
	)
	require.ErrorIs(t, err, denied)
	require.Equal(t, "tenant-other", products.tenantID)
	require.Equal(t, "product-owned", products.productID)
	require.Zero(t, instances.calls)
}
func TestBillingRequiresEnabledActiveTenantScopedIntegration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		instance functional.Option[integration.Instance]
	}{
		{"missing", functional.None[integration.Instance]()},
		{
			"disabled",
			functional.Some(
				integration.Instance{ID: "integration", IsEnabled: false, Status: integration.StatusActive},
			),
		},
		{
			"configuring",
			functional.Some(
				integration.Instance{ID: "integration", IsEnabled: true, Status: integration.StatusConfiguring},
			),
		},
		{
			"connection error",
			functional.Some(integration.Instance{ID: "integration", IsEnabled: true, Status: integration.StatusError}),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			products := &scopedProducts{}
			instances := &scopedInstances{instance: tc.instance}
			manager := NewManager(Params{Products: products, Instances: instances})
			err := manager.WithService(
				context.Background(),
				"tenant",
				"product",
				func(context.Context, *billing.Service) error {
					t.Fatal("disconnected billing action must not execute")
					return nil
				},
			)
			require.Error(t, err)
			require.Equal(t, "tenant", instances.tenantID)
			require.Equal(t, "product", instances.productID)
		})
	}
}

type billingResolver struct {
	provider.Provider
	err error
}

func (*billingResolver) Type() string { return "STRIPE" }

func (r *billingResolver) ResolveBillingConfig(
	context.Context,
	integration.Instance,
) (billing.ConnectionConfig, error) {
	return billing.ConnectionConfig{}, r.err
}

func TestStateReadsAuthorizeProductWithoutTakingMutationPermitOrLock(t *testing.T) {
	t.Parallel()
	refused := errors.New("fixture stops before external Stripe traffic")
	resolver := &billingResolver{err: refused}
	instances := &scopedInstances{instance: functional.Some(integration.Instance{
		ID: "iin_fixture", IsEnabled: true, Status: integration.StatusActive,
	})}
	manager := NewManager(Params{Products: &scopedProducts{}, Instances: instances,
		Registry: provider.NewRegistry(provider.RegistryParams{Providers: []provider.Provider{resolver}})})
	// No database is provided; acquiring a write session would be an invalid call.
	// Occupied mutation permits must also leave a state read available.
	for range cap(manager.permits) {
		manager.permits <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	_, err := manager.ReadState(ctx, "tenant", "product")
	require.ErrorIs(t, err, refused)
	require.Equal(t, "tenant", instances.tenantID)
	require.Equal(t, "product", instances.productID)
}

func TestReadOnlyBillingStateRejectsAccidentalMutations(t *testing.T) {
	t.Parallel()
	store := readOnlyStateStore{}
	err := store.Update(
		func(*billing.StoredState) error { t.Fatal("read-only state must never apply writes"); return nil },
	)
	require.ErrorContains(t, err, "cannot perform mutations")
}

type recoveryIntegrationService struct {
	service.IntegrationService
	calls []string
}

func (s *recoveryIntegrationService) VerifyAndActivateInternal(_ context.Context, id string) error {
	s.calls = append(s.calls, id)
	return nil
}

func (r *scopedInstances) ListByProviderInternal(_ context.Context, kind string) ([]integration.Instance, error) {
	if kind != "STRIPE" {
		return nil, errors.New("wrong provider")
	}
	return r.list, nil
}

func TestReconciliationRecoversEnabledConfiguringConnections(t *testing.T) {
	t.Parallel()
	instances := &scopedInstances{list: []integration.Instance{
		{ID: "iin_interrupted", IsEnabled: true, Status: integration.StatusConfiguring},
		{ID: "iin_paused", IsEnabled: false, Status: integration.StatusConfiguring},
		{ID: "iin_error", IsEnabled: true, Status: integration.StatusError},
	}}
	integrations := &recoveryIntegrationService{}
	manager := NewManager(Params{Instances: instances, Integrations: integrations, Logger: zerolog.Nop()})
	require.NoError(t, manager.ReconcileInternal(t.Context()))
	require.Equal(t, []string{"iin_interrupted"}, integrations.calls)
}
