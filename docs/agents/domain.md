# Domain documentation

This repository has one canonical glossary: [CONTEXT.md](../../CONTEXT.md). Read it and relevant [ADRs](../adr/) before exploring or changing domain behavior. Use its terms in types, endpoints, test names and issue titles rather than introducing synonyms.

Update resolved vocabulary in `CONTEXT.md` in the same PR. Keep implementation detail in technical docs and rationale in ADRs. An installed domain-modeling skill can assist; the files and local [delivery workflow](../development/agent-workflow.md) do not depend on one.

Surface a conflict with an existing ADR explicitly. Preserve historical numbers. A reversal adds a new ADR with the prior decision marked superseded. If concurrent changes claim the same new number, renumber the unmerged decision to the next free number and update its title and every reference.

Create ADRs for consequential, costly-to-reverse choices with real alternatives and rationale a future reader needs. Do not manufacture a decision just to populate the folder. Research and postmortems are optional and created only for useful evidence or significant incidents.
