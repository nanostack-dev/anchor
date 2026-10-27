# Coding standards

Judgement rules a reviewer applies to every changed hunk. Each one comes from a correction the maintainer made more than once in review. A rule a tool can check lives in lint instead: see [Enforced by tooling](#enforced-by-tooling).

Report each violation as `[standards] file:line | rule | minimal fix`, next to the correctness findings. A violation blocks the change like a defect does.

## 1. Comments are rare, short and inline

Name things so the code explains itself. Comment only the line a careful reader would misread, with one or two lines placed beside it. A block above a function or type that retells its body is a violation; so is a comment that restates the line below it.

Example: a `// Validate the input` line above `validate(input)` restates the code; delete it.

Prose in docs, ADRs and PR bodies is not affected.

## 2. Build what was asked

Ship the smallest design that fully solves the request. A safeguard, lint rule, wrapper type, builder, interface with one implementation or option nobody asked for is a violation: propose it in the PR body instead, and let the maintainer pull it in.

Example: a new rule in `apps/anchor/gorules/` added while fixing an unrelated bug; drop it and suggest it in the PR body.

## 3. One decision lives in one place

Logic that decides one thing sits in one function that reads top to bottom: gather the inputs, decide, act. Split it only when a second caller exists. Files are named after what they own; several files that each hold a slice of one auth decision mean the split is wrong.

Example: a `requireMember` that calls a one-use `AccessCheck` builder; inline the builder.

## 4. Reuse what the stack already has

Before writing a helper, client call or component, look for it in the pinned nanostack-framework, pgkit, the Anchor Go SDK in `clients/go`, the generated UI client in `anchor-ui/src/client` and `@nanostackorg/design-system`. A hand-written HTTP call where a generated client exists, a local utility duplicating a framework one, or a raw element where a design-system component exists is a violation.

Example: a test fixture that builds its own HTTP request to an Anchor endpoint; call the `clients/go` SDK instead.

## 5. Names say what the code checks or does

A function name states its effect on the domain (`requireWorkspaceMember`, `resolveCallerWorkspace`), not its mechanism (`authenticateScheme`, `handle`, `process`). A reader should not need the body to know what a call decides.

Example: an `authenticateScheme` that rejects a caller outside the workspace; rename it `requireWorkspaceMember`.

## 6. Constraints match the domain

Uniqueness, locks and validation cover exactly what the domain requires. Ask what the value is for and who contends for the lock: a display name is not unique, and a lock key shared by unrelated callers serializes them for nothing.

Example: a unique index on an organization's display name, or one lock key taken by every organization creation; drop both.

## Enforced by tooling

These rules fail lint or the UI contract check, so a reviewer does not repeat them:

- Scroll regions use the design-system `ScrollArea`, never `overflow-*-auto|scroll` classes (`anchor-ui/scripts/check-ui-contract.mjs`).
- Design-system components take no `className` or `style`, and UI imports only public design-system entry points (`anchor-ui/scripts/check-ui-contract.mjs`).
- Raw palette and arbitrary colors are banned in favour of tokens (`anchor-ui/scripts/check-ui-contract.mjs`).
- `functional.Seq` chains use the fused forms and a row-locked context stays inline in the call it locks (`apps/anchor/gorules/`).
