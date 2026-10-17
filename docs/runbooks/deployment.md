# Deploy and verify Anchor

Product source belongs in this repository; deployment credentials, runtime stacks and routing belong to [nanostack-dev/infra](https://github.com/nanostack-dev/infra). You can read/follow these links from a standalone clone. A workspace is optional. Apply the task's deployment authorization before performing a release.

## Normal release

1. Finish required validation on the exact PR head, then merge according to the repository delivery rules. [.github/workflows/go.yml](../../.github/workflows/go.yml) builds/verifies the source and dispatches `deploy-service` to infra for backend releases. Anchor frontend releases follow the backend dispatch.
2. Inspect the source run and the infra [Deploy Router](https://github.com/nanostack-dev/infra/blob/main/.github/workflows/router.yml):

   ```sh
   gh run list -R nanostack-dev/anchor --workflow go.yml --limit 5
   gh run view <source-run-id> -R nanostack-dev/anchor
   gh run list -R nanostack-dev/infra --workflow router.yml --limit 10
   gh run view <deploy-run-id> -R nanostack-dev/infra
   ```

   Match the source SHA and the `anchor` service/jobs, including dev deployment, post-deploy flow suite and production promotion. A green source build or merged PR does not prove a deployment. Backend health does not establish frontend deployment.
3. Verify the expected SHA at the actual API:

   ```sh
   curl -fsS https://apidev.tryanchor.dev/health | jq -r .commit_sha
   curl -fsS https://api.tryanchor.dev/health | jq -r .commit_sha
   ```

   In an optional infra clone, `scripts/wait-live.sh anchor <source-sha> <dev|prod>` polls this contract. Record the observed SHA and environment; do not infer them from the branch name.
4. For rendered frontend changes, inspect the matching source-ref frontend deploy job and perform a public/browser smoke on the deployed app. Confirm its configured API target, visible journey and errors. Keep source, backend health, frontend smoke and flow-suite results separate in the release evidence.

## Deployment inputs and ownership

Read infra's [anchor workflow](https://github.com/nanostack-dev/infra/blob/main/.github/workflows/anchor-deploy.yml), [Swarm service deployment](https://github.com/nanostack-dev/infra/blob/main/.github/workflows/deploy-swarm-service.yml) and `stacks/anchor.dev.yml`/`stacks/anchor.prod.yml` before changing runtime inputs. New app environment variables must be wired into those matching stacks and deploy templates alongside the product configuration. File-backed credentials remain in approved provisioning paths, outside repository/PR evidence.

The supported release dispatch pairs `source_repository`, `source_ref`, `image_repository`, `image_tag`, `environment` and `service`; the router validates that contract. For an operator-authorized redeploy, construct the payload from the verified source run's published image and the current router schema, then dispatch with `gh api repos/nanostack-dev/infra/dispatches --input <private-payload-file>`. Reuse the matching source SHA/image, never guess a tag. Reapply the normal health, flow-suite and frontend verification before calling it live.

Use [rollback](rollback.md) for failed releases. No deployment is executed by changing this documentation.
