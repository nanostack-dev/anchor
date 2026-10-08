package stripeprototype

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
	"golang.org/x/sys/unix"
)

type organizationRecord struct {
	Organization
	CustomerIntent string `json:"customer_intent"`
	CustomerName   string `json:"customer_name"`
	CheckoutIntent string `json:"checkout_intent"`
	CheckoutID     string `json:"checkout_id"`
	CheckoutPrice  string `json:"checkout_price"`
	CheckoutTrial  int    `json:"checkout_trial"`
}

type eventRecord struct {
	BillingEvent
	Attempts    int       `json:"attempts"`
	NextAttempt time.Time `json:"next_attempt"`
}

type StoredState struct {
	InstallationID string                        `json:"installation_id"`
	AccountID      string                        `json:"account_id"`
	ProductID      string                        `json:"product_id"`
	Settings       Settings                      `json:"settings"`
	Prices         map[string]Price              `json:"prices"`
	Organizations  map[string]organizationRecord `json:"organizations"`
	Events         map[string]eventRecord        `json:"events"`
	PriceIntents   map[string]CreatePriceRequest `json:"price_intents"`
	StripeProducts map[string]string             `json:"stripe_products"`
	ProductNames   map[string]string             `json:"product_names"`
}

type Store struct {
	mu       sync.Mutex
	path     string
	lock     *os.File
	contents StoredState
}

func OpenStore(path, accountID, productID, fallbackID string) (*Store, error) {
	if path == "" || accountID == "" || productID == "" {
		return nil, errors.New("state path, Stripe account and Anchor product are required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, errors.New("another billing prototype owns this state file")
	}
	store := &Store{path: path, lock: lock, contents: StoredState{
		InstallationID: ksuid.New().String(),
		AccountID:      accountID,
		ProductID:      productID,
		Settings:       Settings{FallbackTemplateID: fallbackID},
		Prices:         map[string]Price{},
		Organizations:  map[string]organizationRecord{},
		Events:         map[string]eventRecord{},
		PriceIntents:   map[string]CreatePriceRequest{},
		StripeProducts: map[string]string{},
		ProductNames:   map[string]string{},
	}}
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &store.contents)
	} else if errors.Is(err, os.ErrNotExist) {
		err = store.Update(func(*StoredState) error { return nil })
	}
	if err == nil && (store.contents.AccountID != accountID || store.contents.ProductID != productID) {
		err = errors.New("state belongs to another Stripe account or Anchor product; use a new prototype state file")
	}
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if err := unix.Flock(int(s.lock.Fd()), unix.LOCK_UN); err != nil {
		return err
	}
	return s.lock.Close()
}

func (s *Store) Snapshot() (StoredState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(s.contents)
	if err != nil {
		return StoredState{}, err
	}
	var snapshot StoredState
	err = json.Unmarshal(data, &snapshot)
	return snapshot, err
}

func (s *Store) Update(change func(*StoredState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.Marshal(s.contents)
	if err != nil {
		return err
	}
	var next StoredState
	if err = json.Unmarshal(data, &next); err != nil {
		return err
	}
	if err = change(&next); err != nil {
		return err
	}
	data, err = json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.path+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(s.path+".tmp", s.path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	err = directory.Sync()
	closeErr = directory.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	s.contents = next
	return nil
}
