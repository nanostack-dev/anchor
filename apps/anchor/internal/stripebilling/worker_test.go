//nolint:testpackage // Exercises private scheduler and queue handlers without requiring a database.
package stripebilling

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nanostack-dev/pgkit/queue"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type schedulerQueueFake struct {
	jobs       map[queue.JobStatus][]queue.Job
	enqueued   []queue.EnqueueParams
	transactor *sql.Tx
	enqueueErr error
}

func (q *schedulerQueueFake) ListJobs(_ context.Context, p queue.ListJobsParams) ([]queue.Job, error) {
	return q.jobs[p.Status], nil
}

func (q *schedulerQueueFake) EnqueueTx(
	_ context.Context,
	tx *sql.Tx,
	p queue.EnqueueParams,
) (int64, error) {
	q.transactor = tx
	q.enqueued = append(q.enqueued, p)
	return 1, q.enqueueErr
}

type schedulerLockFake struct {
	acquired bool
	tx       *sql.Tx
}

func (l schedulerLockFake) TryWithLock(
	ctx context.Context,
	_ string,
	callback func(context.Context, *sql.Tx) error,
) (bool, error) {
	if !l.acquired {
		return false, nil
	}
	return true, callback(ctx, l.tx)
}

type billingProcessorFake struct {
	instanceID string
	runErr     error
	resyncErr  error
}

func (p *billingProcessorFake) RunInternal(_ context.Context, instanceID string) error {
	p.instanceID = instanceID
	return p.runErr
}

func (p *billingProcessorFake) ReconcileInternal(_ context.Context) error {
	return p.resyncErr
}

func TestSchedulerStartupDoesNotDuplicateDurableJobs(t *testing.T) {
	t.Parallel()
	for _, status := range []queue.JobStatus{queue.StatusPending, queue.StatusProcessing} {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			jobs := map[queue.JobStatus][]queue.Job{
				queue.StatusPending: nil, queue.StatusProcessing: nil,
				queue.StatusDone: nil, queue.StatusFailed: nil,
			}
			jobs[status] = []queue.Job{{ID: 1}}
			client := &schedulerQueueFake{jobs: jobs}
			scheduler := reconcileScheduler{queue: client, lock: schedulerLockFake{acquired: true}}
			require.NoError(t, scheduler.schedule(t.Context(), true))
			require.Empty(t, client.enqueued)
		})
	}
}

func TestSchedulerPersistsNextRunInLockTransaction(t *testing.T) {
	t.Parallel()
	client := &schedulerQueueFake{}
	tx := &sql.Tx{}
	scheduler := reconcileScheduler{queue: client, lock: schedulerLockFake{acquired: true, tx: tx}}
	before := time.Now().Add(reconcileInterval)
	require.NoError(t, scheduler.schedule(t.Context(), false))
	after := time.Now().Add(reconcileInterval)
	require.Len(t, client.enqueued, 1)
	require.Same(t, tx, client.transactor)
	job := client.enqueued[0]
	require.Equal(t, ReconcileQueueName, job.QueueName)
	require.True(t, json.Valid(job.Payload))
	require.NotNil(t, job.AvailableAt)
	require.False(t, job.AvailableAt.Before(before))
	require.False(t, job.AvailableAt.After(after))
}

func TestSchedulerRescheduleSurvivesDuplicateProcessingJobs(t *testing.T) {
	t.Parallel()
	client := &schedulerQueueFake{
		jobs: map[queue.JobStatus][]queue.Job{
			queue.StatusPending: nil, queue.StatusProcessing: {{ID: 1}, {ID: 2}},
			queue.StatusDone: nil, queue.StatusFailed: nil,
		},
	}
	scheduler := reconcileScheduler{queue: client, lock: schedulerLockFake{acquired: true}}
	require.NoError(t, scheduler.schedule(t.Context(), false))
	require.Len(t, client.enqueued, 1)
}

func TestSchedulerRetriesUnavailableLockAndPersistence(t *testing.T) {
	t.Parallel()
	client := &schedulerQueueFake{enqueueErr: errors.New("database unavailable")}
	scheduler := reconcileScheduler{queue: client, lock: schedulerLockFake{}}
	require.NoError(t, scheduler.schedule(t.Context(), true))
	require.ErrorContains(t, scheduler.schedule(t.Context(), false), "scheduler is busy")
	require.Empty(t, client.enqueued)
	scheduler.lock = schedulerLockFake{acquired: true}
	require.ErrorIs(t, scheduler.schedule(t.Context(), false), client.enqueueErr)
}

func TestEventHandlerRejectsMalformedAndMissingInstance(t *testing.T) {
	t.Parallel()
	processor := &billingProcessorFake{}
	handler := billingJobHandler{processor: processor}
	for _, payload := range []string{`{`, `{}`, `null`} {
		err := handler.event(t.Context(), queue.Job{Payload: []byte(payload)})
		require.Error(t, err)
		require.True(t, queue.IsNonRetryable(err))
	}
	require.Empty(t, processor.instanceID)
}

func TestEventHandlerPassesInstanceAndProcessingFailure(t *testing.T) {
	t.Parallel()
	processor := &billingProcessorFake{runErr: errors.New("reconciliation failed")}
	handler := billingJobHandler{processor: processor}
	err := handler.event(t.Context(), queue.Job{
		Payload: []byte(`{"integration_instance_id":"iin_example"}`),
	})
	require.ErrorIs(t, err, processor.runErr)
	require.Equal(t, "iin_example", processor.instanceID)
}

func TestFailedReconciliationKeepsRecurringScheduleAlive(t *testing.T) {
	t.Parallel()
	processor := &billingProcessorFake{resyncErr: errors.New("Stripe temporarily unavailable")}
	client := &schedulerQueueFake{}
	handler := billingJobHandler{
		processor: processor,
		scheduler: reconcileScheduler{
			queue: client, lock: schedulerLockFake{acquired: true}, logger: zerolog.Nop(),
		},
	}
	require.NoError(t, handler.reconcile(t.Context(), queue.Job{}))
	require.Len(t, client.enqueued, 1)
}
