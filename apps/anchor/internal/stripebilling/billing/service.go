package billing

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/segmentio/ksuid"
	"github.com/stripe/stripe-go/v87"
)

var (
	ErrInput    = errors.New("invalid request")
	ErrConflict = errors.New("billing action cannot be completed")
	ErrNotFound = errors.New("billing resource not found")
)

const (
	statusNone                 = "none"
	statusPending              = "pending"
	statusError                = "error"
	maximumVisibleEvents       = 100
	maximumRetrySeconds        = 60
	maximumRetryExponent       = 6
	maximumPortalProducts      = 10
	stripeInstallationMetadata = "anchor_prototype_id"
	stripeProductMetadata      = "anchor_product_id"
	stripeOrganizationMetadata = "anchor_organization_id"
	stripeSubscriptionPageSize = 100
	maximumStripeSubscriptions = 1000
)

type Config struct {
	ExpectedAccountID string `validate:"required,startswith=acct_"`
	ProductID         string `validate:"required"`
	ReturnURL         string `validate:"required,url"`
	WebhookSecret     string `validate:"required,startswith=whsec_"`
}

type Service struct {
	mu     sync.Mutex
	config Config
	stripe StripeGateway
	anchor AnchorGateway
	store  StateStore
}

func NewService(config Config, stripe StripeGateway, anchor AnchorGateway, store StateStore) (*Service, error) {
	if err := validate.ValidateStruct(config); err != nil {
		return nil, err
	}
	callback, err := url.Parse(config.ReturnURL)
	if err != nil ||
		(callback.Scheme != "https" && (callback.Scheme != "http" || !loopbackHost(callback.Hostname()))) ||
		callback.User != nil ||
		callback.RawQuery != "" ||
		callback.Fragment != "" {
		return nil, errors.New(
			"the billing return URL must be HTTPS or an HTTP loopback origin without credentials, query or fragment",
		)
	}
	if stripe == nil || anchor == nil || store == nil {
		return nil, errors.New("stripe, Anchor and persistent state are required")
	}
	return &Service{config: config, stripe: stripe, anchor: anchor, store: store}, nil
}

func (s *Service) State(ctx context.Context) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stateLocked(ctx)
}

func (s *Service) stateLocked(ctx context.Context) (State, error) {
	account, err := s.stripe.Account(ctx)
	if err != nil {
		return State{}, err
	}
	snapshot, err := s.anchor.Snapshot(ctx)
	if err != nil {
		return State{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return State{}, err
	}
	organizations := functional.Slice(snapshot.Organizations).Map(func(source AnchorOrganization) Organization {
		organization := stored.Organizations[source.ID].Organization
		organization.ID, organization.Name = source.ID, source.Name
		organization.TemplateID, organization.LicenseValues = source.TemplateID, source.LicenseValues
		if organization.Status == "" {
			organization.Status, organization.SyncState = statusNone, "unlinked"
		}
		return organization
	}).SortedBy(func(organization Organization) string { return organization.Name })
	prices := functional.Slice(slices.Collect(maps.Values(stored.Prices))).
		SortedBy(func(price Price) string { return price.Name })
	events := functional.Slice(slices.Collect(maps.Values(stored.Events))).
		Map(func(event eventRecord) BillingEvent { return event.BillingEvent })
	slices.SortFunc(events, func(a, b BillingEvent) int { return b.ReceivedAt.Compare(a.ReceivedAt) })
	if len(events) > maximumVisibleEvents {
		events = events[:maximumVisibleEvents]
	}
	refunds := functional.Slice(slices.Collect(maps.Values(stored.FraudRefunds))).
		Map(func(record fraudRefundRecord) FraudRefund { return record.FraudRefund })
	slices.SortFunc(refunds, func(a, b FraudRefund) int { return b.CreatedAt.Compare(a.CreatedAt) })
	if len(refunds) > maximumVisibleEvents {
		refunds = refunds[:maximumVisibleEvents]
	}
	return State{Account: account, Product: snapshot.Product, Templates: append([]Template{}, snapshot.Templates...),
		Organizations: append([]Organization{}, organizations...), Prices: append([]Price{}, prices...),
		Settings: normalizedSettings(stored.Settings), Events: append([]BillingEvent{}, events...),
		FraudRefunds: append([]FraudRefund{}, refunds...)}, nil
}

func (s *Service) CreatePrice(ctx context.Context, request CreatePriceRequest) (Price, error) {
	if err := validate.ValidateStruct(struct {
		Name       string `validate:"required,min=1,max=120"`
		TemplateID string `validate:"required"`
		Amount     int64  `validate:"gte=1,lte=99999999"`
		Currency   string `validate:"oneof=usd cad eur"`
		Interval   string `validate:"oneof=month year"`
	}{strings.TrimSpace(request.Name), request.TemplateID, request.Amount, string(request.Currency), string(request.Interval)}); err != nil {
		return Price{}, fmt.Errorf("%w: enter a name, template, supported currency and recurring amount", ErrInput)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireTemplate(ctx, request.TemplateID); err != nil {
		return Price{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return Price{}, err
	}
	request.Name = strings.TrimSpace(request.Name)
	intentID := functional.Slice(slices.Collect(maps.Keys(stored.PriceIntents))).FindFirst(func(id string) bool {
		return stored.PriceIntents[id] == request
	}).OrElse(ksuid.New().String())
	if err = s.store.Update(
		func(state *StoredState) error { state.PriceIntents[intentID] = request; return nil },
	); err != nil {
		return Price{}, err
	}
	productID, err := s.ensureStripeProduct(ctx, request, stored)
	if err != nil {
		return Price{}, err
	}
	remote, err := s.stripe.Client().V1Prices.Create(ctx, &stripe.PriceCreateParams{
		IdempotencyKey: stripe.String(stored.InstallationID + "-price-" + intentID),
		Product: stripe.String(
			productID,
		),
		UnitAmount: new(request.Amount),
		Currency:   stripe.String(string(request.Currency)),
		Recurring: &stripe.PriceCreateRecurringParams{
			Interval: stripe.String(string(request.Interval)),
		},
		Nickname: stripe.String(request.Name),
		Metadata: map[string]string{"anchor_price_id": intentID, stripeInstallationMetadata: stored.InstallationID},
	})
	if err != nil {
		return Price{}, err
	}
	price := Price{
		ID:         intentID,
		Name:       request.Name,
		TemplateID: request.TemplateID,
		Amount:     request.Amount,
		Currency: PriceCurrency(
			request.Currency,
		),
		Interval:        PriceInterval(request.Interval),
		StripePriceID:   remote.ID,
		StripeProductID: productID,
		Active:          true,
	}
	err = s.store.Update(func(state *StoredState) error {
		state.Prices[price.ID] = price
		delete(state.PriceIntents, intentID)
		return nil
	})
	return price, err
}

func (s *Service) ArchivePrice(ctx context.Context, priceID string) (Price, error) {
	if err := validateID(priceID); err != nil {
		return Price{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	price, err := s.price(priceID, false)
	if err != nil {
		if errors.Is(err, ErrInput) {
			return Price{}, fmt.Errorf("%w: this price does not belong to the product", ErrNotFound)
		}
		return Price{}, err
	}
	if _, err = s.stripe.Client().V1Prices.Update(
		ctx,
		price.StripePriceID,
		&stripe.PriceUpdateParams{Active: new(false)},
	); err != nil {
		return Price{}, err
	}
	price.Active = false
	err = s.store.Update(func(state *StoredState) error { state.Prices[price.ID] = price; return nil })
	return price, err
}

func (s *Service) UpdateSettings(ctx context.Context, request UpdateSettingsRequest) (Settings, error) {
	if request.FallbackTemplateID == nil && request.FraudRefundPolicy == nil {
		return Settings{}, ErrInput
	}
	if request.FallbackTemplateID != nil {
		if err := validateID(*request.FallbackTemplateID); err != nil {
			return Settings{}, err
		}
	}
	if request.FraudRefundPolicy != nil {
		policy := request.FraudRefundPolicy
		if err := validate.ValidateStruct(struct {
			Currency  string `validate:"oneof=usd cad eur"`
			MaxAmount int64  `validate:"gte=0,lte=99999999"`
		}{string(policy.Currency), policy.MaxAmount}); err != nil || policy.Enabled && policy.MaxAmount == 0 {
			return Settings{}, fmt.Errorf(
				"%w: select a supported currency and a positive payment limit when enabled",
				ErrInput,
			)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if request.FallbackTemplateID != nil {
		if err := s.requireTemplate(ctx, *request.FallbackTemplateID); err != nil {
			return Settings{}, err
		}
	}
	var settings Settings
	err := s.store.Update(func(state *StoredState) error {
		settings = normalizedSettings(state.Settings)
		if request.FallbackTemplateID != nil {
			settings.FallbackTemplateID = *request.FallbackTemplateID
		}
		if request.FraudRefundPolicy != nil {
			settings.FraudRefundPolicy = *request.FraudRefundPolicy
		}
		state.Settings = settings
		return nil
	})
	return settings, err
}

func (s *Service) Checkout(ctx context.Context, organizationID string, request CheckoutRequest) (URLResponse, error) {
	if err := validateID(organizationID); err != nil {
		return URLResponse{}, err
	}
	if err := validate.ValidateStruct(struct {
		PriceID string `validate:"required"`
		Trial   int    `validate:"gte=0,lte=30"`
	}{request.PriceID, request.TrialDays}); err != nil {
		return URLResponse{}, ErrInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	price, err := s.price(request.PriceID, true)
	if err != nil {
		return URLResponse{}, err
	}
	if err = s.requireTemplate(ctx, price.TemplateID); err != nil {
		return URLResponse{}, err
	}
	_, err = s.ensureCustomer(ctx, organizationID)
	if err != nil {
		return URLResponse{}, err
	}
	if _, err = s.syncLocked(ctx, organizationID); err != nil {
		return URLResponse{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return URLResponse{}, err
	}
	organization := stored.Organizations[organizationID]
	if organization.SubscriptionID != "" && !terminal(organization.Status) {
		return URLResponse{}, fmt.Errorf(
			"%w: this organization already has a subscription; change its price instead",
			ErrConflict,
		)
	}
	if organization.CheckoutID == "" && organization.CheckoutIntent != "" &&
		(organization.CheckoutPrice != request.PriceID || organization.CheckoutTrial != request.TrialDays) {
		return URLResponse{}, fmt.Errorf(
			"%w: retry the previous checkout price and trial first to recover its pending session",
			ErrConflict,
		)
	}
	if organization.CheckoutID != "" {
		existing, recoveryErr := s.recoverCheckout(ctx, &organization, request)
		if recoveryErr != nil {
			return URLResponse{}, recoveryErr
		}
		if existing.IsPresent() {
			return existing.Value(), nil
		}
	}
	if organization.CheckoutIntent == "" || organization.CheckoutPrice != request.PriceID ||
		organization.CheckoutTrial != request.TrialDays {
		organization.CheckoutIntent = ksuid.New().String()
	}
	organization.CheckoutPrice, organization.CheckoutTrial = request.PriceID, request.TrialDays
	if err = s.saveOrganization(organization); err != nil {
		return URLResponse{}, err
	}
	params := &stripe.CheckoutSessionCreateParams{
		IdempotencyKey: stripe.String(organization.CheckoutIntent),
		Mode:           stripe.String("subscription"),
		Customer:       stripe.String(organization.CustomerID),
		LineItems: []*stripe.CheckoutSessionCreateLineItemParams{
			{Price: stripe.String(price.StripePriceID), Quantity: new(int64(1))},
		},
		ClientReferenceID: stripe.String(
			organizationID,
		),
		SuccessURL: stripe.String(s.organizationReturnURL(organizationID) + "?checkout=success"),
		CancelURL:  stripe.String(s.organizationReturnURL(organizationID) + "?checkout=canceled"),
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{
			BillingMode: &stripe.CheckoutSessionCreateSubscriptionDataBillingModeParams{Type: stripe.String("classic")},
			Metadata: map[string]string{
				stripeInstallationMetadata: stored.InstallationID,
				stripeOrganizationMetadata: organizationID,
				stripeProductMetadata:      s.config.ProductID,
			},
		},
	}
	if request.TrialDays > 0 {
		params.SubscriptionData.TrialPeriodDays = new(int64(request.TrialDays))
	}
	session, err := s.stripe.Client().V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return URLResponse{}, err
	}
	organization.CheckoutID = session.ID
	if err = s.saveOrganization(organization); err != nil {
		return URLResponse{}, err
	}
	return URLResponse{URL: session.URL}, nil
}

func (s *Service) ensureCustomer(ctx context.Context, organizationID string) (organizationRecord, error) {
	source, err := s.organizationSource(ctx, organizationID)
	if err != nil {
		return organizationRecord{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return organizationRecord{}, err
	}
	organization := stored.Organizations[organizationID]
	organization.ID, organization.Name = source.ID, source.Name
	organization.TemplateID, organization.LicenseValues = source.TemplateID, source.LicenseValues
	if organization.CustomerID != "" {
		return organization, nil
	}
	if organization.CustomerIntent == "" {
		organization.CustomerIntent = ksuid.New().String()
		organization.CustomerName = source.Name
	}
	if err = s.saveOrganization(organization); err != nil {
		return organizationRecord{}, err
	}
	customer, err := s.stripe.Client().V1Customers.Create(ctx, &stripe.CustomerCreateParams{
		IdempotencyKey: stripe.String(organization.CustomerIntent),
		Name:           stripe.String(organization.CustomerName),
		Metadata: map[string]string{
			stripeOrganizationMetadata: organizationID,
			stripeProductMetadata:      s.config.ProductID,
			stripeInstallationMetadata: stored.InstallationID,
		},
	})
	if err != nil {
		return organizationRecord{}, err
	}
	organization.CustomerID, organization.Status, organization.SyncState = customer.ID, statusNone, statusPending
	err = s.saveOrganization(organization)
	return organization, err
}

func (s *Service) SyncOrganization(ctx context.Context, organizationID string) (Organization, error) {
	if err := validateID(organizationID); err != nil {
		return Organization{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	organization, err := s.syncLocked(ctx, organizationID)
	if err != nil {
		err = errors.Join(err, s.markSyncError(organizationID, err))
	}
	return organization, err
}

func (s *Service) syncLocked(ctx context.Context, organizationID string) (Organization, error) {
	source, err := s.organizationSource(ctx, organizationID)
	if err != nil {
		return Organization{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return Organization{}, err
	}
	organization, exists := stored.Organizations[organizationID]
	if !exists || organization.CustomerID == "" {
		return Organization{
			ID:            source.ID,
			Name:          source.Name,
			TemplateID:    source.TemplateID,
			LicenseValues: source.LicenseValues,
			Status:        statusNone,
			SyncState:     "unlinked",
		}, nil
	}
	owned, err := s.ownedSubscriptions(ctx, organization, stored)
	if err != nil {
		return Organization{}, err
	}

	current, err := canonicalSubscription(owned, organization.SubscriptionID)
	if err != nil {
		return Organization{}, err
	}
	if current.ID == "" {
		if organization.SubscriptionID != "" {
			return Organization{}, errors.New("the linked subscription was not found in its sandbox customer")
		}
		organization.Status, organization.SyncState = statusNone, "synced"
		now := time.Now().UTC()
		organization.LastSyncedAt = &now
		if err = s.saveOrganization(organization); err != nil {
			return Organization{}, err
		}
		return organization.Organization, nil
	}
	if !validSubscriptionItem(current) {
		return Organization{}, errors.New("billing supports one recurring price per organization")
	}
	price, err := functional.Slice(slices.Collect(maps.Values(stored.Prices))).
		FindFirst(func(price Price) bool { return price.StripePriceID == subscriptionItems(current)[0].Price.ID }).
		ToResult(errors.New("subscription price is not mapped to an Anchor license template")).
		Value()
	if err != nil {
		return Organization{}, err
	}
	organization.SubscriptionID, organization.Status, organization.PriceID = current.ID, string(
		current.Status,
	), price.ID
	organization.CancelAtPeriodEnd = current.CancelAtPeriodEnd || current.CancelAt > 0
	organization.PendingUpdate = current.PendingUpdate != nil
	if end := subscriptionItems(current)[0].CurrentPeriodEnd; end > 0 {
		value := time.Unix(end, 0).UTC()
		organization.CurrentPeriodEnd = &value
	}
	organization.SyncState, organization.SyncError = statusPending, ""
	if err = s.saveOrganization(organization); err != nil {
		return Organization{}, err
	}
	targetTemplate := ""
	switch current.Status {
	case stripe.SubscriptionStatusActive, stripe.SubscriptionStatusTrialing:
		targetTemplate = price.TemplateID
	case stripe.SubscriptionStatusCanceled,
		stripe.SubscriptionStatusUnpaid,
		stripe.SubscriptionStatusPaused,
		stripe.SubscriptionStatusIncompleteExpired:
		targetTemplate = stored.Settings.FallbackTemplateID
	case stripe.SubscriptionStatusPastDue, stripe.SubscriptionStatusIncomplete:
	default:
		return Organization{}, fmt.Errorf("unknown Stripe subscription status %q", current.Status)
	}
	if targetTemplate != "" {
		if err = s.requireTemplate(ctx, targetTemplate); err != nil {
			return Organization{}, err
		}
		if err = s.anchor.ApplyTemplate(ctx, organizationID, targetTemplate); err != nil {
			return Organization{}, err
		}
	} else if current.Status != "past_due" && current.Status != "incomplete" {
		return Organization{}, errors.New("configure a fallback license template before reconciling this subscription")
	}
	source, err = s.organizationSource(ctx, organizationID)
	if err != nil {
		return Organization{}, err
	}
	organization.TemplateID, organization.LicenseValues = source.TemplateID, source.LicenseValues
	now := time.Now().UTC()
	organization.LastSyncedAt = &now
	organization.SyncState, organization.SyncError = "synced", ""
	if err = s.saveOrganization(organization); err != nil {
		return Organization{}, err
	}
	return organization.Organization, nil
}

func (s *Service) ChangeSubscription(
	ctx context.Context,
	organizationID string,
	request SubscriptionRequest,
) (Organization, error) {
	if err := validateID(organizationID); err != nil {
		return Organization{}, err
	}
	if err := validateID(request.PriceID); err != nil {
		return Organization{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	price, err := s.price(request.PriceID, true)
	if err != nil {
		return Organization{}, err
	}
	if err = s.requireTemplate(ctx, price.TemplateID); err != nil {
		return Organization{}, err
	}
	organization, err := s.syncLocked(ctx, organizationID)
	if err != nil {
		return Organization{}, err
	}
	if organization.SubscriptionID == "" || terminal(organization.Status) {
		return Organization{}, fmt.Errorf("%w: start checkout for a new subscription", ErrConflict)
	}
	current, err := s.stripe.Client().V1Subscriptions.Retrieve(ctx, organization.SubscriptionID, nil)
	if err != nil {
		return Organization{}, err
	}
	if !validSubscriptionItem(current) {
		return Organization{}, ErrConflict
	}
	if _, err = s.stripe.Client().V1Subscriptions.Update(
		ctx,
		organization.SubscriptionID,
		&stripe.SubscriptionUpdateParams{
			IdempotencyKey: stripe.String(ksuid.New().String()),
			Items: []*stripe.SubscriptionUpdateItemParams{
				{ID: stripe.String(subscriptionItems(current)[0].ID), Price: stripe.String(price.StripePriceID)},
			},
			PaymentBehavior:   stripe.String("pending_if_incomplete"),
			ProrationBehavior: stripe.String("always_invoice"),
		},
	); err != nil {
		return Organization{}, err
	}
	return s.syncLocked(ctx, organizationID)
}

func (s *Service) SetCancellation(ctx context.Context, organizationID string, cancel bool) (Organization, error) {
	if err := validateID(organizationID); err != nil {
		return Organization{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	organization, err := s.syncLocked(ctx, organizationID)
	if err != nil {
		return Organization{}, err
	}
	if organization.SubscriptionID == "" || terminal(organization.Status) {
		return Organization{}, fmt.Errorf("%w: no current subscription to change", ErrConflict)
	}
	if _, err = s.stripe.Client().V1Subscriptions.Update(ctx, organization.SubscriptionID,
		&stripe.SubscriptionUpdateParams{CancelAtPeriodEnd: new(cancel)}); err != nil {
		return Organization{}, err
	}
	return s.syncLocked(ctx, organizationID)
}

func (s *Service) Portal(ctx context.Context, organizationID string) (URLResponse, error) {
	if err := validateID(organizationID); err != nil {
		return URLResponse{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.organizationSource(ctx, organizationID); err != nil {
		return URLResponse{}, err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return URLResponse{}, err
	}
	organization := stored.Organizations[organizationID]
	if organization.CustomerID == "" {
		return URLResponse{}, fmt.Errorf("%w: start checkout to create a billing customer", ErrConflict)
	}
	prices := functional.Slice(slices.Collect(maps.Values(stored.Prices))).
		Filter(func(price Price) bool { return price.Active })
	slices.SortFunc(prices, func(a, b Price) int { return strings.Compare(b.ID, a.ID) })
	prices = prices.UniqueBy(func(price Price) string {
		return price.StripeProductID + "/" + string(price.Currency) + "/" + string(price.Interval)
	})
	if len(
		prices.Map(func(price Price) string { return price.StripeProductID }).
			UniqueBy(func(id string) string { return id }),
	) > maximumPortalProducts {
		prices = []Price{}
	}
	update := &stripe.BillingPortalConfigurationCreateFeaturesSubscriptionUpdateParams{
		Enabled: new(len(prices) > 0),
	}
	if len(prices) > 0 {
		update.DefaultAllowedUpdates = []*string{stripe.String("price")}
		update.ProrationBehavior = stripe.String("always_invoice")
		productIDs := prices.Map(func(price Price) string { return price.StripeProductID }).
			UniqueBy(func(id string) string { return id })
		for _, productID := range productIDs {
			group := prices.Filter(func(price Price) bool { return price.StripeProductID == productID })
			update.Products = append(
				update.Products,
				&stripe.BillingPortalConfigurationCreateFeaturesSubscriptionUpdateProductParams{
					Product: stripe.String(
						productID,
					),
					Prices: group.Map(func(price Price) *string { return stripe.String(price.StripePriceID) }),
				},
			)
		}
	}
	configuration, err := s.stripe.Client().V1BillingPortalConfigurations.Create(
		ctx,
		&stripe.BillingPortalConfigurationCreateParams{
			BusinessProfile: &stripe.BillingPortalConfigurationCreateBusinessProfileParams{
				Headline: stripe.String("Manage your Anchor organization subscription"),
			},
			DefaultReturnURL: stripe.String(s.config.ReturnURL),
			Features: &stripe.BillingPortalConfigurationCreateFeaturesParams{
				InvoiceHistory: &stripe.BillingPortalConfigurationCreateFeaturesInvoiceHistoryParams{
					Enabled: new(true),
				},
				PaymentMethodUpdate: &stripe.BillingPortalConfigurationCreateFeaturesPaymentMethodUpdateParams{
					Enabled: new(true),
				},
				CustomerUpdate: &stripe.BillingPortalConfigurationCreateFeaturesCustomerUpdateParams{
					Enabled:        new(true),
					AllowedUpdates: []*string{stripe.String("email")},
				},
				SubscriptionCancel: &stripe.BillingPortalConfigurationCreateFeaturesSubscriptionCancelParams{
					Enabled: new(true),
					Mode:    stripe.String("at_period_end"),
				},
				SubscriptionUpdate: update,
			},
		},
	)
	if err != nil {
		return URLResponse{}, err
	}
	session, err := s.stripe.Client().V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer: stripe.String(
			organization.CustomerID,
		),
		Configuration: stripe.String(configuration.ID),
		ReturnURL:     stripe.String(s.organizationReturnURL(organizationID)),
	})
	if err != nil {
		return URLResponse{}, err
	}
	return URLResponse{URL: session.URL}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	return ReceiveWebhook(ctx, s.config, s.store, body, signature)
}

func (s *Service) ProcessPending(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	jobs := functional.Slice(slices.Collect(maps.Values(stored.Events))).Filter(func(event eventRecord) bool {
		return (event.Status == statusPending || event.Status == statusError) && !event.NextAttempt.After(time.Now())
	}).SortedBy(func(event eventRecord) int64 { return event.ReceivedAt.UnixNano() })
	for _, job := range jobs {
		if err = ctx.Err(); err != nil {
			return err
		}
		var syncErr error
		if financialEvent(job.Type) {
			syncErr = s.processFinancialEvent(ctx, job)
		} else {
			_, syncErr = s.syncLocked(ctx, job.OrganizationID)
			if syncErr != nil {
				syncErr = errors.Join(syncErr, s.markSyncError(job.OrganizationID, syncErr))
			}
		}
		if err = s.store.Update(func(state *StoredState) error {
			event := state.Events[job.ID]
			event.Attempts++
			if syncErr == nil {
				event.Status, event.LastError = "processed", ""
			} else {
				event.Status, event.LastError = statusError, syncErr.Error()
				if financialEvent(job.Type) {
					event.LastError = "Stripe could not confirm the financial action; Anchor will reconcile it safely."
				}
				event.NextAttempt = time.Now().
					Add(time.Duration(min(maximumRetrySeconds, 1<<min(event.Attempts, maximumRetryExponent))) * time.Second)
			}
			state.Events[job.ID] = event
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) price(id string, requireActive bool) (Price, error) {
	stored, err := s.store.Snapshot()
	if err != nil {
		return Price{}, err
	}
	price, exists := stored.Prices[id]
	if !exists || requireActive && !price.Active {
		return Price{}, fmt.Errorf("%w: select an active price from this product", ErrInput)
	}
	return price, nil
}

func (s *Service) organizationSource(ctx context.Context, id string) (AnchorOrganization, error) {
	snapshot, err := s.anchor.Snapshot(ctx)
	if err != nil {
		return AnchorOrganization{}, err
	}
	return functional.Slice(snapshot.Organizations).
		FindFirst(func(organization AnchorOrganization) bool { return organization.ID == id }).
		ToResult(fmt.Errorf("%w: this organization does not belong to the product", ErrNotFound)).
		Value()
}

func (s *Service) requireTemplate(ctx context.Context, id string) error {
	snapshot, err := s.anchor.Snapshot(ctx)
	if err != nil {
		return err
	}
	if functional.Slice(snapshot.Templates).
		FindFirst(func(template Template) bool { return template.ID == id && !template.Archived }).
		IsAbsent() {
		return fmt.Errorf("%w: select an active license template from this product", ErrInput)
	}
	return nil
}

func (s *Service) saveOrganization(organization organizationRecord) error {
	return s.store.Update(
		func(state *StoredState) error { state.Organizations[organization.ID] = organization; return nil },
	)
}

func (s *Service) markSyncError(id string, cause error) error {
	return s.store.Update(func(state *StoredState) error {
		organization, exists := state.Organizations[id]
		if exists {
			organization.SyncState, organization.SyncError = statusError, cause.Error()
			state.Organizations[id] = organization
		}
		return nil
	})
}

func validateID(id string) error {
	if err := validate.ValidateStruct(struct {
		ID string `validate:"required"`
	}{id}); err != nil {
		return ErrInput
	}
	if !validAnchorID(id) {
		return ErrInput
	}
	return nil
}

func terminal(status string) bool { return status == "canceled" || status == "incomplete_expired" }

func (s *Service) queueLinkedOrganizations() error {
	return s.store.Update(func(state *StoredState) error {
		for _, organization := range state.Organizations {
			if organization.CustomerID == "" {
				continue
			}
			now := time.Now().UTC()
			id := ksuid.New().String()
			state.Events[id] = eventRecord{
				ID: id, Type: "reconciliation.scheduled", OrganizationID: organization.ID,
				Status: statusPending, ReceivedAt: now,
				NextAttempt: now,
			}
		}
		for _, refund := range state.FraudRefunds {
			if refund.IdempotencyKey == "" || refund.Status == FraudRefundSkipped ||
				refund.Status == FraudRefundSucceeded ||
				refund.Status == FraudRefundFailed ||
				refund.Status == FraudRefundCanceled {
				continue
			}
			id := "fraud-reconcile-" + refund.ID
			if event, exists := state.Events[id]; exists &&
				(event.Status == statusPending || event.Status == statusError) {
				continue
			}
			now := time.Now().UTC()
			state.Events[id] = eventRecord{
				ID:             id,
				Type:           fraudReconciliation,
				OrganizationID: refund.OrganizationID,
				Status:         statusPending,
				ReceivedAt:     now,
				ChargeID:       refund.ChargeID,
				ResourceID:     refund.RefundID,
				NextAttempt:    now,
			}
		}
		queueTerminalRefundMonitoring(state)
		return nil
	})
}

func (s *Service) recoverCheckout(
	ctx context.Context,
	organization *organizationRecord,
	request CheckoutRequest,
) (functional.Option[URLResponse], error) {
	existing, err := s.stripe.Client().V1CheckoutSessions.Retrieve(ctx, organization.CheckoutID, nil)
	if err != nil {
		return functional.None[URLResponse](), err
	}
	if existing.Status == "complete" {
		if err = s.retireCompletedCheckout(ctx, organization, existing); err != nil {
			return functional.None[URLResponse](), err
		}
	}
	if existing.Status == "open" {
		if organization.CheckoutPrice == request.PriceID && organization.CheckoutTrial == request.TrialDays {
			return functional.Some(URLResponse{URL: existing.URL}), nil
		}
		if _, err = s.stripe.Client().V1CheckoutSessions.Expire(ctx, organization.CheckoutID, nil); err != nil {
			return functional.None[URLResponse](), err
		}
	}
	organization.CheckoutID, organization.CheckoutIntent = "", ""
	return functional.None[URLResponse](), nil
}

func (s *Service) retireCompletedCheckout(
	ctx context.Context,
	organization *organizationRecord,
	session *stripe.CheckoutSession,
) error {
	if _, err := s.syncLocked(ctx, organization.ID); err != nil {
		return err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	current := stored.Organizations[organization.ID]
	if session.Livemode || session.ID != current.CheckoutID ||
		(session.Customer == nil || session.Customer.ID != current.CustomerID) ||
		session.ClientReferenceID != current.ID ||
		session.Subscription == nil ||
		session.Subscription.ID != current.SubscriptionID ||
		!terminal(current.Status) {
		return fmt.Errorf(
			"%w: completed checkout needs a reconciled terminal subscription before replacement",
			ErrConflict,
		)
	}
	previous, err := s.stripe.Client().V1Subscriptions.Retrieve(ctx, current.SubscriptionID, nil)
	if err != nil {
		return err
	}
	if previous.ID != current.SubscriptionID || !terminal(string(previous.Status)) || previous.Livemode ||
		(previous.Customer == nil || previous.Customer.ID != current.CustomerID) || previous.Metadata[stripeInstallationMetadata] != stored.InstallationID ||
		previous.Metadata[stripeOrganizationMetadata] != current.ID || previous.Metadata[stripeProductMetadata] != s.config.ProductID {
		return fmt.Errorf(
			"%w: completed checkout subscription ownership or terminal state could not be verified",
			ErrConflict,
		)
	}
	// Reconciliation may have changed license receipts; never save the older record.
	*organization = current
	return nil
}

func (s *Service) ownedSubscriptions(
	ctx context.Context,
	organization organizationRecord,
	stored StoredState,
) (functional.Seq[*stripe.Subscription], error) {
	subscriptions := []*stripe.Subscription{}
	params := &stripe.SubscriptionListParams{
		Limit:    new(int64(stripeSubscriptionPageSize)),
		Customer: stripe.String(organization.CustomerID),
		Status:   stripe.String("all"),
	}
	for {
		page := s.stripe.Client().V1Subscriptions.List(ctx, params)
		if err := page.Err(); err != nil {
			return nil, err
		}
		data := page.Data()
		subscriptions = append(subscriptions, data...)
		if len(subscriptions) > maximumStripeSubscriptions {
			return nil, errors.New("unexpected Stripe subscription pagination")
		}
		if !page.Meta().HasMore {
			break
		}
		if len(data) == 0 || data[len(data)-1] == nil || data[len(data)-1].ID == "" {
			return nil, errors.New("unexpected Stripe subscription pagination")
		}
		params.StartingAfter = stripe.String(data[len(data)-1].ID)
	}
	return functional.Slice(subscriptions).Filter(func(subscription *stripe.Subscription) bool {
		return subscription != nil && subscription.Customer != nil &&
			subscription.Customer.ID == organization.CustomerID &&
			!subscription.Livemode &&
			subscription.Metadata[stripeInstallationMetadata] == stored.InstallationID &&
			subscription.Metadata[stripeOrganizationMetadata] == organization.ID &&
			subscription.Metadata[stripeProductMetadata] == s.config.ProductID
	}), nil
}

func (s *Service) ensureStripeProduct(
	ctx context.Context,
	request CreatePriceRequest,
	stored StoredState,
) (string, error) {
	if productID := stored.StripeProducts[request.TemplateID]; productID != "" {
		return productID, nil
	}
	if stored.ProductNames[request.TemplateID] == "" {
		snapshot, err := s.anchor.Snapshot(ctx)
		if err != nil {
			return "", err
		}
		templateName := functional.Slice(snapshot.Templates).
			FindFirst(func(template Template) bool { return template.ID == request.TemplateID }).
			Map(func(template Template) string { return template.Name }).OrElse(request.Name)
		stored.ProductNames[request.TemplateID] = "Anchor · " + templateName
		if err = s.store.Update(func(state *StoredState) error {
			state.ProductNames[request.TemplateID] = stored.ProductNames[request.TemplateID]
			return nil
		}); err != nil {
			return "", err
		}
	}
	product, err := s.stripe.Client().V1Products.Create(ctx, &stripe.ProductCreateParams{
		IdempotencyKey: stripe.String(stored.InstallationID + "-product-" + request.TemplateID),
		Name:           stripe.String(stored.ProductNames[request.TemplateID]),
		Metadata: map[string]string{
			stripeProductMetadata:      s.config.ProductID,
			"anchor_template_id":       request.TemplateID,
			stripeInstallationMetadata: stored.InstallationID,
		},
	})
	if err != nil {
		return "", err
	}
	if err = s.store.Update(func(state *StoredState) error {
		state.StripeProducts[request.TemplateID] = product.ID
		return nil
	}); err != nil {
		return "", err
	}
	return product.ID, nil
}

func canonicalSubscription(owned functional.Seq[*stripe.Subscription], savedID string) (*stripe.Subscription, error) {
	active := owned.Filter(func(value *stripe.Subscription) bool { return !terminal(string(value.Status)) })
	if len(active) > 1 {
		return &stripe.Subscription{}, fmt.Errorf(
			"%w: multiple current subscriptions need manual resolution in Stripe",
			ErrConflict,
		)
	}
	if len(active) == 1 {
		return active[0], nil
	}
	if savedID != "" {
		return owned.FindFirst(func(value *stripe.Subscription) bool { return value.ID == savedID }).
				OrElse(&stripe.Subscription{}),
			nil
	}
	// A terminal subscription may be the first canonical state observed after downtime.
	return owned.FoldLeft(&stripe.Subscription{}, func(latest, candidate *stripe.Subscription) *stripe.Subscription {
		if candidate.Created > latest.Created || latest.ID == "" {
			return candidate
		}
		return latest
	}), nil
}

func (s *Service) organizationReturnURL(organizationID string) string {
	return strings.TrimRight(s.config.ReturnURL, "/") + "/organizations/license/" + organizationID + "/billing"
}

// QueueReconciliation records missed-event repair as durable work in the state store.
func (s *Service) QueueReconciliation() error { return s.queueLinkedOrganizations() }

func subscriptionItems(subscription *stripe.Subscription) []*stripe.SubscriptionItem {
	if subscription == nil || subscription.Items == nil {
		return nil
	}
	return subscription.Items.Data
}

func validSubscriptionItem(subscription *stripe.Subscription) bool {
	items := subscriptionItems(subscription)
	return len(items) == 1 && items[0] != nil && items[0].ID != "" && items[0].Price != nil && items[0].Price.ID != ""
}
