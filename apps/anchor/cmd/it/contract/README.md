# Golden contract suite

This suite pins Anchor's wire contract and its alert-relevant log fields
before the move to the Nanostack framework beta (slog logging, the nano HTTP
stack, nano modules, Nanostack DB). A migration step is safe for clients and
alerts when this suite passes unchanged.

It boots the real server in-process through the shared integration harness
(`cmd/it/shared`): TimescaleDB and Redis in testcontainers, then the same fx
graph as `cmd/main.go` (`app.StartAnchor`), so the router, the OpenAPI request
validator, the auth middleware, the strict error hooks and the slog logger
(`internal/applog`, legacy wire) are the production ones.

## Run

From `apps/anchor` (Docker must be running):

```sh
go test -tags contract -count=1 ./cmd/it/contract/                      # compare with the goldens (~3.5 min)
go test -tags contract -count=1 ./cmd/it/contract/ -update              # rewrite the goldens and the table below
go test -tags contract -count=1 ./cmd/it/contract/ -run TestFrameworkSeams   # seams only (~15 s)
go test -tags contract -count=1 ./cmd/it/contract/ -run 'TestGateFlows/^auth$'
go test -tags contract -run '^$' -bench . -benchmem -count=10 ./cmd/it/contract/   # baseline benchmarks
```

The repository `Makefile` has the same commands as `contract-test`,
`contract-update` and `contract-bench`. The suite is not in the Dagger checks:
`goTests` in `.dagger/modules/anchor/main.dang` runs `./...` without build
tags, so adding it is more than a one-line change.

The suite sets `time.Local` to UTC, as in the production containers: some
values come from `time.Now()` in the local zone (licence `migrated_at`, the
log `time` field), and their zone is part of the golden.

`CONTRACT_LOG_DUMP=/tmp/anchor.log` writes every server log line to a file,
for debugging a failing flow.

## What it checks

`TestGateFlows` replays the 32 production gate flows of the Anchor workspace
(`testdata/gateflows`, see its `MANIFEST.md`). An interpreter runs each flow
the way echopoint-runner does: request, module, delay, set_variable, loop and
webhook_wait nodes; `on_success` and `always` (cleanup) phases; `{{node.output}}`,
`{{$runId}}`, `{{$uuid}}`, `{{$isoTimestamp:+60s}}`, `{{$fullName}}`,
`{{$slug:3}}` and `{{webhook.url}}` templates; the RFC 9535 JSONPath subset the
flows use; the runner's string comparison operators. It runs the nodes one at
a time in a stable order, so the goldens do not depend on scheduling. A local
receiver stands in for the run's webhook endpoint, so the product-event
deliveries and their `webhook-signature` header are checked too.

- Every gate assertion must hold, except the skipped nodes listed below.
- Every request node writes a golden, successes included:
  `testdata/golden/gate/<flow>/<node>.golden` (a module or loop node nests
  its nodes under its own id). A 2xx can change on the wire too (JSON
  encoding, timestamp layout, `null` against an omitted member, headers, log
  fields), and the flows assert only a few of its members.
- Every webhook_wait node writes a golden of the deliveries it saw: their
  headers (`User-Agent`, `Content-Type`, `webhook-id`, the
  `webhook-signature` layout, ...) and payloads, sorted so that delivery
  order does not matter.

`TestFrameworkSeams` covers what the gate flows do not: routing (unknown path,
wrong method, trailing slash), `/health` (GET, HEAD, POST, DELETE), CORS,
`/docs` and `/openapi.yaml`, path and query parameter validation, the auth
refusals, request decoding (malformed JSON, empty body, wrong type, array,
text/plain, a form content type, no Content-Type, unknown, case-variant and
duplicate members,
trailing data, a 2 MiB body), HEAD on a GET operation, OPTIONS without
preflight headers, an inbound webhook body over 1 MiB, validation and domain
errors, and the 200, 201
and 204 success shapes with their log lines. Goldens:
`testdata/golden/seams/<case>.golden`.

`TestCoverageTable` regenerates the table below from the flows, the goldens
and the skip list, and fails when the README is stale.

## Golden format

Each golden records, after normalisation:

- the request line, the request header names and the request body;
- the status, the value of every response header (except `Date`,
  `Content-Length` and `Transfer-Encoding`): a fixed list first
  (`Content-Type`, `Allow`, `Set-Cookie`, `Vary`, `X-Request-Id`, ...), then
  any other header sorted by name, and the sorted names of all of them;
- the body bytes exactly as sent (`/docs` and `/openapi.yaml` as length +
  sha256);
- every log line the server wrote with the request's `request_id`, as a
  key-sorted JSON object.

The suite sends a deterministic `X-Request-Id` (`ct/<flow>/<node>`); the
server reuses it as the `request_id` log field. A barrier line written through
the same pipe as the server's stdout makes sure every line of a request is
read before it is compared.

Normalisation replaces only values that change per run, and keeps their type
and format:

| Value | Rendered as |
|---|---|
| A value a flow captured (`outputs`) | `{{node.output}}` |
| The run id, generated names and timestamps | `{{$runId}}`, `{{$fullName}}`, `{{$slug}}`, `{{$isoTimestamp:+60s}}` |
| KSUID ids not captured | `<prefix>_<id>` (the prefix stays) |
| RFC 3339 timestamps | `<timestamp>`, with `.frac` when the value has fractional seconds and the zone when it is not `Z` (`+00:00`, or `+offset`) |
| UUIDs, JWTs | `<uuid>`, `<jwt>` |
| Cookie token and HTTP dates | `refresh_token=<token>`, `<http-date>` (the cleared-cookie `Thu, 01 Jan 1970 00:00:00 GMT` stays) |
| Obfuscated key suffix | `***_<hex>` |
| One-time API key values not captured | `<prefix>_org_apikey_<secret>_<hex8>` (the checksum suffix kept as a marker when present) |
| Pagination cursors (base64url JSON) | `<cursor>` |
| Random 32-hex tokens (invitation codes) | `<hex32>` |
| Webhook signing secrets | `whsec_<secret>` |
| SMTP integration `last_error` (a background connection check to `smtp.example.com` races the read) | `<smtp check: not configured or connection failed>` |
| Masked secrets | the stars stay, the visible tail becomes `<tail>` |
| URL-encoded timestamps in a query string | `<timestamp-urlencoded>` |
| The product webhook `events` list | sorted (the server builds it from a Go map, so its order changes per call) |
| Log `time` | `<time:timestamp>` with the same layout markers (a Unix number renders `<time>:number`) |
| Log `client_ip` | `<ip>`, or `<ip:port>` when a port is appended |
| Log `bytes_out` | `<int:=body-length>` when it equals the length of the body the client received, else both numbers |
| Log `caller` | `<file>.go:<line>` or `/<path>/<file>.go:<line>` (absolute) only when it is `file:line` (otelcol splits it on `:`) |
| Webhook `webhook-timestamp`, `webhook-signature` | `<unix-seconds>`, `v1,<base64>` |
| Log `duration` | `<duration:number:ms>`: the unit is read against the duration printed in the summary message, so a switch to nanoseconds fails (the slow-request alert filters `duration:>1000`) |
| Log `trace_id`, `span_id` | `<hex32>`, `<hex16>` when hex |
| Log `request_body` | `<present>`: the harness sets `LOG_REQUEST_BODY=true`, and the value would hold the passwords of login and register bodies |

Eleven gate nodes record their body in a reduced form, listed in
`gateBodyModes` (`gate_test.go`) with the reason; their status, headers and
log lines stay exact. The events catalog is re-encoded with arrays sorted (it
is built from a Go map). The by-name cursor searches of organizations and
email templates, and the organization-licence cursor search, record only the
JSON shape (keys, types, array lengths): all
their items share one name, so the order follows the random id tie-break.

Everything else, including `level` (lowercase), the message key `message`,
`method`, `path`, `status`, `request_id`, `env`, `tenant_id`, `user_id`,
`principal_type`, `api_key_id` and `api_key_name`, is compared as is.

## Contract notes a migration must keep (or change on purpose)

Observed on main `4b10d222` and pinned by the goldens:

- Auth refusals write **no log line at all**: the auth middleware wraps the
  request logger (oapi-codegen applies the last middleware outermost), so a
  401 or a 403 from the middleware never reaches the summary line
  (`seams/auth-*.golden`, `gate/auth/me-noauth.golden`).
- The OpenAPI validator runs before auth and answers in `text/plain;
  charset=utf-8` with `X-Content-Type-Options: nosniff`: 404 `no matching
  operation was found`, 405 `method not allowed`, and 400 with the kin-openapi
  text, multi-line for path patterns (`Schema:\n  null\nValue:\n  null`). An
  unauthenticated request with a malformed id gets 400, not 401.
- `/health` skips the validator: HEAD and other methods get chi's 405 with
  `Allow: GET` and an empty body. A `net/http.ServeMux` GET pattern would answer
  HEAD with 200.
- HEAD on a `/v1` GET operation and OPTIONS without CORS preflight headers get
  the validator's 405 `text/plain` (HEAD with an empty body), not 200
  (`seams/route-head-v1.golden`, `seams/route-options-no-preflight.golden`).
- An inbound webhook body over 1 MiB gets 400 `text/plain` `Cannot read
  webhook body` before the request id is set: no `X-Request-Id` header and no
  log line (`seams/webhook-oversized.golden`).
- The licence migration's `migrated_at` is serialised in the server's local
  zone (`time.Now()` without `.UTC()`); it reads `Z` only because production
  runs in UTC.
- Product-event deliveries go out with `User-Agent: Go-http-client/1.1`,
  `Accept-Encoding: gzip`, a `pevt_` `webhook-id`, a Unix-seconds
  `webhook-timestamp`, a `v1,<base64>` signature and a body with no trailing
  newline (`gate/product-event-delivery/*.golden`).
- `POST /v1/auth/refresh` refusals are 401 with an **empty body and no
  Content-Type**, only the clearing `Set-Cookie`.
- Decoding is lenient: unknown members ignored, member names matched without
  case, the last duplicate wins, data after the JSON value ignored, any
  Content-Type (or none) accepted, no body size limit (2 MiB accepted). The
  raw `encoding/json` error reaches the client as `BAD_REQUEST` (`can't decode
  JSON body: json: cannot unmarshal number into Go struct field
  ProductRequest.name of type string`).
- Validation errors are 400 `VALIDATION_ERROR` with
  `metadata.{field,param,rule,value}` and no top-level `field`.
- 4xx log lines: a `Request error` line at `info` with `error`, then the
  summary at `info`; `duration` is in milliseconds; `caller` is an absolute
  `file:line`; owner requests carry `user_id`, `tenant_id`, `principal_type`
  and `env` (`development` here, from `ENVIRONMENT`), product-key requests
  carry `api_key_id` and `api_key_name`.
- Two JWTs issued for the same session in the same second are identical (no
  `jti`): a refresh right after register returns the register access token.
- With `LOG_REQUEST_BODY=true` (the test harness), the summary line logs the
  login and register bodies, passwords included; only the evaluation,
  introspection and validation routes are excluded.

Not pinned (no fault injection in-process): the unmodelled 500
`UNEXPECTED_ERROR` and its `error`-level log lines (the 5xx log alert), the
499 on client cancel, 503 `ACCESS_DECISIONS_UNAVAILABLE` and 429
`EMAIL_RATE_LIMIT_EXCEEDED`. No golden holds a `warn` or `error` line; check
those alert paths by hand on dev after a logging step.

Baseline benchmarks for the same commit: `docs/benchmarks/baseline-v0.20.0.md`
at the repository root.

## Gate assertion coverage

<!-- coverage:begin (generated by TestCoverageTable; run with -update) -->
Distinct gate refusal patterns (status + `errors[0].code`): **101 covered, 2 not covered**.

Gate refusal nodes: **310 of 310** have a golden.

| Status | errors[0].code | Gate nodes | Golden case | Also asserted |
|---|---|---:|---|---|
| 400 | `(status only)` | 3 | `gate/access-evaluation/bad-subject-type.golden` +2 |  |
| 400 | `BAD_REQUEST` | 1 | `gate/invitations/create-bad-email.golden` |  |
| 400 | `CUSTOM_ROLES_DISABLED` | 5 | `gate/organization-roles/custom-off.golden` +4 |  |
| 400 | `EMAIL_TEMPLATE_RENDER_ERROR` | 1 | `gate/email-templates/preview-bomb.golden` |  |
| 400 | `INTEGRATION_INVALID_CONFIG` | 2 | `gate/integration/update-host-nopw.golden` +1 | `$.errors[0].message contains password` |
| 400 | `INTEGRATION_PROVIDER_INVALID` | 1 | `gate/integration/bad-provider.golden` |  |
| 400 | `INVALID_ACCESS_ACTION` | 1 | `gate/access-evaluation/item-no-action.golden` |  |
| 400 | `INVALID_ACCESS_RESOURCE` | 2 | `gate/access-evaluation/ws-no-id.golden` +1 |  |
| 400 | `INVALID_CURSOR` | 16 | `gate/cursor-search/bad-cursor.golden` +15 |  |
| 400 | `INVALID_EVENT_ENDPOINT_URL` | 2 | `gate/product-event-delivery/bad-url.golden` +1 |  |
| 400 | `INVALID_PRODUCT_ORGANIZATION_API_KEY_PREFIX` | 1 | `gate/product/invalid-prefix.golden` |  |
| 400 | `INVALID_RESOURCE_ID` | 1 | `gate/resources-registry-roles-on-resources-access/bad-id.golden` |  |
| 400 | `INVALID_RESOURCE_PARENT` | 1 | `gate/resources-registry-roles-on-resources-access/folder-under-org.golden` | `$.errors[0].metadata.allowed_parent_types contains project` |
| 400 | `INVITATION_CODE_IS_INVALID` | 2 | `gate/auth/register-reused-code.golden` +1 |  |
| 400 | `INVITATION_CODE_NOT_PROVIDED` | 1 | `gate/auth/register-nocode.golden` |  |
| 400 | `LICENSE_FIELD_NOT_A_LIMIT` | 1 | `gate/licensing/usage-not-a-limit.golden` | `$.errors[0].field equals sso` |
| 400 | `ORGANIZATION_API_KEY_EXPIRES_AT_IN_PAST` | 1 | `gate/organization-api-key/past-exp.golden` |  |
| 400 | `ORGANIZATION_INVITATION_EXPIRY_NOT_IN_FUTURE` | 2 | `gate/invitations/update-f-past.golden` +1 |  |
| 400 | `PERMISSIONS_NOT_FOUND` | 3 | `gate/organization-api-key/unknown-permission.golden` +2 |  |
| 400 | `PERMISSION_OUTSIDE_ROLE_SCOPE` | 1 | `gate/resource-types/role-outside.golden` |  |
| 400 | `PRODUCT_API_KEY_PERMISSIONS_IMMUTABLE` | 1 | `gate/product-api-key/update-immutable-perms.golden` |  |
| 400 | `PRODUCT_USER_NOT_FOUND_IN_REQUEST` | 3 | `gate/invitations/accept-f-unknown-user.golden` +2 |  |
| 400 | `RESOURCE_TYPE_CYCLE` | 1 | `gate/resource-types/patch-cycle.golden` |  |
| 400 | `RESOURCE_TYPE_NOT_FOUND` | 2 | `gate/resource-types/create-unknown-parent.golden` +1 |  |
| 400 | `RESOURCE_TYPE_PARENT_REQUIRED` | 1 | `gate/resource-types/create-no-parent.golden` |  |
| 400 | `RESOURCE_TYPE_REQUIRED` | 2 | `gate/resource-types/perm-no-type.golden` +1 |  |
| 400 | `RESOURCE_TYPE_TOO_DEEP` | 1 | `gate/resource-types/create-too-deep.golden` |  |
| 400 | `ROLE_NOT_FOUND_IN_REQUEST` | 9 | `gate/access-isolation-and-privilege-changes/globex-custom.golden` +8 |  |
| 400 | `ROLE_NOT_ORGANIZATION_SCOPED` | 5 | `gate/register-on-assign-organization-role-scope/scope-add.golden` +4 | `$.errors[0].metadata.role_id equals {{role-project-admin.id}}`<br>`$.errors[0].metadata.resource_type equals project` |
| 400 | `ROLE_SCOPE_MISMATCH` | 2 | `gate/register-on-assign-organization-role-scope/reg-scope-mismatch.golden` +1 |  |
| 400 | `SINGLE_ROLE_PER_RESOURCE` | 1 | `gate/resources-registry-roles-on-resources-access/assign-two.golden` |  |
| 400 | `UNKNOWN_EVENT_TYPE` | 1 | `gate/product-event-delivery/unknown-event.golden` |  |
| 400 | `UNSUPPORTED_ACCESS_OVERRIDE` | 2 | `gate/access-evaluation/override-resource.golden` +1 |  |
| 400 | `VALIDATION_ERROR` | 20 | `gate/auth/register-weak-password.golden` +19 | `$.errors[0].metadata.field equals sort_by`<br>`$.errors[0].metadata.field equals Subject`<br>`$.errors[0].metadata.value equals archived` |
| 401 | `(status only)` | 6 | `gate/auth/refresh-no-cookie.golden` +5 |  |
| 401 | `EMAIL_OR_PASSWORD_INCORRECT` | 3 | `gate/auth/login-badpass.golden` +2 |  |
| 401 | `INVALID_PRODUCT_API_KEY` | 13 | `gate/access-isolation-and-privilege-changes/other-read.golden` +12 |  |
| 401 | `UNAUTHORIZED` | 15 | `gate/access-isolation-and-privilege-changes/platform-assign.golden` +14 |  |
| 403 | `(status only)` | 7 | `gate/access-evaluation/no-scope.golden` +6 |  |
| 403 | `EMAIL_DRAFT_SEND_FORBIDDEN` | 1 | `gate/email-templates/send-draft-by-key.golden` |  |
| 403 | `PRODUCT_API_KEY_INSUFFICIENT_PERMISSIONS` | 8 | `gate/invitations/create-read-only.golden` +7 | `$.errors[0].metadata.required_scopes[0] equals organization_invitation:create`<br>`$.errors[0].metadata.required_scopes[0] equals organization_invitation:update`<br>`$.errors[0].metadata.required_scopes[0] equals organization_invitation:delete`<br>`$.errors[0].metadata.required_scopes contains resource:write` |
| 403 | `SELF_DELETION_NOT_ALLOWED` | 1 | `gate/platform-users/self-delete.golden` |  |
| 404 | `(status only)` | 7 | `gate/access-evaluation-user-writes-apply-at-once/verify-org-gone.golden` +6 |  |
| 404 | `EMAIL_TEMPLATE_NOT_FOUND` | 1 | `gate/email-templates/verify-gone.golden` |  |
| 404 | `INTEGRATION_INSTANCE_NOT_FOUND` | 2 | `gate/integration/ingest-clerk-webhook-missing.golden` +1 |  |
| 404 | `LICENSE_SCHEMA_NOT_FOUND` | 1 | `gate/licensing/verify-schema-gone.golden` |  |
| 404 | `LICENSE_TEMPLATE_NOT_FOUND` | 1 | `gate/licensing/verify-template-gone.golden` |  |
| 404 | `ORGANIZATION_API_KEY_NOT_FOUND` | 3 | `gate/organization-api-key/verify-gone-key.golden` +2 |  |
| 404 | `ORGANIZATION_INVITATION_NOT_FOUND` | 11 | `gate/invitations/get-a-gone.golden` +10 |  |
| 404 | `ORGANIZATION_LICENSE_NOT_FOUND` | 1 | `gate/licensing/get-license-x.golden` |  |
| 404 | `ORGANIZATION_MEMBERSHIP_NOT_FOUND` | 10 | `gate/access-isolation-and-privilege-changes/globex-member.golden` +9 |  |
| 404 | `ORGANIZATION_NOT_FOUND` | 11 | `gate/invitations/create-unknown-org.golden` +10 |  |
| 404 | `PERMISSION_NOT_FOUND` | 1 | `gate/product-permissions/get-missing.golden` |  |
| 404 | `PLATFORM_INVITATION_NOT_FOUND` | 3 | `gate/auth/invitation-consumed.golden` +2 |  |
| 404 | `PLATFORM_USER_NOT_FOUND` | 2 | `gate/platform-users/get-missing.golden` +1 |  |
| 404 | `PRODUCT_API_KEY_NOT_FOUND` | 3 | `gate/product-api-key/get-deleted-key.golden` +2 |  |
| 404 | `PRODUCT_NOT_FOUND` | 6 | `gate/product-event-delivery/catalog-unknown-product.golden` +5 |  |
| 404 | `PRODUCT_USER_NOT_FOUND` | 4 | `gate/product-user/get-unknown.golden` +3 |  |
| 404 | `RESOURCE_MEMBER_NOT_FOUND` | 1 | `gate/access-isolation-and-privilege-changes/role-gone.golden` |  |
| 404 | `RESOURCE_NOT_FOUND` | 13 | `gate/access-isolation-and-privilege-changes/globex-read.golden` +12 |  |
| 404 | `RESOURCE_PERMISSION_NOT_FOUND` | 6 | `gate/resource-permission/get-missing.golden` +5 |  |
| 404 | `RESOURCE_TYPE_NOT_FOUND` | 2 | `gate/resource-types/get-page-gone.golden` +1 |  |
| 404 | `RESOURCE_TYPE_NOT_REGISTRABLE` | 1 | `gate/resources-registry-roles-on-resources-access/path-organization.golden` |  |
| 404 | `ROLE_NOT_FOUND` | 11 | `gate/access-isolation-and-privilege-changes/globex-read-custom.golden` +10 |  |
| 405 | `(status only)` | 1 | `gate/product-user-external-id/user-patch.golden` |  |
| 409 | `AUTHZ_MODEL_TOO_LARGE` | 1 | `gate/authz-model-size-limit/tree-too-large.golden` | `$.errors[0].metadata.max_model_bytes equals 1048576`<br>`$.errors[0].metadata.model_bytes regex ^[0-9]{7,}$` |
| 409 | `BUILT_IN_RESOURCE_TYPE` | 3 | `gate/resource-types/create-organization.golden` +2 |  |
| 409 | `EMAIL_INTEGRATION_NOT_CONFIGURED` | 1 | `gate/email-templates/send-email-no-integration.golden` |  |
| 409 | `EMAIL_TEMPLATE_NOT_PUBLISHED` | 1 | `gate/email-templates/preview-unpublished.golden` |  |
| 409 | `EMAIL_TEMPLATE_SLUG_TAKEN` | 1 | `gate/email-templates/duplicate-slug.golden` |  |
| 409 | `INTEGRATION_INSTANCE_ALREADY_EXISTS` | 1 | `gate/integration/dup-create.golden` |  |
| 409 | `INVITATION_ALREADY_EXISTS` | 1 | `gate/platform-invitation/create-duplicate.golden` |  |
| 409 | `INVITATION_USER_ALREADY_EXISTS` | 1 | `gate/platform-invitation/create-existing-user.golden` |  |
| 409 | `LICENSE_SCHEMA_EXISTS` | 1 | `gate/licensing/schema-conflict.golden` |  |
| 409 | `LICENSE_TEMPLATE_ARCHIVED` | 1 | `gate/licensing/instantiate-archived.golden` |  |
| 409 | `LICENSE_TEMPLATE_IN_USE` | 1 | `gate/licensing/delete-template-in-use.golden` |  |
| 409 | `LICENSE_TEMPLATE_NAME_EXISTS` | 1 | `gate/licensing/template-name-conflict.golden` |  |
| 409 | `ORGANIZATION_API_KEY_INACTIVE_OR_EXPIRED` | 1 | `gate/organization-api-key/reactivate-expired.golden` |  |
| 409 | `ORGANIZATION_API_KEY_NAME_DUPLICATE` | 2 | `gate/concurrent-unique-names/okey-late.golden` +1 |  |
| 409 | `ORGANIZATION_INVITATION_ALREADY_ACCEPTED` | 2 | `gate/invitations/accept-a-again.golden` +1 |  |
| 409 | `ORGANIZATION_INVITATION_ALREADY_PENDING` | 1 | `gate/invitations/invite-e-case.golden` |  |
| 409 | `ORGANIZATION_INVITATION_EMAIL_IS_MEMBER` | 1 | `gate/invitations/invite-member-email.golden` |  |
| 409 | `ORGANIZATION_INVITATION_EXPIRED` | 2 | `gate/invitations/accept-x.golden` +1 |  |
| 409 | `ORGANIZATION_LICENSE_EXISTS` | 1 | `gate/licensing/instantiate-twice.golden` |  |
| 409 | `ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS` | 2 | `gate/invitations/accept-d-by-member.golden` +1 |  |
| 409 | `PARENT_NOT_REGISTERED` | 2 | `gate/register-on-assign-organization-role-scope/reg-parent-missing.golden` +1 |  |
| 409 | `PERMISSION_SCOPE_IMMUTABLE` | 1 | `gate/resource-types/perm-rescope.golden` |  |
| 409 | `PRODUCT_ALREADY_EXISTS` | 2 | `gate/product/create-duplicate.golden` +1 |  |
| 409 | `PRODUCT_API_KEY_NAME_DUPLICATE` | 1 | `gate/product-api-key/create-duplicate.golden` |  |
| 409 | `PRODUCT_PROTECTED` | 1 | `gate/product/delete-protected-product.golden` |  |
| 409 | `PRODUCT_USER_EMAIL_ALREADY_EXISTS` | 1 | `gate/product-user/duplicate-email.golden` |  |
| 409 | `PRODUCT_USER_EXTERNAL_ID_ALREADY_EXISTS` | 1 | `gate/product-user-external-id/user-dup.golden` |  |
| 409 | `RESOURCE_CYCLE` | 1 | `gate/resources-registry-roles-on-resources-access/cycle.golden` |  |
| 409 | `RESOURCE_IN_USE` | 2 | `gate/resources-registry-roles-on-resources-access/delete-in-use.golden` +1 | `$.errors[0].metadata.children equals 1`<br>`$.errors[0].metadata.role_assignments equals 0`<br>`$.errors[0].metadata.role_assignments equals 1` |
| 409 | `RESOURCE_PERMISSION_ALREADY_EXISTS` | 2 | `gate/concurrent-unique-names/perm-late.golden` +1 |  |
| 409 | `RESOURCE_TYPE_IN_USE` | 3 | `gate/resource-types/delete-in-use.golden` +2 | `$.errors[0].metadata.blocked_by.child_types equals 1`<br>`$.errors[0].metadata.blocked_by.roles equals 1`<br>`$.errors[0].metadata.blocked_by.permissions equals 1`<br>`$.errors[0].metadata.role_ids contains {{role-project.rid}}`<br>`$.errors[0].metadata.role_ids[0] equals {{role-folder-viewer.id}}` |
| 409 | `RESOURCE_TYPE_SLUG_DUPLICATE` | 1 | `gate/resource-types/create-duplicate.golden` |  |
| 409 | `ROLE_IN_USE` | 3 | `gate/invitations/delete-role-three-held.golden` +2 |  |
| 409 | `ROLE_NAME_DUPLICATE` | 5 | `gate/concurrent-unique-names/role-late.golden` +4 |  |
| 409 | `ROLE_NOT_EDITABLE` | 2 | `gate/organization-roles/edit-managed.golden` +1 |  |
| 409 | `ROLE_SCOPE_IMMUTABLE` | 1 | `gate/resource-types/role-rescope.golden` |  |

Asserted response headers (all covered by the goldens of the nodes that assert them):

- `Content-Type` contains `application/json` (18)
- `Set-Cookie` contains `Expires=Thu, 01 Jan 1970` (7)
- `Set-Cookie` contains `refresh_token=` (3)
- `webhook-signature` starts_with `v1,` (10)

Not covered:

- `404 WORKSPACE_NOT_FOUND`: flow `anchor: workspace` is tagged `anchor-retired` and failing; the workspace routes were dropped (migration 000073)
- `409 WORKSPACE_NAME_DUPLICATE`: same retired flow

Refusal nodes without a golden (skipped, or downstream of a skipped node):


Skipped gate nodes (assertions not enforced in-process):

- `email-templates/preview-optional-given`: needs the Echopoint workspace variable emailOptionalVarBody, which the flow export does not carry
- `email-templates/preview-optional-missing`: needs the Echopoint workspace variable emailOptionalVarBody, which the flow export does not carry
<!-- coverage:end -->
