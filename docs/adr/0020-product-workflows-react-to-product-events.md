# ADR-0020: Product workflows react to product events

**Status:** Proposed (prototype)

## Context

Every Product built on Anchor writes the same glue: when an Organization is created, give it a default Workspace and a license; when a Product User signs up with a company email, add them to that company's Organization; when a member joins, send a welcome email. Today each Product receives the event on its endpoint ([ADR-0017](0017-product-events-use-standard-webhooks.md)) and calls the Anchor API back. The glue differs only in its details, so a Product should be able to lay it out itself, inside Anchor, without code.

## Decision

A **workflow** belongs to one Product. It names a trigger (an event type from the product event catalog), conditions on the event, and up to 20 ordered **steps**. Each step runs one **action** from a fixed catalog: read or write an Organization, create a Workspace, add, re-role or remove a member, invite, read a Product User, instantiate, migrate or adjust a license, send an email. Parameters are strings with `{{ path }}` references to the event (`event.data.*`) and to the output of an earlier step (`steps.<id>.*`). A step can carry its own conditions and can be allowed to fail.

- **Triggered by the event bus, in the same transaction.** `events.Emit` also enqueues a job on `product-workflows` when the Product has an enabled workflow for that event type, in the transaction of the write. A Product with no workflow pays one indexed read per event and no job.
- **Runs as the Product, through the services.** Actions call the same services as the API, so every write is validated, tenant-scoped by the workflow's own Product, and emits its own events. No template value can move an action to another Product.
- **At most once per event.** A run is stored as `running` before its first step, under a unique `(workflow, event)` key. A redelivered event starts nothing; a crashed run stays `running` and is never repeated, because a step that wrote cannot be safely replayed.
- **Bounded chains.** An event carries its causation depth. A write a workflow makes emits its events one level deeper; at depth 3 no further run starts. This stops a workflow that reacts to its own writes.
- **Only events after the workflow exists.** An event emitted before the workflow was created starts no run, even if its job is still queued.
- **Checked on save.** Unknown triggers, actions, parameters and operators are refused, every required parameter must be set, and every reference must name a key the trigger carries or an output of an earlier step.
- **Dry run.** An unsaved workflow can run against sample event data: reads run for real, writes are only resolved and report placeholder outputs.

Anchor still validates but never gates ([ADR-0001](0001-anchor-validates-but-never-gates.md)): a workflow writes what a Product told it to write, through the same rules as the API.

## Considered Options

- **Keep it in each Product.** No new surface in Anchor, but every Product rebuilds the same reactions and their retries.
- **A general-purpose engine (arbitrary HTTP, scripts, loops).** Rejected for the prototype: outbound HTTP needs SSRF and secret handling, scripting needs a sandbox. Echopoint already is the flow engine for arbitrary APIs; Anchor workflows only touch Anchor's own resources.
- **Retry a failed run.** Rejected: a run that failed after writing would repeat its writes. The run reports the failure; a person reruns it by hand.

## Consequences

`workflow:*` scopes on a Product API key are effectively administrative: a key that can create a workflow can make Anchor perform any action in the catalog for that Product. A new catalogued event type needs its data keys in the workflow trigger table. A new action is one entry in `internal/workflow/engine/actions.go` and is picked up by the catalog endpoint and the UI without further change.
