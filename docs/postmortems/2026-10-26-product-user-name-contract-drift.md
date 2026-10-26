# Create product user with email only answered 400: the server required a name the contract marks optional

| | |
|---|---|
| **Date** | 2026-10-26 |
| **Severity** | SEV3 |
| **Status** | In review |
| **Service** | anchor |
| **Authors** | Nanostack routine (echopoint-investigation) |
| **Time to detect** | 167d 07h |

> Blameless: this document names systems, processes and missing guards, not people.

## Summary

`POST /v1/products/{product_id}/product-users` documents `email` as the only required field, but Anchor answered 400 `VALIDATION_ERROR` (`field: Name, rule: required`) when `name` was left out. The domain input `CreateProductUserInput` has required `Name` since the first snapshot (2026-05-11), and the OpenAPI validator skips request bodies, so nothing compared the two rules; a CT even asserted the 400. The echopoint flow suite found it on 2026-10-26 while covering anchor#268 (`external_id`), the first test to send a product user without a name. No customer report and no log evidence of impact: validation refusals leave no trace. The fix makes `name` optional on create, as the contract says; the lesson is to send each create operation's minimum contract body once.

## Impact

Any client that followed the contract or skipped `.Name()` in the Go SDK got a 400 on product user creation, in dev and prod, for about 5.5 months. Products that are their own identity provider (anchor#268, ADR-0023) are the new callers most likely to create users without a display name. Impact on customers is unknown: 400 validation refusals are not logged with their field and rule.

- **168 days** contract and server disagreed
- **0** log rows that would show a customer hitting it
- **1** operation affected (createProductUser)

## Trigger

The first snapshot of Anchor (98d19607, 2026-05-11) shipped `Name string validate:"required,notblank"` in `CreateProductUserInput` while the contract listed only `email` as required.

## Detection

The echopoint-investigation routine built `anchor: product user external id` for anchor#268 at 2026-10-26T05:13Z. Its create node sent `email` and `external_id` only and got 400 `Name is a required field`. Not sooner because every CT helper, SDK example and existing flow sends a name, and the refusal is not logged.

## Resolution

The flow now sends a name and the email-only check sits in `anchor: known bugs` (tag `known-bugs`, outside CI). [anchor#291](https://github.com/nanostack-dev/anchor/pull/291) makes `name` optional on create; it waits for human review because it reverses a behavior a CT pinned, so a maintainer decides whether the contract or the server is right.

## Timeline (UTC)

| Time | | Event |
|---|---|---|
| 21:16:14 | T+0s | **First snapshot: name required in code, optional in contract** ([link](https://github.com/nanostack-dev/anchor/commit/98d19607)) |
| 15:09:30 | T+166d 17h | **anchor#268 adds external_id to the same input** The new field's bounds match the contract; the existing `Name` rule is unchanged. ([link](https://github.com/nanostack-dev/anchor/pull/268)) |
| 05:13:10 | T+167d 07h | **New #268 flow: create user answers 400 Name required** Execution `01a176a0` of `anchor: product user external id` (prod CI org, dev API). |
| 05:13:47 | T+167d 07h | **Flow sends a name; #268 coverage green** Execution `01a176a1`; green 3x, tagged `anchor`. |
| 05:14:25 | T+167d 07h | **Bug reproduced in `anchor: known bugs`** `user-email-only` expects 201, gets 400. |
| ~05:30 | T+167d 08h | **anchor#291 opened, waiting human review** ([link](https://github.com/nanostack-dev/anchor/pull/291)) |

## Root cause analysis: the five whys

Start at the impact and ask why until the answer is something the team can change. Solid edges ask why it happened; dashed edges ask why the impact was as large as it was.

```mermaid
flowchart TD
  P["A product creating a user with email only, as the contract allows, got 400 from Anchor"]
  W1["`CreateProductUserInput.Name` is `required,notblank`; the contract lists only `email` as required"]
  W2["Two sources state the body rules and nothing compares them: the OpenAPI validator skips request bodies"]
  W3["Tests assert the server's behavior, not the contract: a CT pinned the 400 that the contract forbids"]
  R1["R1 · CTs are written from the implementation; no check that each create operation accepts the minimum body its contract allows"]
  W4["Every test and flow that creates a product user sends a name"]
  W5["Test builders default to the fullest realistic body; the minimal request is never a named case"]
  R2["R2 · Test and flow conventions fill optional fields by default, with no rule to also send the contract minimum"]
  W6["The mismatch stayed unseen for 168 days and its customer impact cannot be measured"]
  W7["A 400 validation refusal leaves no log row with its operation, field and rule"]
  R3["R3 · Client-error refusals are not counted by operation and field, so a contract-vs-server mismatch is invisible"]
  P -->|why?| W1
  W1 -->|why?| W2
  W2 -->|why?| W3
  W3 -->|why?| R1
  W1 -->|why?| W4
  W4 -->|why?| W5
  W5 -->|why?| R2
  P -.->|why so bad?| W6
  W6 -->|why?| W7
  W7 -->|why?| R3
  class P problem
  class R1,R2,R3 root
  classDef problem fill:#fdecea,stroke:#c2362f,stroke-width:2px,color:#4a1210
  classDef root fill:#e7f5ec,stroke:#1f7a45,stroke-width:2px,color:#0f3a20,font-weight:bold
  classDef open fill:#fff4d9,stroke:#9a6200,stroke-dasharray:5 4,color:#3f2a00
```

- **Problem:** A product creating a user with email only, as the contract allows, got 400 from Anchor
  - **Why?** `CreateProductUserInput.Name` is `required,notblank`; the contract lists only `email` as required _Evidence: `apps/anchor/internal/domain/product/user/user_input_types.go` line 13; openapi.yaml createProductUser `required: [email]`; response body `{"field":"Name","rule":"required"}`_
    - **Why?** Two sources state the body rules and nothing compares them: the OpenAPI validator skips request bodies _Evidence: `apps/anchor/cmd/http/server.go:178` `ExcludeRequestBody: true`_
      - **Why?** Tests assert the server's behavior, not the contract: a CT pinned the 400 that the contract forbids _Evidence: `cmd/it/ct/product_user_create_test.go` "Creating a product user with missing name returns a bad request" asserted 400 `Name is a required field` since the first snapshot_
        - **Why?** CTs are written from the implementation; no check that each create operation accepts the minimum body its contract allows → **Root cause R1** (fixed by A1, A2)
    - **Why?** Every test and flow that creates a product user sends a name _Evidence: CT helper `newProductUserBody` sets `Name` from Faker; SDK examples call `.Name()`; all anchor flows send `name`_
      - **Why?** Test builders default to the fullest realistic body; the minimal request is never a named case
        - **Why?** Test and flow conventions fill optional fields by default, with no rule to also send the contract minimum → **Root cause R2** (fixed by A3)
  - **Why so bad?** The mismatch stayed unseen for 168 days and its customer impact cannot be measured
    - **Why?** A 400 validation refusal leaves no log row with its operation, field and rule _Evidence: PostHog logs, service `anchor`, last 24 h, search `Name is a required field`: 0 rows, although this run sent such requests_
      - **Why?** Client-error refusals are not counted by operation and field, so a contract-vs-server mismatch is invisible → **Root cause R3** (fixed by A4)

## Root causes

- **R1** CTs are written from the implementation; no check that each create operation accepts the minimum body its contract allows
- **R2** Test and flow conventions fill optional fields by default, with no rule to also send the contract minimum
- **R3** Client-error refusals are not counted by operation and field, so a contract-vs-server mismatch is invisible

## Contributing factors

- A CT asserted the 400 with its message, so the suite defended the drift instead of catching it.
- anchor#268 reviewed the new `external_id` bounds against the contract but not the neighbouring fields of the same input.
- The Go SDK `Users().Create(email)` builder makes the name look optional, so an SDK caller would hit the 400 only at runtime.

## Lessons learned

### What went well

- Covering a new feature with a flow that sends only what the feature needs found an old bug in minutes.
- The known-bugs flow keeps the failing check visible without blocking the CI gate.

### What went wrong

- A five-month-old contract lie went unnoticed by CTs, SDK tests and 25 anchor flows, all of which send a name.

### Where we got lucky

- No known customer creates product users without a name yet; ADR-0023 products (own identity provider) would have been the first.
- The mismatch refused a request instead of storing bad data, so nothing needs repair.

## Action items

| ID | Action | Type | Owner | Due | Status | Ticket | Fixes |
|---|---|---|---|---|---|---|---|
| A1 | Make `name` optional on product user create, as the contract says; CT for the email-only body (or, if a human decides name is required, change the contract instead) | prevent | Nanostack routine (echopoint-investigation) | 2026-10-27 | open | [anchor#291](https://github.com/nanostack-dev/anchor/pull/291) | R1 |
| A2 | CT per create operation that sends only the contract's required properties and expects 2xx (generated from openapi.yaml) | prevent | anchor maintainers | 2026-11-09 | open |  | R1 |
| A3 | flow-suite-testing skill + anchor testing doc: send each create's minimum contract body once | process | Nanostack routine (echopoint-investigation) | 2026-10-26 | in progress |  | R2 |
| A4 | Count 400 VALIDATION_ERROR by operation, field and rule (metric or info log) and alert on a new pair | detect | anchor maintainers | 2026-11-09 | open |  | R3 |

## Responders

- **Detection, fix and write-up**: Nanostack routine (echopoint-investigation)

## Messaging

### Internal

Anchor answered 400 to product user creates without `name`, although the contract marks it optional (since 2026-05-11). Found by the #268 flow. Fix makes name optional; known-bugs flow tracks it until live.

### Customers

None: no customer report and no evidence of impact.

## Supporting information

- [Report (artifact)](https://claude.ai/artifact/9cd5VjqLrPT86J5Kr7rsVq)
- [anchor#268](https://github.com/nanostack-dev/anchor/pull/268)
- [anchor#291 fix](https://github.com/nanostack-dev/anchor/pull/291)
