# Browser and asynchronous form review

Review behavior across the UI, query lifecycle and persistence assertions. The
inventory and completion checks verify discovery and execution; reviewers verify
that each scenario asserts the user-visible behavior it claims to cover.

When a server query initializes editable local state:

- Trace initial loading, successful hydration and failed loading separately.
  Confirm when writes become available and how the user retries a failed load.
- Follow a late initial response and a background refetch. Check that neither
  silently replaces edits or lets an early save replace existing server data.
- Require a regression that controls the response boundary when timing affects
  correctness. Hold the real response, assert pending behavior, release it, then
  verify existing and new values persist through save and reload.

The [email examples regression](troubleshooting/integrations.md#initial-examples-hydration)
records the observed race and its verification. Use its controlled-response
pattern for the relevant risk; fixed sleeps do not prove ordering.

Finish review with evidence for each applicable state and persistence assertion.
Report an uncovered state explicitly rather than counting a route annotation as
proof that its controls work.
