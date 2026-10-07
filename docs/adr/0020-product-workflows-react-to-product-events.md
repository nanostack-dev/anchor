# ADR-0020: Product workflows react to product events

**Status:** Proposed (prototype)

## Context

Every Product built on Anchor writes the same glue: when an Organization is created, give it a default Workspace and a license; when a Product User signs up with a company email, add them to that company's Organization; when a member joins, send a welcome email. Today each Product receives the event on its endpoint ([ADR-0017](0017-product-events-use-standard-webhooks.md)) and calls the Anchor API back. The glue differs only in its details, so a Product should be able to lay it out itself, inside Anchor, without code.

## Decision

A **workflow** belongs to one Product. It names a trigger, conditions on the event, and up to 20 ordered **steps**. The trigger is an event type from the product event catalog (integration events such as `clerk.user.created` included) or a **custom event** another workflow emits. Each step runs one **action** from a fixed catalog: read or write an Organization, create a Workspace, add, re-role or remove a member, invite, read a Product User, instantiate, migrate or adjust a license, send an email, **call the Product's backend**, or **start other workflows**. Parameters are strings with `{{ path }}` references to the event (`event.data.*`) and to the output of an earlier step (`steps.<id>.*`). A step can carry its own conditions and can be allowed to fail.

- **Custom events chain workflows.** `workflow.emit` emits `custom.<name>` with a data object; every enabled workflow triggered by it runs next. Custom events never leave Anchor and are never delivered to the event endpoint. The catalog lists the custom events a Product's workflows emit, with the keys they declare.
- **Custom actions call the Product.** `http.request` sends a request signed like an event delivery (Standard Webhooks, the event endpoint's secret) and gives later steps the JSON answer. In production it refuses plain HTTP and any address that is not public, checked on the dialed address.

- **Triggered by the event bus, in the same transaction.** `events.Emit` also enqueues a job on `product-workflows` when the Product has an enabled workflow for that event type, in the transaction of the write. A Product with no workflow pays one indexed read per event and no job.
- **Runs as the Product, through the services.** Actions call the same services as the API, so every write is validated, tenant-scoped by the workflow's own Product, and emits its own events. No template value can move an action to another Product.
- **At most once per event.** A run is stored as `running` before its first step, under a unique `(workflow, event)` key. A redelivered event starts nothing; a crashed run stays `running` and is never repeated, because a step that wrote cannot be safely replayed.
- **No loop is saved.** Every action declares the event types it emits. On create, update or enable, Anchor walks the graph of enabled workflows (trigger → emitted events) and refuses, with `WORKFLOW_LOOP` naming every hop, a workflow that could start itself again. Step conditions are ignored: a loop a condition would break is still refused, because a condition is data and data changes.
- **No loop runs.** Every event carries its **causation**: the chain of workflows that led to it. A workflow already in the chain does not run again; it records a skipped run saying "Loop prevented". This covers what the save check cannot see, such as two saves racing. A chain also stops after 5 runs (`Chain stopped`).
- **Loops through the Product's backend.** `http.request` sends the causation in an `Anchor-Workflow-Causation` header. A backend that sends it back on its own calls to Anchor keeps those writes in the chain, so the guard still holds across the round trip. A manual run carrying a chain that holds the workflow is refused with a conflict. A backend that drops the header escapes the guard: that is the documented limit.
- **Only events after the workflow exists.** An event emitted before the workflow was created starts no run, even if its job is still queued.
- **Checked on save.** Unknown triggers, actions, parameters and operators are refused, every required parameter must be set, and every reference must name a key the trigger carries or an output of an earlier step.
- **Dry run.** An unsaved workflow can run against sample event data: reads run for real, writes are only resolved and report placeholder outputs.

Anchor still validates but never gates ([ADR-0001](0001-anchor-validates-but-never-gates.md)): a workflow writes what a Product told it to write, through the same rules as the API.

## Considered Options

- **Keep it in each Product.** No new surface in Anchor, but every Product rebuilds the same reactions and their retries.
- **A general-purpose engine (scripts, loops, arbitrary third parties).** Rejected: scripting needs a sandbox and a third-party call needs credentials Anchor would have to hold. The one outbound action calls the Product's own backend, signed with a secret the Product already has. Echopoint remains the flow engine for arbitrary APIs.
- **Only a depth limit against loops.** Rejected as the main guard: a loop would still run several times before stopping, writing each time. Refusing the loop on save, and refusing a second run of the same workflow in one chain, stop it before the first repeated write.
- **Retry a failed run.** Rejected: a run that failed after writing would repeat its writes. The run reports the failure; a person reruns it by hand.

## Consequences

`workflow:*` scopes on a Product API key are effectively administrative: a key that can create a workflow can make Anchor perform any action in the catalog for that Product. A new catalogued event type needs its data keys in the workflow trigger table. A new action is one entry in `internal/workflow/engine/actions.go`, with the event types it emits, and is picked up by the catalog endpoint, the loop check and the UI without further change. An action that forgets to declare an event it emits weakens the save check; the runtime guard still stops the loop.
