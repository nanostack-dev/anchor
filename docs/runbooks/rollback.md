# Recover an unsuccessful Anchor release

Rollback is an operational action owned by [infra](https://github.com/nanostack-dev/infra), with this repository supplying compatible source. This runbook does not authorize a deployment or database change.

## Diagnose the failed release

1. Record service, environment, source SHA and the failing source/deploy run. Inspect `gh run view <deploy-run-id> -R nanostack-dev/infra`; identify whether the failure is backend convergence, health, frontend deploy or post-deploy assertions.
2. Read the API `/health` SHA using [deployment verification](deployment.md). Inspect the frontend job/public surface separately. A failed job can leave a mixture of component versions.
3. Infra's [Swarm deploy workflow](https://github.com/nanostack-dev/infra/blob/main/.github/workflows/deploy-swarm-service.yml) records the previous service image and attempts `Fallback to previous service image` after a failed service update. Inspect that step's outcome and the live SHA. It restores an image; it does not undo database migrations, configuration changes or frontend deployment.

## Choose a compatible recovery

- For a product defect, prepare a focused revert/fix PR from current `origin/main`, run affected validation and use the normal release pipeline. Preserve schema/data compatibility and generated contracts. Link the incident and failed source.
- For an authorized immediate redeploy, use the previously verified source SHA **and its matching published image** through the current infra router payload described in [deployment](deployment.md). Supply the intended environment explicitly and coordinate frontend/source-ref restoration when relevant.
- Before selecting an older binary, inspect every intervening migration and its consumers. A migration's `down.sql` is not proof that a live downgrade is safe. If compatibility is unknown, prefer a forward repair and involve the schema/deployment owner rather than forcing a database version.

## Verify recovery

Wait for the target backend health SHA, confirm the matching frontend job/smoke and execute the relevant API flow suite in the required profile/environment. Check that normal work succeeds and no release-specific error persists. Record source, image, environment, completed/skipped/failed checks and any remaining component mismatch. Add verified diagnosis/repair to technical docs or the relevant runbook; significant incidents warrant a dated postmortem with prevention.
