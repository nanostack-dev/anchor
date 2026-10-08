# Verified development troubleshooting

| Symptom | Verified cause and repair | Verify |
| --- | --- | --- |
| Lint reports `0 issues` from root | Root has no Go module; run lint in `apps/anchor` with `../../.golangci.yml`. | Inspect the actual module invocation and exit status. |
| Local UI cannot call shared dev API | Shared dev CORS does not allow the local origin; use the managed disposable app or documented read-only smoke proxy. | Browser guide's health/source checks and affected journey. |
| Current manual app asks for a missing secret file | `application.yaml` uses `${file:...}` inputs; legacy inline-password samples do not supply required file paths. Use managed runtime or approved local file-backed config. | Local `/health` after startup; no secret contents in evidence. |
| Product API key still refuses a newly granted scope | Out-of-band edits do not evict the permission cache. Change scopes through the management API. | Read back key permissions and retry the authorized request; see [cache guide](../api-key-permission-cache.md). |
| Browser UI shows only bootstrap or no scenarios | Playwright retains project/text/status filters. Open its current root URL, clear filters and select bootstrap + chromium. | Compare displayed scenarios with CLI discovery. |

Detailed reproduced failures and repairs stay in the [browser troubleshooting guide](../../anchor-ui/e2e/guide/troubleshooting.md) and [CI evidence guide](../../anchor-ui/e2e/guide/ci-failures.md). Add a recipe only after a cause and passing repair are verified; unresolved hypotheses belong in research or the issue.
