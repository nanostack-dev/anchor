//nolint:testpackage // The durable recovery fixtures need direct access to private persisted event and organization records.
package stripeprototype

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/segmentio/ksuid"
	"golang.org/x/sys/unix"
)

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
