# Dependency ownership and standalone builds

A standalone clone contains Anchor's source, agent guides, contract, generators and test fixtures. Ordinary builds resolve released dependencies from [apps/anchor/go.mod](../../apps/anchor/go.mod), [clients/go/go.mod](../../clients/go/go.mod), [anchor-ui/package.json](../../anchor-ui/package.json) and the root lockfile. The only local Go replacement is `../../clients/go`, inside this same repository. No Nanostack workspace or sibling checkout is needed.

| Owner | Anchor use | When to read its source |
| --- | --- | --- |
| [nanostack-framework](https://github.com/nanostack-dev/nanostack-framework) | Validation, functional values, faults, config, logging, health and infrastructure modules | Before changing framework integration; inspect the pinned Go revision/API. |
| [pgkit](https://github.com/nanostack-dev/pgkit) | Persisted queue/lock primitives | Before changing durable jobs, lock or retry behavior; app event semantics remain in Anchor. |
| [nanostack-design-system](https://github.com/nanostack-dev/nanostack-design-system) | Shared React components, layout and tokens | Before UI work; use source and `src/styles.css` on `origin/main`, reconciling the app's package pin. |
| [echopoint-cli](https://github.com/nanostack-dev/echopoint-cli) | Runs deployed API flow suites selected by `anchor` tag | Before updating flow definitions/assertions or diagnosing post-deploy coverage. |
| [ci-workflows](https://github.com/nanostack-dev/ci-workflows) | Reusable verification jobs called at `@main` | Read the workflow currently referenced by [.github/workflows/go.yml](../../.github/workflows/go.yml). |
| [infra](https://github.com/nanostack-dev/infra) | Stack/environment rendering, deployment, post-deploy suites and promotion | Before adding runtime environment inputs, releasing or rollback. |

Frameworks outside Nanostack include Uber FX wiring, go-jet generated SQL, OpenAPI/oapi-codegen and React/TanStack/Vite/Playwright. Exact versions belong in module/package lockfiles, not a second version list in prose.

A local clone of a dependency is an optional source-inspection shortcut. Without one, read its GitHub guide/source at the relevant ref, or create an isolated checkout wherever convenient; never assume a parent/sibling path. Normal product work uses the published pin. A neutral reusable UI change goes to the design-system repository; Anchor's screens/permissions remain here. Keep each dependency change in its owner's PR and link companion PRs when contracts move together.
