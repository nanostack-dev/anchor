# Deployed API flow suites

These API suites are Echopoint flow definitions stored per organization in the service, rather than Go files in this repository. A Go build/test pass does not validate their assertions. Changes to an HTTP status, error code or response contract must update the relevant definitions in the same change.

## Inspect scope, update and execute

1. Check the installed `echopoint` version and help for the commands you will use. Verify the intended `dev` and `prod` profiles with `echopoint --profile <profile> auth status`; profiles name API/authentication targets, while `--environment dev` selects a named variable overlay inside the organization. Each organization's flow IDs differ.
2. Inspect the suite selected by this product's tag. Use supported CLI/API management to discover the actual definition, organization, environment, assertion and any active copies. Preserve unrelated fields. Never edit the database to configure a flow.
3. Update each relevant status/error assertion and any display name containing its expected result. Keep setup prerequisites and negative-case inputs valid so the assertion isolates the intended cause.
4. Run the tag on both required profiles using the command in `testing.md`. Inspect completed executions and individual node results; accepted launch, skipped dependency and unevaluated assertion are distinct from pass. Separate product assertions from setup/provider failures.
5. Read back external side effects through their public interface. For emitted events, arrange an isolated capture endpoint, correlate events with this run's resource/action, and check required/forbidden/duplicate matches under the delivery contract. Wait with a bounded deadline before cleanup.
6. Remove only resources this run created. Verify cleanup through supported read/list APIs even after an assertion failure; a success-only graph edge is insufficient for failure/cancellation cleanup. Use verified always/finally behavior or an owned external teardown ledger.
7. Repeat new or reshaped concurrent graphs to sample collisions and ordering, and confirm that the infra post-deploy job selects the updated definitions. Record source, engine version, scope, selection, actual outcomes, repeat runs, cleanup and remaining boundaries.

Independent branches share their real prerequisites; order nodes only when they consume an earlier output or observe its write. Keep independent mutations on separate fixtures. Baseline counts precede every writer in their scope; post-write counts wait for all relevant writers. Poll asynchronous projections by this run's identifiers rather than fixed sleeps.

Survey existing variable names and their precedence before adding inputs. Generate disposable identifiers once and reuse the value wherever the same resource is intended. Keep credentials in supported secret inputs and secure provisioning, outside definitions, command arguments, commits, PR text and shared execution evidence. Check request/result/event redaction before publishing.

The optional Nanostack `flow-suite-testing` skill provides additional graph guidance. This local procedure and the installed CLI schema/help remain sufficient for a standalone checkout.
