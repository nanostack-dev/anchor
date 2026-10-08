package stripebilling

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/integration"
	"anchor/internal/domain/product"
	"anchor/internal/integration/provider"
	licensesvc "anchor/internal/license/service"
	"anchor/internal/mapper"
	"anchor/internal/repository"
	"anchor/internal/service"
	billing "anchor/internal/stripebilling/billing"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

const (
	actionTimeout            = 3 * time.Minute
	maximumConcurrentActions = 4
	unlockTimeout            = 5 * time.Second
	webhookReceiptTimeout    = 10 * time.Second
)

type Params struct {
	fx.In
	DB            *sql.DB
	Queue         *queue.Client
	Instances     repository.IntegrationInstanceRepository
	Integrations  service.IntegrationService
	Registry      *provider.Registry
	Products      service.ProductService
	Organizations service.OrganizationService
	Templates     licensesvc.LicenseTemplateService
	Licenses      licensesvc.OrganizationLicenseService
	Migrations    licensesvc.LicenseMigrationService
	Logger        zerolog.Logger
}

type Manager struct {
	params  Params
	permits chan struct{}
}

func NewManager(p Params) *Manager {
	return &Manager{params: p, permits: make(chan struct{}, maximumConcurrentActions)}
}

// WithService authorizes the product before resolving encrypted Stripe settings.
// The instance advisory lock serializes mutations across API requests and workers;
// each state update commits independently so external-operation intents survive errors.
func (m *Manager) WithService(
	ctx context.Context,
	tenantID, productID string,
	action func(context.Context, *billing.Service) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	instance, err := m.authorizeInstance(ctx, tenantID, productID)
	if err != nil {
		return err
	}
	return m.withInstance(ctx, instance, action)
}

// ReadState authorizes the same product and provider configuration as mutations,
// then reads committed state without taking the exclusive billing operation lock.
func (m *Manager) ReadState(ctx context.Context, tenantID, productID string) (billing.State, error) {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	instance, err := m.authorizeInstance(ctx, tenantID, productID)
	if err != nil {
		return billing.State{}, err
	}
	svc, err := m.serviceForInstance(ctx, instance, true)
	if err != nil {
		return billing.State{}, err
	}
	return svc.State(ctx)
}

func (m *Manager) authorizeInstance(ctx context.Context, tenantID, productID string) (integration.Instance, error) {
	if _, err := m.params.Products.Get(
		ctx,
		product.GetProductInput{TenantID: tenantID, ProductID: productID},
	); err != nil {
		return integration.Instance{}, err
	}
	found, err := m.params.Instances.FindByProductAndProvider(
		ctx,
		tenantID,
		productID,
		string(integration.ProviderTypeStripe),
	)
	if err != nil {
		return integration.Instance{}, err
	}
	if found.IsAbsent() {
		return integration.Instance{}, fault.NotFound(
			"STRIPE_INTEGRATION_NOT_FOUND",
			"Connect Stripe from Integrations first.",
		)
	}
	return found.Value(), nil
}

func (m *Manager) withInstance(
	ctx context.Context,
	instance integration.Instance,
	action func(context.Context, *billing.Service) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, actionTimeout)
	defer cancel()
	if !instance.IsEnabled || instance.Status != integration.StatusActive {
		return fault.Conflict("STRIPE_INTEGRATION_NOT_ACTIVE", "Enable Stripe and complete its connection setup first.")
	}
	select {
	case m.permits <- struct{}{}:
		defer func() { <-m.permits }()
	case <-ctx.Done():
		return ctx.Err()
	}
	connection, err := m.params.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	var locked bool
	key := "stripe-billing:" + instance.ID
	if err = connection.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended($1,0))", key).
		Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return fault.Conflict("STRIPE_BILLING_BUSY", "Another billing update is running. Try again shortly.")
	}
	defer func() {
		cleanup, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), unlockTimeout)
		defer releaseCancel()
		if _, unlockErr := connection.ExecContext(
			cleanup,
			"SELECT pg_advisory_unlock(hashtextextended($1,0))",
			key,
		); unlockErr != nil {
			m.params.Logger.Error().Err(unlockErr).Msg("failed to release Stripe billing lock")
			// A failed unlock must discard the session rather than return a held lock to the pool.
			_ = connection.Raw(func(any) error { return driver.ErrBadConn })
		}
	}()
	// Reload after locking so disable/config changes cannot use stale credentials.
	current, err := m.params.Instances.FindByID(ctx, instance.PlatformTenantID, instance.ID)
	if err != nil {
		return err
	}
	if current.IsAbsent() || !current.Value().IsEnabled || current.Value().Status != integration.StatusActive {
		return fault.Conflict("STRIPE_INTEGRATION_NOT_ACTIVE", "The Stripe integration is no longer active.")
	}
	svc, err := m.serviceForInstance(ctx, current.Value(), false)
	if err != nil {
		return err
	}
	return action(ctx, svc)
}

type readOnlyStateStore struct{ billing.StateStore }

func (readOnlyStateStore) Update(func(*billing.StoredState) error) error {
	return errors.New("billing state reads cannot perform mutations")
}

func (m *Manager) serviceForInstance(
	ctx context.Context,
	instance integration.Instance,
	readOnly bool,
) (*billing.Service, error) {
	if !instance.IsEnabled || instance.Status != integration.StatusActive {
		return nil, fault.Conflict(
			"STRIPE_INTEGRATION_NOT_ACTIVE",
			"Enable Stripe and complete its connection setup first.",
		)
	}
	config, err := m.resolveConnectionConfig(ctx, instance)
	if err != nil {
		return nil, err
	}
	var stripe billing.StripeGateway
	if config.AuthMethod == "LOCAL_CLI" {
		stripe, err = billing.NewCLI(config.AccountID, nil)
	} else {
		stripe, err = billing.NewSDKGateway(config, nil)
	}
	if err != nil {
		return nil, err
	}
	store := &databaseStore{
		ctx:       ctx,
		db:        m.params.DB,
		queue:     m.params.Queue,
		instance:  instance,
		accountID: config.AccountID,
	}
	if _, err = store.Snapshot(); err != nil {
		return nil, err
	}
	var stateStore billing.StateStore = store
	if readOnly {
		stateStore = readOnlyStateStore{StateStore: store}
	}
	anchor := &anchorGateway{
		tenantID:      instance.PlatformTenantID,
		productID:     instance.ProductID,
		products:      m.params.Products,
		organizations: m.params.Organizations,
		templates:     m.params.Templates,
		licenses:      m.params.Licenses,
		migrations:    m.params.Migrations,
	}
	svc, err := billing.NewService(
		billing.Config{
			ExpectedAccountID: config.AccountID,
			ProductID:         instance.ProductID,
			ReturnURL:         config.ReturnURL,
			WebhookSecret:     config.WebhookSecret,
		},
		stripe,
		anchor,
		stateStore,
	)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func (m *Manager) resolveConnectionConfig(
	ctx context.Context,
	instance integration.Instance,
) (billing.ConnectionConfig, error) {
	selected, err := m.params.Registry.GetProvider(string(integration.ProviderTypeStripe))
	if err != nil {
		return billing.ConnectionConfig{}, err
	}
	resolver, ok := selected.(billing.ConnectionResolver)
	if !ok {
		return billing.ConnectionConfig{}, errors.New("stripe provider cannot resolve billing settings")
	}
	return resolver.ResolveBillingConfig(ctx, instance)
}

// IngestInternal receives signed webhooks addressed to a product. A short shared
// integration row lock captures current configuration and policy with the durable
// event commit, without waiting for the external-operation advisory lock.
func (m *Manager) IngestInternal(ctx context.Context, productID string, payload []byte, signature string) error {
	ctx, cancel := context.WithTimeout(ctx, webhookReceiptTimeout)
	defer cancel()
	found, err := m.params.Instances.FindByProductAndProviderInternal(
		ctx,
		productID,
		string(integration.ProviderTypeStripe),
	)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return fault.NotFound("STRIPE_INTEGRATION_NOT_FOUND", "Stripe integration does not exist.")
	}
	return transactor.New(m.params.DB).InTx(ctx, func(txCtx context.Context) error {
		instance, reloadErr := m.lockWebhookInstance(txCtx, found.Value())
		if reloadErr != nil {
			return reloadErr
		}
		config, resolveErr := m.resolveConnectionConfig(txCtx, instance)
		if resolveErr != nil {
			return resolveErr
		}
		store := &databaseStore{
			ctx: txCtx, db: m.params.DB, queue: m.params.Queue,
			instance: instance, accountID: config.AccountID,
		}
		return billing.ReceiveWebhook(txCtx, billing.Config{
			ExpectedAccountID: config.AccountID,
			ProductID:         instance.ProductID,
			WebhookSecret:     config.WebhookSecret,
		}, store, payload, signature)
	})
}

func (m *Manager) lockWebhookInstance(
	ctx context.Context,
	instance integration.Instance,
) (integration.Instance, error) {
	t := table.IntegrationInstances
	current, err := transactor.QueryOptional[model.IntegrationInstances](ctx, m.params.DB,
		t.SELECT(t.AllColumns).FROM(t).WHERE(
			t.ID.EQ(postgres.String(instance.ID)).
				AND(t.PlatformTenantID.EQ(postgres.String(instance.PlatformTenantID))).
				AND(t.ProductID.EQ(postgres.String(instance.ProductID))).
				AND(t.ProviderType.EQ(postgres.String(string(integration.ProviderTypeStripe)))),
		).FOR(postgres.SHARE()),
	)
	if err != nil {
		return integration.Instance{}, err
	}
	if current.IsAbsent() || !current.Value().IsEnabled || current.Value().Status != string(integration.StatusActive) {
		return integration.Instance{}, fault.Conflict(
			"STRIPE_INTEGRATION_NOT_ACTIVE", "The Stripe integration is no longer active.",
		)
	}
	return mapper.NewIntegrationInstanceMapper().ToDomain(current.Value()), nil
}

// RunInternal handles durable jobs whose instance identity was recorded at receipt.
func (m *Manager) RunInternal(ctx context.Context, instanceID string) error {
	found, err := m.params.Instances.FindByIDInternal(ctx, instanceID)
	if err != nil {
		return err
	}
	if found.IsAbsent() || !found.Value().IsEnabled || found.Value().Status != integration.StatusActive {
		return nil
	}
	return m.withInstance(
		ctx,
		found.Value(),
		func(actionCtx context.Context, s *billing.Service) error { return s.ProcessPending(actionCtx) },
	)
}

// ReconcileInternal repairs missed webhook delivery from canonical Stripe state.
func (m *Manager) ReconcileInternal(ctx context.Context) error {
	instances, err := m.params.Instances.ListByProviderInternal(ctx, string(integration.ProviderTypeStripe))
	if err != nil {
		return err
	}
	// Each iteration performs external reconciliation side effects independently.
	for _, instance := range instances {
		if instance.IsEnabled && instance.Status == integration.StatusConfiguring {
			if recoveryErr := m.params.Integrations.VerifyAndActivateInternal(ctx, instance.ID); recoveryErr != nil {
				m.params.Logger.Warn().Err(recoveryErr).Str("integration_id", instance.ID).
					Msg("Stripe connection verification recovery deferred")
			}
			continue
		}
		if !instance.IsEnabled || instance.Status != integration.StatusActive {
			continue
		}
		if reconcileErr := m.withInstance(
			ctx,
			instance,
			func(_ context.Context, s *billing.Service) error { return s.QueueReconciliation() },
		); reconcileErr != nil {
			m.params.Logger.Warn().
				Err(reconcileErr).
				Str("integration_id", instance.ID).
				Msg("Stripe reconciliation deferred")
		}
	}
	return nil
}
