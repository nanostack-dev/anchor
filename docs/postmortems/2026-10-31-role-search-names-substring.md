# Role search returned roles nobody asked for: `names` matched substrings and `name_contains` was ignored

| | |
|---|---|
| **Date** | 2026-10-31 |
| **Severity** | SEV3 |
| **Status** | In review |
| **Service** | anchor |
| **Authors** | Nanostack routine (echopoint-investigation) |
| **Duration** | ~1h 04m (trigger to resolution) |
| **Time to detect** | 44m 03s |
| **Time to mitigate** | 1h 02m |

> Blameless: this document names systems, processes and missing guards, not people.

## Summary

`POST /v1/products/{product}/roles/search` has filtered `filter.names` with `LIKE '%name%'` since the v1 snapshot (2026-05-11), although the contract says "exact matches". The `filter.name_contains` field that the contract offers for substrings was never mapped, so the server silently ignored it and returned every role. Nobody noticed until infra#271 added a `workspace_admin` role to the Echopoint product on dev and prod at 22:29Z on 2026-10-31, the first role name that contains another one. At 23:13Z the routine's echopoint suite failed: `echopoint: cursor searches — members, invitations & api keys` looked up `names: ["admin"]` and got two roles. The flow was narrowed by `resource_types` at 23:32Z and the exact-match checks moved to `anchor: known bugs`; the server fix (exact `names`, mapped `name_contains`, near-miss tests) waits for review because clients see the change.

## Impact

Every Anchor API client on dev and prod that searched roles by `names` got every role whose name contains the searched name, and a client that used `name_contains` got every role of the product. The responses looked plausible (more rows, never an error), so the impact on customers is unmeasurable. Echopoint does not call the role search. One routine suite run lost the cursor-search checks for members, invitations and API keys (one flow of 40); the next echopoint post-deploy gate would have failed on the same flow and held the deploy.

- **173 days** contract drift in prod (2026-05-11 to 2026-10-31)
- **1 of 71** suite flows red (cursor searches)
- **0** gates blocked (no deploy in the window)

## Trigger

infra#271 applied the Echopoint `workspace_admin`, `workspace_editor` and `workspace_viewer` managed roles to the Echopoint product on dev (22:29:12Z) and prod (22:29:26Z). `workspace_admin` contains `admin`, so `names: ["admin"]` started to return two roles.

## Detection

The routine's echopoint suite run at 23:13Z: node `admin-role` asserted `$.total equals 1` and got 2. Detection took 44 minutes after the trigger. It took 173 days after the defect, because every fixture and every caller used role names where none contained another, and the existing tests could not tell exact from substring: the repository test even pinned the substring behaviour.

## Resolution

At 23:31Z `anchor: known bugs` gained an own-product reproduction (`kbadmin`, `ws_kbadmin`, `kbviewer`): `names` gives 2 for 1 and `name_contains` gives 3 for 2, twice. At 23:32Z the cursor-search flow's `admin-role` lookup added `resource_types: ["organization"]` and ran green three times. The fix PR filters `names` by equality, maps `name_contains` to an escaped, case-insensitive substring match like the resource search, and adds a near-miss role to the CT and repository tests.

## Timeline (UTC)

| Time | | Event |
|---|---|---|
| ~21:16 | T−173d 01h | **v1 snapshot: role `names` filter is `LIKE '%name%'`** `product_role_repository.go` builds one `LIKE` per name; the API mapper never copies `NameContains`. The contract already said "exact matches" and offered `name_contains`. ([link](https://github.com/nanostack-dev/anchor/commit/98d19607)) |
| 02:40:37 | T−15d 19h | **Cursor-search flow finds role `admin` by `names`, total 1** Created by another session for echopoint#645; it reads the shared dev Echopoint product's catalog. Source: flow `updated_at`. |
| 22:28:47 | T−21s | **infra#271 merged: Echopoint workspace roles** ([link](https://github.com/nanostack-dev/infra/pull/271)) |
| 22:29:08 | T+0s | **`workspace_admin` created on the dev Echopoint product** `created_at` of the role in the failing search response; Terraform job `apply-dev` 22:28:57-22:29:12Z. ([link](https://github.com/nanostack-dev/infra/actions/runs/41248358677)) |
| 22:29:26 | T+18s | **Same roles applied to the prod Echopoint product** Terraform job `apply-prod` 22:29:13-22:29:26Z. No flow suite runs after an Anchor catalog apply. ([link](https://github.com/nanostack-dev/infra/actions/runs/41248358677)) |
| 23:13:11 | T+44m 03s | **Suite: `admin-role` expected total 1, got 2** Execution `01a1943d-038f`; 112 later nodes skipped, cleanup ran. The five org API keys whose cleanup failed (403: the tester never became a member) were removed by the org delete's `ON DELETE CASCADE`. |
| ~23:20 | T+50m 52s | **Contract read: `names` exact, `name_contains` unmapped** |
| 23:31:21 | T+1h 02m | **`anchor: known bugs` reproduces both drifts twice** |
| 23:32:06 | T+1h 02m | **Cursor-search flow narrowed by `resource_types`** |
| ~23:34 | T+1h 04m | **Cursor-search flow green three times** |
| ~23:55 | T+1h 25m | **Fix PR opened, waiting for human review** |

## Root cause analysis: the five whys

Start at the impact and ask why until the answer is something the team can change. Solid edges ask why it happened; dashed edges ask why the impact was as large as it was.

```mermaid
flowchart TD
  P["A client that searched roles by `names: [#quot;admin#quot;]` got `admin` and `workspace_admin`; one that used `name_contains` got every role"]
  W1["The repository filters `names` with `LIKE '%name%'` instead of equality"]
  W2["The v1 code was written that way and no test compared it with the contract's #quot;exact matches#quot;"]
  W3["The CT searched names that share no substring (`Editor_…`, `Admin_…`), so exact and substring answer alike"]
  R1["R1 · Filter tests use fixtures without a near-miss row, so they cannot tell an exact filter from a substring one"]
  W4["`name_contains` reaches the server and is dropped: the API mapper never copies it to the domain filter"]
  W5["Mappers from generated filter types are written by hand, field by field"]
  W6["An unread generated field compiles and passes every test; only a test that sends it would fail"]
  R2["R2 · Nothing checks that every contract filter field of a search reaches its query"]
  W7["It surfaced as a red suite flow, and would have held the next echopoint deploy, though no app code changed"]
  W8["infra#271 changed the shared dev Echopoint catalog that the gate reads, from Terraform, outside any app deploy"]
  W9["The Anchor catalog Terraform applies dev and prod in one run, with no suite between them"]
  R3["R3 · Anchor catalog applies reach dev and prod without running the flow suites that read that catalog"]
  W10["The drift lived 173 days in prod with no report"]
  W11["Both defects return extra rows, never an error, and no caller had a name containing another"]
  W12["The suites searched only shared catalogs whose names happened to be distinct"]
  R4["R4 · No flow checked a search filter on its own fixture with a lookalike value, so the shared data decided what was tested"]
  P -->|why?| W1
  W1 -->|why?| W2
  W2 -->|why?| W3
  W3 -->|why?| R1
  P -->|why?| W4
  W4 -->|why?| W5
  W5 -->|why?| W6
  W6 -->|why?| R2
  P -.->|why so bad?| W7
  W7 -->|why?| W8
  W8 -->|why?| W9
  W9 -->|why?| R3
  P -.->|why so bad?| W10
  W10 -->|why?| W11
  W11 -->|why?| W12
  W12 -->|why?| R4
  class P problem
  class R1,R2,R3,R4 root
  classDef problem fill:#fdecea,stroke:#c2362f,stroke-width:2px,color:#4a1210
  classDef root fill:#e7f5ec,stroke:#1f7a45,stroke-width:2px,color:#0f3a20,font-weight:bold
  classDef open fill:#fff4d9,stroke:#9a6200,stroke-dasharray:5 4,color:#3f2a00
```

- **Problem:** A client that searched roles by `names: ["admin"]` got `admin` and `workspace_admin`; one that used `name_contains` got every role
  - **Why?** The repository filters `names` with `LIKE '%name%'` instead of equality _Evidence: `apps/anchor/internal/repository/product_role_repository.go` (`Name.LIKE("%" + name + "%")`); the failing node's response holds both roles_
    - **Why?** The v1 code was written that way and no test compared it with the contract's "exact matches" _Evidence: Repository test `TestProductRoleRepositorySearchByProductIDFilterByNames` asserted "a partial name is enough"_
      - **Why?** The CT searched names that share no substring (`Editor_…`, `Admin_…`), so exact and substring answer alike _Evidence: `cmd/it/ct/product_role_search_test.go` before the fix_
        - **Why?** Filter tests use fixtures without a near-miss row, so they cannot tell an exact filter from a substring one → **Root cause R1** (fixed by A1, A2)
  - **Why?** `name_contains` reaches the server and is dropped: the API mapper never copies it to the domain filter _Evidence: `mapToSearchProductRoleInput` in `internal/api/product_role_api.go` copies ids, names, permissions, resource types only_
    - **Why?** Mappers from generated filter types are written by hand, field by field
      - **Why?** An unread generated field compiles and passes every test; only a test that sends it would fail
        - **Why?** Nothing checks that every contract filter field of a search reaches its query → **Root cause R2** (fixed by A1, A3)
  - **Why so bad?** It surfaced as a red suite flow, and would have held the next echopoint deploy, though no app code changed _Evidence: The echopoint post-deploy gate runs the `echopoint` tag, which holds the cursor-search flow_
    - **Why?** infra#271 changed the shared dev Echopoint catalog that the gate reads, from Terraform, outside any app deploy _Evidence: Run 41248358677: prep, apply-dev, apply-prod, notify; no flow job_
      - **Why?** The Anchor catalog Terraform applies dev and prod in one run, with no suite between them
        - **Why?** Anchor catalog applies reach dev and prod without running the flow suites that read that catalog → **Root cause R3** (fixed by A4, A5)
  - **Why so bad?** The drift lived 173 days in prod with no report
    - **Why?** Both defects return extra rows, never an error, and no caller had a name containing another
      - **Why?** The suites searched only shared catalogs whose names happened to be distinct
        - **Why?** No flow checked a search filter on its own fixture with a lookalike value, so the shared data decided what was tested → **Root cause R4** (fixed by A5, A6)

## Root causes

- **R1** Filter tests use fixtures without a near-miss row, so they cannot tell an exact filter from a substring one
- **R2** Nothing checks that every contract filter field of a search reaches its query
- **R3** Anchor catalog applies reach dev and prod without running the flow suites that read that catalog
- **R4** No flow checked a search filter on its own fixture with a lookalike value, so the shared data decided what was tested

## Contributing factors

- The other `names` filters (organizations, API keys, workspaces) already use `jetx.BuildStringArrayFilter`; only roles kept the hand-built `LIKE`.
- The cursor-search flow's API key cleanup used the tester's session, whose access comes from a later step (accepting the invitation), so it answered 403 when an earlier node failed.

## Lessons learned

### What went well

- A count assertion on a lookup (`total equals 1`) caught a filter defect that 173 days of green results had hidden.
- The reproduction went into `anchor: known bugs` on an own product before the feature flow changed, so the defect stays visible.

### What went wrong

- The repository test documented the substring behaviour as intended, against the contract next to it.
- A catalog change to dev and prod shipped with a Terraform plan as its only check.

### Where we got lucky

- No echopoint or anchor deploy ran between 22:29Z and 23:32Z, so no post-deploy gate failed and no promotion was held.
- Echopoint does not call the role search, so the new workspace roles did not change any Echopoint authorization.
- The org delete cascades to its API keys; the flow's own key cleanup failed and would otherwise have leaked five keys.

## Action items

| ID | Action | Type | Owner | Due | Status | Ticket | Fixes |
|---|---|---|---|---|---|---|---|
| A1 | Filter `names` by equality, map `name_contains` (escaped, case-insensitive, like resources), add a near-miss role to the CT and repository tests | prevent | Anchor maintainers (review) | 2026-11-07 | open | [anchor#316](https://github.com/nanostack-dev/anchor/pull/316) | R1, R2 |
| A2 | Testing guide line: a filter test includes a near-miss row, so exact and substring answer differently | process | Nanostack routine (echopoint-investigation) | 2026-10-31 | done |  | R1 |
| A3 | Test that every field of each generated search filter is read by its mapper (reflection over the generated structs, with an allowlist) | detect | Anchor maintainers | 2026-11-14 | open |  | R2 |
| A4 | Run the `anchor` and `echopoint` flow tags after the Anchor catalog apply on dev, before apply-prod | detect | Infra maintainers | 2026-11-14 | open |  | R3 |
| A5 | Own-product reproduction in `anchor: known bugs`; cursor-search lookup narrowed by `resource_types` | mitigate | Nanostack routine (echopoint-investigation) | 2026-10-31 | done |  | R3, R4 |
| A6 | Flow-suite-testing skill line: test a search filter on an owned fixture with a lookalike value, and narrow lookups in shared catalogs by every discriminator | process | Nanostack routine (echopoint-investigation) | 2026-10-31 | done |  | R4 |

## Responders

- **Detection, diagnosis and fix**: Nanostack routine (echopoint-investigation)

## Messaging

### Internal

SEV3 anchor: role search `names` matched substrings and `name_contains` was ignored since v1, in dev and prod. infra#271's `workspace_admin` exposed it. Flow narrowed and reproduction in `anchor: known bugs`; fix waits for review because clients see the change.

### Customers

Role search now treats `filter.names` as exact matches and supports `filter.name_contains`, as documented. Clients that relied on partial matches in `names` should move them to `name_contains`.

## Supporting information

- [Report](https://claude.ai/artifact/TDFZ5F5nMnaYhUsDtV9VeL)
- [infra#271](https://github.com/nanostack-dev/infra/pull/271)
- [Terraform run 41248358677](https://github.com/nanostack-dev/infra/actions/runs/41248358677)
