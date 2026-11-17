# Anchor email send answered 500 on a transient SMTP AUTH reply and turned a gate flow red

| | |
|---|---|
| **Date** | 2026-11-17 |
| **Severity** | SEV4 |
| **Status** | In review |
| **Service** | anchor |
| **Authors** | Nanostack routine (echopoint-investigation) |
| **Duration** | 17m 21s (trigger to resolution) |
| **Time to detect** | 4m 40s |
| **Time to mitigate** | 17m 21s |

> Blameless: this document names systems, processes and missing guards, not people.

## Summary

On 2026-11-17 at 12:09Z the dev Anchor API answered `500 EMAIL_DELIVERY_FAILED` to one of four parallel email sends in the flow `Parallel email sends reach the right email step`, so the anchor flow suite ran 34/35. The SMTP relay of the suite product had answered AUTH with `454 4.7.0 Temporary authentication failure` after 11 s, and no layer of Anchor retries a transient 4yz reply. It was the seventh such reply since 2026-11-06, all on dev; the shared ledger had filed the first six as `expected` and never escalated them. The flow is in the `anchor` gate tag, so the same reply during a deploy would hold Anchor's prod promotion. The rerun went green at 12:26Z. Anchor now retries a transient AUTH reply once on a fresh connection, before any message is sent.

## Impact

Dev only, no customer saw it. One red suite run; no gate ran during it, so prod promotion was not held this time. Seven failed email sends on dev since 2026-11-06, each recorded as a FAILED send.

- **7** 454 AUTH replies on dev since 2026-11-06
- **11.2 s** until the 500 reached the flow
- **1 of 35** anchor suite flows red

## Trigger

The suite product's SMTP relay answered AUTH with `454 4.7.0 Temporary authentication failure: Connection lost to authentication server` while the flow sent four emails in parallel.

## Detection

The routine's suite run (`run_suites.sh`) printed `FAILED anchor db6c38bb …: status_code equals expected=201 actual=500` at the end of the anchor tag, 5 minutes after the send. The six earlier replies were seen by the log routine (`grafana-watch`) within hours, but the ledger status `expected (watch)` kept them out of the postmortem loop, so detection of the pattern took 11 days.

## Resolution

A rerun of the flow went green at 12:26:45Z. anchor#395 (merged 12:35:45Z as 0ce3dbb6) makes the Anchor SMTP provider redial and re-authenticate once when AUTH answers 4yz; no message has been sent at that point, so the retry cannot send twice. A relay outage longer than one retry still fails the send, and a gate-tagged real-SMTP flow still holds prod on it (A3).

## Timeline (UTC)

| Time | | Event |
|---|---|---|
| 06:19:41 | T−11d 05h | **First 454 AUTH reply on dev, ledgered as expected** Logged by `grafana-watch`; the row says to act if it repeats. ([link](http://100.83.213.10:8428/select/logsql/query)) |
| 12:16:45 | T−2d 23h | **Sixth 454: send ends FAILED behind a 201 duplicate** The caller gave up at about 10 s and resent with the same dedupe key; the duplicate got 201 with the in-flight record, then the first attempt failed. |
| 12:09:24 | T+0s | **Flow sends four emails in parallel; send-a starts** ([link](https://api.echopoint.dev/flows/db6c38bb-2ac8-4154-a546-115b7c3f1b4d/executions/01a1e969-72ba-77a1-a4f0-c18d32d461e1/nodes)) |
| 12:09:36 | T+12s | **Relay answers AUTH 454; Anchor answers 500 after 11.2 s** `POST /v1/products/prd_…/email/sends -> 500 (11.210764s)`, task `anchor-dev_anchor.2`. |
| 12:14:04 | T+4m 40s | **run_suites.sh prints FAILED for the flow** |
| ~12:22 | T+12m 36s | **Routine matches ledger row: seventh occurrence** |
| 12:26:45 | T+17m 21s | **Rerun of the flow completes green** |
| 12:26:45 | T+17m 21s | **Anchor suite back to 35/35** |
| 12:27:42 | T+18m 18s | **Fix PR anchor#395 opened: one AUTH retry** ([link](https://github.com/nanostack-dev/anchor/pull/395)) |
| 12:35:45 | T+26m 21s | **anchor#395 merged (0ce3dbb6)** ([link](https://github.com/nanostack-dev/anchor/pull/395)) |

## Root cause analysis: the five whys

Start at the impact and ask why until the answer is something the team can change. Solid edges ask why it happened; dashed edges ask why the impact was as large as it was.

```mermaid
flowchart TD
  P["The anchor flow suite went red on dev, and a gate run at that moment would have held Anchor's prod promotion"]
  W1["`send-a` got `500 EMAIL_DELIVERY_FAILED` from `POST /email/sends`"]
  W2["The SMTP relay answered AUTH with `454 4.7.0 Temporary authentication failure` after 11 s"]
  W3["Anchor fails the send on any AUTH error; nothing retries a 4yz reply, which RFC 5321 marks transient"]
  W4["The provider comment says the email service owns retry, and the service has no retry, so each layer assumed the other retried"]
  R1["R1 · No layer owns retry of a transient SMTP reply, and no test covers a 4yz AUTH reply"]
  W5["Six earlier 454 replies since 2026-11-06 produced no fix, so the seventh reached a gate-tagged flow"]
  W6["The ledger status `expected (watch)` is not one of the loop's known states, so a repeat only appended a note"]
  R2["R2 · The postmortem loop has no rule that escalates a repeating `expected` error to a postmortem"]
  W7["A caller that times out before the relay answers gets a 201 duplicate while the first send ends FAILED"]
  W8["Delivery runs inside the caller's request, so a slow relay is the caller's wait and the caller's failure"]
  R3["R3 · Email dispatch is synchronous to the API request; there is no queued or detached delivery with retries"]
  P -->|why?| W1
  W1 -->|why?| W2
  W2 -->|why?| W3
  W3 -->|why?| W4
  W4 -->|why?| R1
  P -.->|why so bad?| W5
  W5 -->|why?| W6
  W6 -->|why?| R2
  P -.->|why so bad?| W7
  W7 -->|why?| W8
  W8 -->|why?| R3
  class P problem
  class R1,R2,R3 root
  classDef problem fill:#fdecea,stroke:#c2362f,stroke-width:2px,color:#4a1210
  classDef root fill:#e7f5ec,stroke:#1f7a45,stroke-width:2px,color:#0f3a20,font-weight:bold
  classDef open fill:#fff4d9,stroke:#9a6200,stroke-dasharray:5 4,color:#3f2a00
```

- **Problem:** The anchor flow suite went red on dev, and a gate run at that moment would have held Anchor's prod promotion
  - **Why?** `send-a` got `500 EMAIL_DELIVERY_FAILED` from `POST /email/sends` _Evidence: Node result of execution `01a1e969`; VictoriaLogs `POST …/email/sends -> 500 (11.210764s)` at 12:09:36Z_
    - **Why?** The SMTP relay answered AUTH with `454 4.7.0 Temporary authentication failure` after 11 s _Evidence: VictoriaLogs `failed to send email: smtp auth: 454 4.7.0 …` at 12:09:36.242Z; the three other parallel sends got 201_
      - **Why?** Anchor fails the send on any AUTH error; nothing retries a 4yz reply, which RFC 5321 marks transient _Evidence: [provider.go Send](https://github.com/nanostack-dev/anchor/blob/f449eb38/apps/anchor/internal/integration/provider/smtp/provider.go#L285-L312) returns `smtp auth: %w`; [service.go](https://github.com/nanostack-dev/anchor/blob/f449eb38/apps/anchor/internal/email/service/service.go#L790) calls `mailer.Send` once_
        - **Why?** The provider comment says the email service owns retry, and the service has no retry, so each layer assumed the other retried _Evidence: Comment on `Send`: 'The email service is responsible for rendering, idempotency, and retry'; `SendStatusFailed` says 'failed permanently after retries'; no retry in `email/service`_
          - **Why?** No layer owns retry of a transient SMTP reply, and no test covers a 4yz AUTH reply → **Root cause R1** (fixed by A1, A4)
  - **Why so bad?** Six earlier 454 replies since 2026-11-06 produced no fix, so the seventh reached a gate-tagged flow _Evidence: Ledger row `logs:dev:anchor:failed to send email: smtp auth: 454 …`, count 6, status `expected (watch)`_
    - **Why?** The ledger status `expected (watch)` is not one of the loop's known states, so a repeat only appended a note _Evidence: `postmortem-loop.md` §1 lists open, fix-awaiting-review, fix-merged, fix-live, verified; the row grew six notes, each deferring to a human_
      - **Why?** The postmortem loop has no rule that escalates a repeating `expected` error to a postmortem → **Root cause R2** (fixed by A2)
  - **Why so bad?** A caller that times out before the relay answers gets a 201 duplicate while the first send ends FAILED _Evidence: Sixth occurrence 2026-11-14T12:16:45Z: resend with the same dedupe key answered 201 in 2 ms; the first attempt got 454 at 11.7 s_
    - **Why?** Delivery runs inside the caller's request, so a slow relay is the caller's wait and the caller's failure _Evidence: `emailService.Send` dispatches with the request context and only detaches the status write_
      - **Why?** Email dispatch is synchronous to the API request; there is no queued or detached delivery with retries → **Root cause R3** (fixed by A3, A4)

## Root causes

- **R1** No layer owns retry of a transient SMTP reply, and no test covers a 4yz AUTH reply
- **R2** The postmortem loop has no rule that escalates a repeating `expected` error to a postmortem
- **R3** Email dispatch is synchronous to the API request; there is no queued or detached delivery with retries

## Contributing factors

- The suite product sends real mail through a third-party relay, so the relay's availability is a gate input.
- A 500 is not a transport error, so neither `run_suites.sh` nor the Deploy Router gate reruns the flow.

## Lessons learned

### What went well

- The suite run caught the failure on its first pass, and the node result plus VictoriaLogs gave the cause in two queries.
- The four parallel sends showed the failure was one relay reply, not a routing bug in the flow's subject.

### What went wrong

- Six occurrences over 11 days were filed as expected; each run deferred the same design question without opening a draft PR.
- The `Send` comment promised a retry that did not exist.

### Where we got lucky

- No Deploy Router gate ran at 12:09Z; the same reply during a gate would have held Anchor's prod promotion.
- All seven replies hit dev; VictoriaLogs shows no 454 AUTH reply on prod since 2026-11-01.

## Action items

| ID | Action | Type | Owner | Due | Status | Ticket | Fixes |
|---|---|---|---|---|---|---|---|
| A1 | SMTP provider redials and re-authenticates once on a 4yz AUTH reply; unit tests for 454→235, 454→454 and 535 | prevent | Nanostack routine (echopoint-investigation) | 2026-11-17 | done | [anchor#395](https://github.com/nanostack-dev/anchor/pull/395) | R1 |
| A2 | postmortem-loop.md and grafana-watch.md: a repeating `expected` or `noise` row (second time in 14 days, or reaching a suite or gate) gets a postmortem | process | Nanostack routine (echopoint-investigation) | 2026-11-17 | done |  | R2 |
| A3 | Decide on queued or detached email dispatch with bounded retries, so a slow relay neither blocks the caller nor leaves a FAILED send behind a 201 duplicate (ADR) | prevent | Anchor maintainers | 2026-12-01 | open |  | R3 |
| A4 | Alert on `failed to send email` with an SMTP 4yz cause on prod | detect | Anchor maintainers | 2026-12-01 | open |  | R1, R3 |

## Responders

- **Investigation**: Nanostack routine (echopoint-investigation)

## Messaging

### Internal

Dev anchor suite went red on one email send: the SMTP relay answered AUTH 454 (transient) and Anchor does not retry it. Rerun green. Fix: one AUTH retry on a fresh connection. Queued dispatch needs a design decision.

### Customers

None: dev only.

## Supporting information

- [Failed execution 01a1e969](https://api.echopoint.dev/flows/db6c38bb-2ac8-4154-a546-115b7c3f1b4d/executions/01a1e969-72ba-77a1-a4f0-c18d32d461e1/nodes)
- [anchor#395](https://github.com/nanostack-dev/anchor/pull/395)
