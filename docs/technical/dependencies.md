# Dependency ownership and standalone builds

A standalone clone contains Anchor's source, agent guides, contract, generators and test fixtures. Ordinary builds resolve released dependencies from [apps/anchor/go.mod](../../apps/anchor/go.mod), [clients/go/go.mod](../../clients/go/go.mod), [anchor-ui/package.json](../../anchor-ui/package.json) and the root lockfile. The only local Go replacement is `../../clients/go`, inside this same repository. No Nanostack workspace or sibling checkout is needed.

| Owner | Anchor use | When to read its source |
| --- | --- | --- |
| [nanostack-framework](https://github.com/nanostack-dev/nanostack-framework) | Validation, functional values, faults, config, logging, tracing, health and infrastructure modules | Before changing framework integration; inspect the pinned Go revision/API. |
| [pgkit](https://github.com/nanostack-dev/pgkit) | Persisted queue/lock primitives | Before changing durable jobs, lock or retry behavior; app event semantics remain in Anchor. |
| [nanostack-design-system](https://github.com/nanostack-dev/nanostack-design-system) | Shared React components, layout and tokens | Before UI work; use source and `src/styles.css` on `origin/main`, reconciling the app's package pin. |
| [echopoint-cli](https://github.com/nanostack-dev/echopoint-cli) | Runs deployed API flow suites selected by `anchor` tag | Before updating flow definitions/assertions or diagnosing post-deploy coverage. |
| [ci-workflows](https://github.com/nanostack-dev/ci-workflows) | Reusable verification jobs tracking the latest `main`; backend lint reads the committed tool-version file | Read the workflow currently referenced by [.github/workflows/go.yml](../../.github/workflows/go.yml). |
| [infra](https://github.com/nanostack-dev/infra) | Stack/environment rendering, deployment, post-deploy suites and promotion | Before adding runtime environment inputs, releasing or rollback. |

Owner-managed reusable workflows under `nanostack-dev/ci-workflows/.github/workflows/` use `@main` to consume the latest merged revision. This is an explicit exception to dependency pinning. Merge a required companion workflow change first, then validate the caller against `@main`; a temporary PR-head reference can become unavailable after a squash merge and branch deletion. Keep the golangci-lint version pinned in `.golangci-lint-version` for both local runs and CI. This exception does not change third-party action or tool version policy.

Frameworks outside Nanostack include Uber FX wiring, go-jet generated SQL, OpenAPI/oapi-codegen and React/TanStack/Vite/Playwright. Exact versions belong in module/package lockfiles, not a second version list in prose.

A local clone of a dependency is an optional source-inspection shortcut. Without one, read its GitHub guide/source at the relevant ref, or create an isolated checkout wherever convenient; never assume a parent/sibling path. Normal product work uses the published pin. A neutral reusable UI change goes to the design-system repository; Anchor's screens/permissions remain here. Keep each dependency change in its owner's PR and link companion PRs when contracts move together.

## Security refreshes

Align the Go toolchain in CI and the runtime image. Regenerate and commit the UI
client when updating its generator, then verify HTTP-status and transport-error
handling as well as authenticated browser journeys. Security overrides apply
only to affected dependency ranges; preserve release-age checks for other
packages. Verify frozen install, audit, typecheck and build before deployment.
The static Go binary runs in a digest-pinned Debian 13 distroless nonroot image;
refresh that digest for CA and timezone package advisories, and scan the final
published image rather than assuming a compiler rebuild refreshes the base.
