package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/pgkit/queue"
	"github.com/nanostack-dev/pgkit/workflow"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	pocRunTimeout          = 3 * time.Minute
	postgresReadyLogCount  = 2
	postgresPingTimeout    = 30 * time.Second
	postgresPingInterval   = 200 * time.Millisecond
	listJobLimit           = 50
	workflowFirstVersion   = 1
	workflowName           = "poc-emit-event"
	postgresReadyLogNeedle = "database system is ready to accept connections"
)

var errForceRollback = errors.New("poc: force rollback")

type scenarioResult struct {
	Name          string
	Orgs          int
	QueueJobs     int
	WorkflowRuns  int
	WriteReturned string
	Pass          bool
}

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithTimeout(context.Background(), pocRunTimeout)
	defer cancel()

	_, _ = fmt.Fprintln(
		os.Stdout,
		"Question: can a feature put the domain write and the event in one transactor.InTx, so a rollback drops both?",
	)
	_, _ = fmt.Fprintln(os.Stdout, "Approach A: pgkit queue EnqueueTx on CurrentTx.")
	_, _ = fmt.Fprintln(os.Stdout, "Approach B: pgkit workflow.Start inside the same InTx.")
	_, _ = fmt.Fprintln(os.Stdout)

	container, db, err := startScratchPostgres(ctx)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()
	defer func() { _ = db.Close() }()

	if setupErr := setupScratch(ctx, db); setupErr != nil {
		return fmt.Errorf("schema: %w", setupErr)
	}

	queueClient, err := queue.New(db)
	if err != nil {
		return fmt.Errorf("queue: %w", err)
	}
	if schemaErr := queueClient.EnsureSchema(ctx); schemaErr != nil {
		return fmt.Errorf("queue schema: %w", schemaErr)
	}

	txor := transactor.New(db)
	emitter := &Emitter{queue: queueClient}

	wf, err := newWorkflow(ctx, db, queueClient)
	if err != nil {
		return fmt.Errorf("workflow: %w", err)
	}

	results := []scenarioResult{
		runQueueCommit(ctx, db, txor, emitter, queueClient),
		runQueueRollback(ctx, db, txor, emitter, queueClient),
		runWorkflowInsideInTxRollback(ctx, db, txor, wf),
		runEmitOutsideTx(ctx, emitter),
	}

	_, _ = fmt.Fprintf(
		os.Stdout,
		"%-28s %-8s %-10s %-14s %-10s %s\n",
		"scenario",
		"orgs",
		"queue_jobs",
		"workflow_runs",
		"write_err",
		"verdict",
	)
	allPass := true
	for _, result := range results {
		verdict := "FAIL"
		if result.Pass {
			verdict = "PASS"
		} else {
			allPass = false
		}
		_, _ = fmt.Fprintf(
			os.Stdout,
			"%-28s %-8d %-10d %-14d %-10s %s\n",
			result.Name,
			result.Orgs,
			result.QueueJobs,
			result.WorkflowRuns,
			result.WriteReturned,
			verdict,
		)
	}
	_, _ = fmt.Fprintln(os.Stdout)
	if allPass {
		_, _ = fmt.Fprintln(
			os.Stdout,
			"Verdict: use pgkit queue EnqueueTx inside transactor.InTx. workflow.Start opens its own transaction and cannot join the write.",
		)
		return nil
	}
	return errors.New("verdict: a scenario failed")
}

func runQueueCommit(
	ctx context.Context,
	db *sql.DB,
	txor transactor.Transactor,
	emitter *Emitter,
	q *queue.Client,
) scenarioResult {
	id := "org_queue_commit"
	err := createOrgWithQueueEvent(ctx, txor, emitter, id)
	orgs := countOrgs(ctx, db, id)
	jobs := countQueueJobs(ctx, q, id)
	return scenarioResult{
		Name:          "queue commit",
		Orgs:          orgs,
		QueueJobs:     jobs,
		WriteReturned: errString(err),
		Pass:          err == nil && orgs == 1 && jobs == 1,
	}
}

func runQueueRollback(
	ctx context.Context,
	db *sql.DB,
	txor transactor.Transactor,
	emitter *Emitter,
	q *queue.Client,
) scenarioResult {
	id := "org_queue_rollback"
	err := txor.InTx(ctx, func(txCtx context.Context) error {
		if inner := insertOrgAndEmit(txCtx, emitter, id); inner != nil {
			return inner
		}
		return errForceRollback
	})
	orgs := countOrgs(ctx, db, id)
	jobs := countQueueJobs(ctx, q, id)
	return scenarioResult{
		Name:          "queue rollback",
		Orgs:          orgs,
		QueueJobs:     jobs,
		WriteReturned: errString(err),
		Pass:          errors.Is(err, errForceRollback) && orgs == 0 && jobs == 0,
	}
}

func runWorkflowInsideInTxRollback(
	ctx context.Context,
	db *sql.DB,
	txor transactor.Transactor,
	wf *workflow.Module,
) scenarioResult {
	id := "org_workflow_rollback"
	err := txor.InTx(ctx, func(txCtx context.Context) error {
		if _, execErr := transactor.CurrentTx(txCtx).ExecContext(
			txCtx,
			`INSERT INTO poc_wipe_me_organizations (id, name) VALUES ($1, $2)`,
			id,
			id,
		); execErr != nil {
			return execErr
		}
		if _, startErr := wf.Start(
			txCtx,
			workflowName,
			map[string]string{"organization_id": id},
			nil,
		); startErr != nil {
			return startErr
		}
		return errForceRollback
	})
	orgs := countOrgs(ctx, db, id)
	runs := countWorkflowRuns(ctx, wf)
	if errors.Is(err, errForceRollback) && orgs == 0 && runs > 0 {
		_, _ = fmt.Fprintf(
			os.Stdout,
			"observed: workflow.Start committed %d run(s) after the organization row rolled back.\n",
			runs,
		)
	}
	return scenarioResult{
		Name:          "workflow Start in InTx",
		Orgs:          orgs,
		WorkflowRuns:  runs,
		WriteReturned: errString(err),
		Pass:          errors.Is(err, errForceRollback) && orgs == 0 && runs > 0,
	}
}

func runEmitOutsideTx(ctx context.Context, emitter *Emitter) scenarioResult {
	err := emitter.Emit(
		ctx,
		Event{Type: "organization.created", Data: json.RawMessage(`{"organization_id":"org_no_tx"}`)},
	)
	return scenarioResult{
		Name:          "Emit without InTx",
		WriteReturned: errString(err),
		Pass:          errors.Is(err, errEmitRequiresTx),
	}
}

func createOrgWithQueueEvent(ctx context.Context, txor transactor.Transactor, emitter *Emitter, id string) error {
	return txor.InTx(ctx, func(txCtx context.Context) error {
		return insertOrgAndEmit(txCtx, emitter, id)
	})
}

func insertOrgAndEmit(ctx context.Context, emitter *Emitter, id string) error {
	if _, err := transactor.CurrentTx(ctx).ExecContext(
		ctx,
		`INSERT INTO poc_wipe_me_organizations (id, name) VALUES ($1, $2)`,
		id,
		id,
	); err != nil {
		return err
	}
	data, err := json.Marshal(map[string]string{"organization_id": id})
	if err != nil {
		return err
	}
	return emitter.Emit(ctx, Event{Type: "organization.created", Data: data})
}

func newWorkflow(ctx context.Context, db *sql.DB, q *queue.Client) (*workflow.Module, error) {
	def, err := workflow.Define(workflowName, func(b *workflow.Builder) {
		b.Title("POC emit")
		b.Step("noop", func(_ context.Context, _ workflow.StepContext) (any, error) {
			return map[string]any{"ok": true}, nil
		}, workflow.StepOptions{QueueName: "poc.workflow"})
	})
	if err != nil {
		return nil, err
	}
	mod, err := workflow.New(db, q, def)
	if err != nil {
		return nil, err
	}
	if schemaErr := mod.EnsureSchema(ctx); schemaErr != nil {
		return nil, schemaErr
	}
	if _, publishErr := mod.Publish(ctx, workflowName); publishErr != nil {
		return nil, publishErr
	}
	if activateErr := mod.Activate(ctx, workflowName, workflowFirstVersion); activateErr != nil {
		return nil, activateErr
	}
	return mod, nil
}

func startScratchPostgres(ctx context.Context) (*tcpostgres.PostgresContainer, *sql.DB, error) {
	container, err := tcpostgres.Run(
		ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("poc_wipe_me"),
		tcpostgres.WithUsername("poc"),
		tcpostgres.WithPassword("poc"),
		testcontainers.WithWaitStrategy(
			wait.ForLog(postgresReadyLogNeedle).
				WithOccurrence(postgresReadyLogCount).
				WithStartupTimeout(time.Minute),
		),
	)
	if err != nil {
		return nil, nil, err
	}
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		return nil, nil, err
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		return nil, nil, err
	}
	deadline := time.Now().Add(postgresPingTimeout)
	for {
		if pingErr := db.PingContext(ctx); pingErr == nil {
			return container, db, nil
		} else if time.Now().After(deadline) {
			_ = db.Close()
			_ = testcontainers.TerminateContainer(container)
			return nil, nil, pingErr
		}
		time.Sleep(postgresPingInterval)
	}
}

func setupScratch(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE poc_wipe_me_organizations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL
)`)
	return err
}

func countOrgs(ctx context.Context, db *sql.DB, id string) int {
	var n int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM poc_wipe_me_organizations WHERE id = $1`, id).Scan(&n)
	return n
}

func countQueueJobs(ctx context.Context, q *queue.Client, orgID string) int {
	jobs, err := q.ListJobs(ctx, queue.ListJobsParams{Limit: listJobLimit, QueueName: eventQueueName})
	if err != nil {
		return -1
	}
	n := 0
	for _, job := range jobs {
		var event Event
		if json.Unmarshal(job.Payload, &event) != nil {
			continue
		}
		var data map[string]string
		if json.Unmarshal(event.Data, &data) != nil {
			continue
		}
		if data["organization_id"] == orgID {
			n++
		}
	}
	return n
}

func countWorkflowRuns(ctx context.Context, wf *workflow.Module) int {
	n, err := wf.CountRuns(ctx, workflow.ListRunsParams{WorkflowName: workflowName, Limit: listJobLimit})
	if err != nil {
		return -1
	}
	return int(n)
}

func errString(err error) string {
	if err == nil {
		return "nil"
	}
	if errors.Is(err, errForceRollback) {
		return "rollback"
	}
	if errors.Is(err, errEmitRequiresTx) {
		return "need_tx"
	}
	return "error"
}
