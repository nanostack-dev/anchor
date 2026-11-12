# CI: how `go.yml` stays fast

`go.yml` checks every pull request and every push to `main`. This page explains how it is built, why each optimization exists, what it measured, and what was tried and dropped. Read it before you change the workflow or the scripts it calls.

All jobs run on GitHub-hosted runners. They are free because this repository is public. The backend jobs use `ubuntu-24.04-arm`.

## Results

Times are per job with warm caches. They include the 6–13 s that a job spends restoring its cache.

| Change in the pull request | Before (2026-11-09/10) | Now (2026-11-12) |
| --- | --- | --- |
| No Go change | one `dagger-backend` job, 684–805 s; run 12–13.5 min | every backend job ≤ 27 s; run 89–117 s |
| One line in a service package (#383, #385) | same | lint 54–56 s, slowest test part 51–53 s; run 83–86 s; frontend skipped |
| A package almost everything imports (`internal/repository`) | same | lint 59–70 s, test parts 46–64 s; run ~100 s |
| `go.sum` (everything compiles) | same | ~200 s |
| `e2e-app` | 264–385 s | 203–248 s |

The bar: lint and each test job under 1 min, and a cached binary build under 10 s. A typical change meets it. A change to a package that nearly everything imports is 0–10 s over on lint and the slowest part. On these runners that is the floor: about 13 s of setup, then about 22 s of compile through a chain of packages that each wait for the one before (repository → service → api → app → test package), then the tests.

## Layout

- **`detect_changes`** decides which surfaces a pull request touched (backend, frontend). A push to `main` runs everything, because the deploy dispatch follows the jobs.
- **`backend`** is a matrix with one runner per group:
  - `lint`
  - `component-tests-a`..`p` (16 parts)
  - `service-tests-a`/`b`
  - `unit-and-wiring-tests`
  - `generated-code`
  - `image-builds`
- **`frontend`** is a matrix: lint/types/API client, unit tests and build, and Storybook in 4 shards. On a pull request it runs only when a frontend file changed.
- **`publish`** builds the binary and pushes the image while the checks run.
- **`ci-ok`** is the one check to require. It passes when every job succeeded or was skipped.

## Optimizations and why

Each item lists the change, the reason, and the measured effect. PR numbers are in this repository.

### 1. One runner per check instead of one big job (#363)

- **Change:** the single `dagger-backend` job became the `backend` matrix above. Lint and the Go tests run natively (`scripts/lint.sh`, `scripts/go-test-part.sh`). The Dagger checks call the same scripts, so `dagger check` on a laptop still runs them.
- **Why:** in Dagger every hosted run started cold: lint took 225–282 s (2 s warm), and each test group 205–268 s. One 4-vCPU runner ran all of it for 12 min.
- **Effect:** a run with no Go change dropped from 763 s to 89–117 s.

### 2. Caches that survive between runs (#363)

- **Change:** each native group restores its own cache of Go modules, Go's build cache, Go's test cache and golangci-lint's cache. `main` saves under its commit. A pull request saves only when nothing matched, because each save costs about 12 s and the repository has 10 GB of cache in total.
- **Why:** a hosted runner starts empty. Go's caches key on file contents, so a restored cache rebuilds and reruns only what changed.

### 3. File times from file contents (#363, `scripts/date-by-content.py`)

- **Change:** after checkout, every tracked file gets a modification time computed from its git blob hash, and every directory one from its tree hash.
- **Why:** Go replays a cached test result only if the files and directories the test opened keep the same size and modification time. A checkout gives everything a new time, so tests that read the migrations or the OpenAPI spec never came from the cache.

### 4. The binary is built once, outside Docker (#363, `scripts/build-anchor.sh`)

- **Change:** `publish` compiles natively with Go's caches, and the image only packages the binary. The Dockerfile dropped `go build -a`, which recompiled everything, and builds with a cache mount through the same script. `image-builds` builds from source only for fork pull requests and when a file only the builder stage reads changes.
- **Effect:** the build step takes 3 s when nothing needs recompiling and 16 s for a typical change.

### 5. Arm runners for the backend (#366)

- **Change:** the backend matrix moved to `ubuntu-24.04-arm`. These runners are free for public repositories and compile faster.
- **Effect:** after a change to `internal/repository`, the lint step went from 83 to 55 s, the unit job from 84 to 69 s, and the run from 131 to 114 s. `image-builds` builds the amd64 image, so it stays on x64.

### 6. Go installed directly (#367, `.github/actions/install-go`)

- **Change:** Go comes from go.dev, checked against its sha256.
- **Why:** `setup-go` spent 10 of its 11–14 s copying Go into a tool cache that hosted runners never reuse.

### 7. Test parts balanced by measured time (#367, #370, #377, `scripts/go-test-times.txt`)

- **Change:** `scripts/go-test-part.sh` hands out tests longest first, each to the part with the least time so far. The times come from CI: each part prints `WORK` lines, and the header of `go-test-times.txt` shows how to regenerate the file. Parts went from 8 to 12 to 16. A part fails unless every test it selected ran.
- **Why:** taking every eighth test name gave parts of 53–112 s.

### 8. Everything compiles without optimizations (#367, #370, #372)

- **Change:** the run step sets `GOFLAGS='-gcflags=all=-N -l'` for every native job: lint, tests and the generators. `publish` still ships an optimized binary.
- **Why:** the tests wait on the database, so an unoptimized build runs them no slower and compiles about 27 % faster. Lint's export-data loading went from 44 to 27 s. In `cmd/app`, `TestGeneratedFilesAreFresh` runs the wire and config generators, and their package loads compiled optimized copies: 29.5 s in CI against 2.3 s on a laptop. As `GOFLAGS`, the setting reaches every `go` command a job runs.

### 9. The authz model is compiled without OpenFGA's parser (#370)

- **Change:** `internal/authz/model` builds the DSL and the proto from one relation tree.
- **Why:** each catalog write rendered the model as DSL and parsed it back with the ANTLR parser. That was 1.2 of 1.4 s of compile CPU, twice per write. A test checks that the proto equals what the parser makes from the DSL.

### 10. Smaller caches (#370, #377, `scripts/ci/go-cache.sh`)

- **Change:** before saving, the job deletes the module zips and the build cache entries the run did not use.
- **Why:** Go builds from the extracted modules, so the zips were 175 of a cache's 490 MB. `main`'s caches also carried every entry built in five days. Caches went from 490 MB to about 330 MB, so restores are faster and the 20 groups fit in the 10 GB budget.
- **How:** "unused" works because Go moves an entry's time to now when it uses one that is older than an hour. The job dates every entry 2000-01-01 before the run, then deletes the ones still dated 2000 after it. A `!` exclude in the cache paths was silently ignored, so the zips are deleted with `find`.

### 11. Test containers start while Go compiles (#376, `scripts/ci/start-test-containers.sh`)

- **Change:** Postgres and Redis start in the background before the compile. The suites use them through `ANCHOR_IT_POSTGRES_ADDR` and `ANCHOR_IT_REDIS_ADDR` (`cmd/it/shared/test_setup.go`) instead of starting their own.
- **Effect:** saves about 5 s per test job.

### 12. Event retries at 50 ms in the test environment (#379)

- **Change:** the product events worker uses a 50 ms backoff base when `core.environment` is `test`. Only the test suites set that value. Deployments write `prod` or `dev` and keep 1 s.
- **Why:** `TestProductEventDeliveryStatusOnConfig` waited about 27 s for six retries, which made its part the slowest.

### 13. Frontend skipped on Go-only pull requests (#384)

- **Change:** `frontend` needs `detect_changes` and runs on a pull request only when `frontend_changed` is true.
- **Why:** on a Go-only pull request the Storybook shards ran 51–69 s, the slowest jobs of the run, although no frontend file had changed.
- **Cost:** a frontend pull request waits about 8 s for `detect_changes`.

### 14. Slow tests are visible (#368)

- **Change:** each part lists its top-level tests over 5 s.
- **Why:** it shows when `go-test-times.txt` drifts and a part grows.

## Tried and dropped

Don't retry these without new evidence.

| Idea | Result |
| --- | --- |
| `-ldflags=-w -s` for test binaries | 2.5× faster link on macOS; no gain on the Linux runner |
| `go test -vet=off` | negligible |
| Lint split over several runners | no gain |
| Trimming golangci-lint's cache (#380, reverted in #382) | no size gain, lint a little slower |
| Moving oapi-codegen's output into its own package | about 1 s per job, for an 858-reference diff |
| Concurrent permission creates in the authz size tests | the server serializes them |
| Vite cache for Storybook | no gain |
| Six Storybook shards (#384) | 53–65 s jobs against 51–69 s with four: each shard spends ~25 s on setup |

## Faster options that cost money or safety

| Option | Measured | Why not today |
| --- | --- | --- |
| Larger (8-core) GitHub runners | not measured | paid; more cores barely shorten the compile chain |
| Self-hosted on the M2 devbox | in an 8-vCPU Lima VM with local caches: lint 10 s, unit 3 s, component part 15 s, service part 8 s for a typical change; 16/4/15/8 s for a repository change (managed steps: 36/21/37/27 and 52/39/49/38 s) | this repository is public, so a fork pull request would run code on a home machine; the org runner group excludes public repositories |

## Changing the workflow

- Keep `backend_patterns` and `frontend_patterns` in step with the `push` `paths:` filter. The one that disagrees loses silently.
- A new cache path or Go flag changes the cache keys. The first run after it compiles cold (about 3 min) and `main` warms the caches.
- To measure a Go change, push a temporary commit that shifts a line in a widely imported package, read the job times, and revert it before merge.
- Regenerate `scripts/go-test-times.txt` from a 16-part run when parts drift apart (the command is in its header).
