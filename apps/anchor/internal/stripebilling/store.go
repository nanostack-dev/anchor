package stripebilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"anchor/internal/db/gen/anchor/public/model"
	"anchor/internal/db/gen/anchor/public/table"
	"anchor/internal/domain/integration"
	billing "anchor/internal/stripeprototype"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
)

type databaseStore struct {
	ctx       context.Context
	db        *sql.DB
	queue     *queue.Client
	instance  integration.Instance
	accountID string
}

func (s *databaseStore) where() postgres.BoolExpression {
	t := table.StripeBillingStates
	return t.IntegrationInstanceID.EQ(postgres.String(s.instance.ID)).
		AND(t.PlatformTenantID.EQ(postgres.String(s.instance.PlatformTenantID))).
		AND(t.ProductID.EQ(postgres.String(s.instance.ProductID)))
}
func (s *databaseStore) Snapshot() (billing.StoredState, error) {
	t := table.StripeBillingStates
	row, err := transactor.QueryOptional[model.StripeBillingStates](
		s.ctx,
		s.db,
		t.SELECT(t.AllColumns).FROM(t).WHERE(s.where()),
	)
	if err != nil {
		return billing.StoredState{}, err
	}
	if row.IsAbsent() {
		state := billing.NewStoredState(s.accountID, s.instance.ProductID)
		data, marshalErr := json.Marshal(state)
		if marshalErr != nil {
			return state, marshalErr
		}
		entity := model.StripeBillingStates{
			IntegrationInstanceID: s.instance.ID,
			PlatformTenantID:      s.instance.PlatformTenantID,
			ProductID:             s.instance.ProductID,
			StateJSON:             string(data),
		}
		_, insertErr := t.INSERT(t.AllColumns.Except(t.CreatedAt, t.UpdatedAt)).
			MODEL(entity).
			ON_CONFLICT(t.IntegrationInstanceID).
			DO_NOTHING().
			ExecContext(s.ctx, s.db)
		if insertErr != nil {
			return state, insertErr
		}
		row, err = transactor.QueryOptional[model.StripeBillingStates](
			s.ctx,
			s.db,
			t.SELECT(t.AllColumns).FROM(t).WHERE(s.where()),
		)
		if err != nil {
			return state, err
		}
		if row.IsAbsent() {
			return state, errors.New("billing state is outside the integration scope")
		}
	}
	var state billing.StoredState
	if err = json.Unmarshal([]byte(row.Value().StateJSON), &state); err != nil {
		return state, err
	}
	if state.AccountID != s.accountID || state.ProductID != s.instance.ProductID {
		return state, errors.New(
			"billing state belongs to another Stripe account; reconnect the original sandbox account",
		)
	}
	return state, nil
}
func (s *databaseStore) Update(change func(*billing.StoredState) error) error {
	before, err := s.Snapshot()
	if err != nil {
		return err
	}
	data, err := json.Marshal(before)
	if err != nil {
		return err
	}
	var after billing.StoredState
	if err = json.Unmarshal(data, &after); err != nil {
		return err
	}
	if err = change(&after); err != nil {
		return err
	}
	if after.AccountID != before.AccountID || after.ProductID != before.ProductID ||
		after.InstallationID != before.InstallationID {
		return errors.New("billing state identity cannot change")
	}
	data, err = json.Marshal(after)
	if err != nil {
		return err
	}
	return transactor.New(s.db).InTx(s.ctx, func(ctx context.Context) error {
		t := table.StripeBillingStates
		updated, updateErr := t.UPDATE(t.StateJSON).
			MODEL(model.StripeBillingStates{StateJSON: string(data)}).
			WHERE(s.where()).
			ExecContext(ctx, transactor.CurrentTx(ctx))
		if updateErr != nil {
			return updateErr
		}
		affected, countErr := updated.RowsAffected()
		if countErr != nil {
			return countErr
		}
		if affected != 1 {
			return errors.New("billing integration was removed during the update")
		}

		// Side effects enqueue newly received or rescheduled events in the same commit.
		for id, event := range after.Events {
			old, existed := before.Events[id]
			if event.Status != "pending" && event.Status != "error" {
				continue
			}
			if existed && old.Status == event.Status && old.Attempts == event.Attempts &&
				old.NextAttempt.Equal(event.NextAttempt) {
				continue
			}
			payload, marshalErr := json.Marshal(EventQueuePayload{IntegrationInstanceID: s.instance.ID})
			if marshalErr != nil {
				return marshalErr
			}
			available := event.NextAttempt
			if available.IsZero() {
				available = time.Now()
			}
			if _, enqueueErr := s.queue.EnqueueTx(
				ctx,
				transactor.CurrentTx(ctx),
				queue.EnqueueParams{
					QueueName:   EventsQueueName,
					Payload:     payload,
					AvailableAt: &available,
					MaxAttempts: workerMaxAttempts,
				},
			); enqueueErr != nil {
				return enqueueErr
			}
		}
		return nil
	})
}
