# Prototype — events outbox (throwaway)

Question: can a feature put the domain write and the event in one `transactor.InTx`, so a rollback drops both?

This is not production code. Table `poc_wipe_me_organizations` is scratch.

## Run

Docker must be running. About 1 minute on a warm machine.

```
cd apps/anchor
go run ./cmd/prototype/events-outbox
```

## What it compares

1. pgkit queue `EnqueueTx` on `transactor.CurrentTx` (the `Emitter.Emit` helper)
2. pgkit `workflow.Start` called inside the same `InTx`

## Watch

- Queue commit: organization row and queue job both exist
- Queue rollback: neither exists
- Workflow Start inside InTx, then rollback: organization row is gone, workflow run remains
- `Emit` outside `InTx`: refused
