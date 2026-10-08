package stripeprototype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/segmentio/ksuid"
	"github.com/stripe/stripe-go/v87/webhook"
)

var (
	ErrInput    = errors.New("invalid request")
	ErrConflict = errors.New("billing action cannot be completed")
	ErrNotFound = errors.New("billing resource not found")
)

const (
	statusNone            = "none"
	statusPending         = "pending"
	statusError           = "error"
	metadataInstallation  = "metadata[anchor_prototype_id]"
	stripeCustomer        = "customer"
	stripeTrue            = "true"
	maximumVisibleEvents  = 100
	maximumRetrySeconds   = 60
	maximumRetryExponent  = 6
	maximumPortalProducts = 10
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
	return State{Account: account, Product: snapshot.Product, Templates: append([]Template{}, snapshot.Templates...),
		Organizations: append([]Organization{}, organizations...), Prices: append([]Price{}, prices...),
		Settings: stored.Settings, Events: append([]BillingEvent{}, events...)}, nil
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
	var remote stripeObject
	err = s.stripe.Request(ctx, "post", "/v1/prices", map[string]string{
		"product":                   productID,
		"unit_amount":               strconv.FormatInt(request.Amount, 10),
		"currency":                  string(request.Currency),
		"recurring[interval]":       string(request.Interval),
		"nickname":                  request.Name,
		"metadata[anchor_price_id]": intentID,
		metadataInstallation:        stored.InstallationID,
	}, stored.InstallationID+"-price-"+intentID, &remote)
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
	if err = s.stripe.Request(
		ctx,
		"post",
		"/v1/prices/"+price.StripePriceID,
		map[string]string{"active": "false"},
		"",
		nil,
	); err != nil {
		return Price{}, err
	}
	price.Active = false
	err = s.store.Update(func(state *StoredState) error { state.Prices[price.ID] = price; return nil })
	return price, err
}

func (s *Service) UpdateSettings(ctx context.Context, request UpdateSettingsRequest) (Settings, error) {
	if err := validateID(request.FallbackTemplateID); err != nil {
		return Settings{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireTemplate(ctx, request.FallbackTemplateID); err != nil {
		return Settings{}, err
	}
	settings := Settings(request)
	err := s.store.Update(func(state *StoredState) error { state.Settings = settings; return nil })
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
	params := map[string]string{
		"mode":                                  "subscription",
		stripeCustomer:                          organization.CustomerID,
		"line_items[0][price]":                  price.StripePriceID,
		"line_items[0][quantity]":               "1",
		"client_reference_id":                   organizationID,
		"success_url":                           s.organizationReturnURL(organizationID) + "?checkout=success",
		"cancel_url":                            s.organizationReturnURL(organizationID) + "?checkout=canceled",
		"subscription_data[billing_mode][type]": "classic",
		"subscription_data[metadata][anchor_prototype_id]":    stored.InstallationID,
		"subscription_data[metadata][anchor_organization_id]": organizationID,
		"subscription_data[metadata][anchor_product_id]":      s.config.ProductID,
	}
	if request.TrialDays > 0 {
		params["subscription_data[trial_period_days]"] = strconv.Itoa(request.TrialDays)
	}
	var session stripeObject
	if err = s.stripe.Request(
		ctx,
		"post",
		"/v1/checkout/sessions",
		params,
		organization.CheckoutIntent,
		&session,
	); err != nil {
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
	var customer stripeObject
	err = s.stripe.Request(ctx, "post", "/v1/customers", map[string]string{
		"name": organization.CustomerName, "metadata[anchor_organization_id]": organizationID,
		"metadata[anchor_product_id]": s.config.ProductID, metadataInstallation: stored.InstallationID,
	}, organization.CustomerIntent, &customer)
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
	if len(current.Items.Data) != 1 {
		return Organization{}, errors.New("the prototype supports one recurring price per organization")
	}
	price, err := functional.Slice(slices.Collect(maps.Values(stored.Prices))).
		FindFirst(func(price Price) bool { return price.StripePriceID == current.Items.Data[0].Price.ID }).
		ToResult(errors.New("subscription price is not mapped to an Anchor license template")).
		Value()
	if err != nil {
		return Organization{}, err
	}
	organization.SubscriptionID, organization.Status, organization.PriceID = current.ID, current.Status, price.ID
	organization.CancelAtPeriodEnd = current.CancelAtPeriodEnd || current.CancelAt > 0
	organization.PendingUpdate = len(current.PendingUpdate) > 0 && string(current.PendingUpdate) != "null"
	if end := current.Items.Data[0].CurrentPeriodEnd; end > 0 {
		value := time.Unix(end, 0).UTC()
		organization.CurrentPeriodEnd = &value
	}
	organization.SyncState, organization.SyncError = statusPending, ""
	if err = s.saveOrganization(organization); err != nil {
		return Organization{}, err
	}
	targetTemplate := ""
	switch current.Status {
	case "active", "trialing":
		targetTemplate = price.TemplateID
	case "canceled", "unpaid", "paused", "incomplete_expired":
		targetTemplate = stored.Settings.FallbackTemplateID
	case "past_due", "incomplete":
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
	var current subscription
	if err = s.stripe.Request(
		ctx,
		"get",
		"/v1/subscriptions/"+organization.SubscriptionID,
		nil,
		"",
		&current,
	); err != nil {
		return Organization{}, err
	}
	if len(current.Items.Data) != 1 {
		return Organization{}, ErrConflict
	}
	if err = s.stripe.Request(ctx, "post", "/v1/subscriptions/"+organization.SubscriptionID, map[string]string{
		"items[0][id]": current.Items.Data[0].ID, "items[0][price]": price.StripePriceID,
		"payment_behavior": "pending_if_incomplete", "proration_behavior": "always_invoice",
	}, ksuid.New().String(), nil); err != nil {
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
	if err = s.stripe.Request(ctx, "post", "/v1/subscriptions/"+organization.SubscriptionID,
		map[string]string{"cancel_at_period_end": strconv.FormatBool(cancel)}, "", nil); err != nil {
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
	params := map[string]string{
		"business_profile[headline]":                    "Manage your Anchor organization subscription",
		"default_return_url":                            s.config.ReturnURL,
		"features[invoice_history][enabled]":            stripeTrue,
		"features[payment_method_update][enabled]":      stripeTrue,
		"features[customer_update][enabled]":            stripeTrue,
		"features[customer_update][allowed_updates][0]": "email",
		"features[subscription_cancel][enabled]":        stripeTrue,
		"features[subscription_cancel][mode]":           "at_period_end",
		"features[subscription_update][enabled]":        strconv.FormatBool(len(prices) > 0),
	}
	if len(prices) > 0 {
		params["features[subscription_update][default_allowed_updates][0]"] = "price"
		params["features[subscription_update][proration_behavior]"] = "always_invoice"
		productIDs := prices.Map(func(price Price) string { return price.StripeProductID }).
			UniqueBy(func(id string) string { return id })
		for i, productID := range productIDs {
			prefix := fmt.Sprintf("features[subscription_update][products][%d]", i)
			params[prefix+"[product]"] = productID
			group := prices.Filter(func(price Price) bool { return price.StripeProductID == productID })
			for j, price := range group {
				params[fmt.Sprintf("%s[prices][%d]", prefix, j)] = price.StripePriceID
			}
		}
	}
	var configuration, session stripeObject
	if err = s.stripe.Request(
		ctx,
		"post",
		"/v1/billing_portal/configurations",
		params,
		"",
		&configuration,
	); err != nil {
		return URLResponse{}, err
	}
	if err = s.stripe.Request(ctx, "post", "/v1/billing_portal/sessions", map[string]string{
		stripeCustomer: organization.CustomerID, "configuration": configuration.ID,
		"return_url": s.organizationReturnURL(organizationID),
	}, "", &session); err != nil {
		return URLResponse{}, err
	}
	return URLResponse{URL: session.URL}, nil
}

func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	if err := validate.ValidateStruct(struct {
		Signature string `validate:"required"`
	}{signature}); err != nil {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	event, err := webhook.ConstructEventWithOptions(
		body,
		signature,
		s.config.WebhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true},
	)
	if err != nil {
		return fmt.Errorf("%w: invalid Stripe webhook signature", ErrInput)
	}
	if event.Livemode || event.Account != "" && event.Account != s.config.ExpectedAccountID {
		return fmt.Errorf("%w: webhook does not belong to the configured sandbox account", ErrInput)
	}
	var object struct {
		Customer string `json:"customer"`
	}
	if err = json.Unmarshal(event.Data.Raw, &object); err != nil {
		return ErrInput
	}
	return s.store.Update(func(state *StoredState) error {
		if _, duplicate := state.Events[event.ID]; duplicate {
			return nil
		}
		organizationID := functional.Slice(slices.Collect(maps.Values(state.Organizations))).
			FindFirst(func(organization organizationRecord) bool {
				return object.Customer != "" && organization.CustomerID == object.Customer
			}).
			Map(func(organization organizationRecord) string { return organization.ID }).
			OrElse("")
		status := statusPending
		if organizationID == "" {
			status = "ignored"
		}
		state.Events[event.ID] = eventRecord{
			ID: event.ID, Type: string(event.Type), OrganizationID: organizationID,
			ReceivedAt: time.Now().UTC(), Status: status,
			NextAttempt: time.Now().UTC(),
		}
		return nil
	})
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
		_, syncErr := s.syncLocked(ctx, job.OrganizationID)
		if syncErr != nil {
			syncErr = errors.Join(syncErr, s.markSyncError(job.OrganizationID, syncErr))
		}
		if err = s.store.Update(func(state *StoredState) error {
			event := state.Events[job.ID]
			event.Attempts++
			if syncErr == nil {
				event.Status, event.LastError = "processed", ""
			} else {
				event.Status, event.LastError = statusError, syncErr.Error()
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
		return nil
	})
}

func (s *Service) recoverCheckout(
	ctx context.Context,
	organization *organizationRecord,
	request CheckoutRequest,
) (functional.Option[URLResponse], error) {
	var err error
	var existing checkoutSession
	if err = s.stripe.Request(
		ctx,
		"get",
		"/v1/checkout/sessions/"+organization.CheckoutID,
		nil,
		"",
		&existing,
	); err != nil {
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
		if err = s.stripe.Request(
			ctx,
			"post",
			"/v1/checkout/sessions/"+organization.CheckoutID+"/expire",
			nil,
			"",
			nil,
		); err != nil {
			return functional.None[URLResponse](), err
		}
	}
	organization.CheckoutID, organization.CheckoutIntent = "", ""
	return functional.None[URLResponse](), nil
}

func (s *Service) retireCompletedCheckout(
	ctx context.Context,
	organization *organizationRecord,
	session checkoutSession,
) error {
	if _, err := s.syncLocked(ctx, organization.ID); err != nil {
		return err
	}
	stored, err := s.store.Snapshot()
	if err != nil {
		return err
	}
	current := stored.Organizations[organization.ID]
	if session.Live || session.ID != current.CheckoutID || string(session.Customer) != current.CustomerID ||
		session.ClientReferenceID != current.ID || session.Subscription == "" ||
		string(session.Subscription) != current.SubscriptionID || !terminal(current.Status) {
		return fmt.Errorf(
			"%w: completed checkout needs a reconciled terminal subscription before replacement",
			ErrConflict,
		)
	}
	var previous subscription
	if err = s.stripe.Request(ctx, "get", "/v1/subscriptions/"+current.SubscriptionID, nil, "", &previous); err != nil {
		return err
	}
	if previous.ID != current.SubscriptionID || !terminal(previous.Status) || previous.Live ||
		previous.Customer != current.CustomerID || previous.Metadata["anchor_prototype_id"] != stored.InstallationID ||
		previous.Metadata["anchor_organization_id"] != current.ID || previous.Metadata["anchor_product_id"] != s.config.ProductID {
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
) (functional.Seq[subscription], error) {
	var err error
	var subscriptions []subscription
	params := map[string]string{stripeCustomer: organization.CustomerID, "status": "all", "limit": "100"}
	for {
		var page struct {
			Data    []subscription `json:"data"`
			HasMore bool           `json:"has_more"`
		}
		if err = s.stripe.Request(ctx, "get", "/v1/subscriptions", params, "", &page); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, page.Data...)
		if !page.HasMore {
			break
		}
		if len(page.Data) == 0 || len(subscriptions) > 1000 {
			return nil, errors.New("unexpected Stripe subscription pagination")
		}
		params["starting_after"] = page.Data[len(page.Data)-1].ID
	}
	return functional.Slice(subscriptions).Filter(func(subscription subscription) bool {
		return subscription.Customer == organization.CustomerID && !subscription.Live &&
			subscription.Metadata["anchor_prototype_id"] == stored.InstallationID &&
			subscription.Metadata["anchor_organization_id"] == organization.ID && subscription.Metadata["anchor_product_id"] == s.config.ProductID
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
	var product stripeObject
	err := s.stripe.Request(ctx, "post", "/v1/products", map[string]string{
		"name": stored.ProductNames[request.TemplateID], "metadata[anchor_product_id]": s.config.ProductID,
		"metadata[anchor_template_id]": request.TemplateID, metadataInstallation: stored.InstallationID,
	}, stored.InstallationID+"-product-"+request.TemplateID, &product)
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

func canonicalSubscription(owned functional.Seq[subscription], savedID string) (subscription, error) {
	active := owned.Filter(func(value subscription) bool { return !terminal(value.Status) })
	if len(active) > 1 {
		return subscription{}, fmt.Errorf(
			"%w: multiple current subscriptions need manual resolution in Stripe",
			ErrConflict,
		)
	}
	if len(active) == 1 {
		return active[0], nil
	}
	if savedID != "" {
		return owned.FindFirst(func(value subscription) bool { return value.ID == savedID }).OrElse(subscription{}), nil
	}
	// A terminal subscription may be the first canonical state observed after downtime.
	return owned.FoldLeft(subscription{}, func(latest, candidate subscription) subscription {
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
