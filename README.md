# ci-demo-go

A demo repository for a conference talk about CI pipeline performance.

It exists to generate GitHub Actions run data that shows, concretely, what
*not* caching dependencies costs — when a build is broken down into a time
budget: queue, setup, dependency install, build, test, teardown.

## What's here

The Go application is deliberately trivial: one HTTP handler (`/healthz`), a
metrics endpoint, and three tests. **The application is not the point.**

The point is the dependency graph. `go.mod` pulls a realistic set of
widely-used libraries:

| Module | Role |
| --- | --- |
| `github.com/gin-gonic/gin` | web framework |
| `github.com/stretchr/testify` | test assertions |
| `github.com/jackc/pgx/v5` | PostgreSQL driver |
| `go.uber.org/zap` | structured logging |
| `go.opentelemetry.io/otel` (+ SDK, OTLP/gRPC exporter) | tracing — drags in the gRPC and protobuf graph |
| `github.com/prometheus/client_golang` | metrics |
| `github.com/testcontainers/testcontainers-go` | integration test harness — drags in the docker/moby graph |
| `k8s.io/client-go` | Kubernetes client — drags in the k8s.io/api and apimachinery graphs |

Measured cold, with an empty module cache:

```
213 modules · 532 MB · go mod download ≈ 82s
```

That 82 seconds is the thing the talk is about.

### About the integration test

`integration_test.go` is behind a `//go:build integration` tag, so
`go test ./...` never runs it and CI never needs a Docker daemon. It's there so
testcontainers is a genuine requirement in `go.mod` — it's the single largest
contributor to cold download time. Run it deliberately if you want:

```sh
go test -tags integration ./...   # requires Docker
```

### The same trick, twice

`kube.go` is behind a `//go:build kube` tag for the same reason: it makes
`k8s.io/client-go` a genuine requirement without putting Kubernetes in the
application binary. Build tags don't affect module resolution — `go mod tidy`
resolves imports across *all* build configurations — so a tagged import still
counts against cold download time.

```sh
go build -tags kube ./...
```

### Two caches, not one

`go mod download` is only half the story. Go keeps a **module cache**
(`~/go/pkg/mod`, the 532 MB above) and a **build cache** (`~/.cache/go-build`,
compiled package archives). `actions/setup-go` with `cache: true` restores
both. Two steps in these workflows exist specifically to put weight on the
second one:

- **`go test -race`** — race instrumentation forces a full recompile of the
  entire dependency tree into its own build cache entry.
- **`golangci-lint`** — typechecks and runs SSA analysis over the whole graph.
  `.golangci.yml` deliberately leans on the analyzers that need that (gosec,
  gocritic, bodyclose, nilerr, prealloc). It also keeps its *own* cache,
  separate from Go's, which is why the workflows toggle `skip-cache` alongside
  `cache`.

## The two workflows

| Workflow | File | Difference |
| --- | --- | --- |
| CI (no cache) | `.github/workflows/ci-no-cache.yml` | `cache: false`, `skip-cache: true` |
| CI (with cache) | `.github/workflows/ci-cached.yml` | `cache: true`, `skip-cache: false` |

The two files are **identical except for the workflow name and the caching
config**. Same runner, same Go version, same steps in the same order, same
lint and test commands. The entire diff is three hunks, and the two real ones
are both booleans:

```diff
-name: CI (no cache)
+name: CI (with cache)
@@
-          cache: false
+          cache: true
@@
-          skip-cache: true
+          skip-cache: false
```

The second boolean is golangci-lint's own cache. It's a separate mechanism
from `actions/setup-go`'s, and turning off only one of them would compare a
half-cached pipeline against an uncached one. The honest version of this demo
is "cache everything" vs "cache nothing" — which is also the more useful
message: caching isn't one checkbox, it's a per-tool discipline.

Dependency download is its own step, separate from build and test, so
step-level timings map cleanly onto budget segments.

### One caveat worth knowing on stage

With `cache: true`, `actions/setup-go` restores the module cache *inside its
own step*, and saves it in a post step. So on the cached workflow the **setup**
segment gets fatter while **dependency install** collapses toward zero, and the
cache save shows up under **teardown**.

The net is still a clear win — but the time doesn't vanish, it moves. Show the
whole bar, not just the install segment, or someone in the audience will
rightly ask where it went.

## Triggering runs manually

Both workflows trigger on `push` **and** `workflow_dispatch`. The dispatch
trigger lets you build up run history without meaningless commits, and warm the
cache the morning of the talk.

```sh
# One run of each
gh workflow run ci-no-cache.yml
gh workflow run ci-cached.yml

# Watch what's happening
gh run list --limit 10
gh run watch

# Timings for a specific run
gh run view <run-id>
```

To dispatch several pairs at once:

```sh
./scripts/dispatch-runs.sh 3        # 3 runs of each, 90s apart
DELAY=120 ./scripts/dispatch-runs.sh 4
```

### Building up a meaningful sample

You want roughly **15–20 completed runs on each workflow** before a p95 figure
means anything.

**Spread these over several sessions across a few days.** Don't fire twenty
runs in one burst — they'd all reflect a single cache state, a single runner
pool, and a single moment of GitHub's queue depth. That's not a sample of
twenty, it's one measurement repeated twenty times, and the p95 will be
misleadingly tight.

A reasonable rhythm: 3–4 pairs per session, two or three sessions a day, over
two or three days.
