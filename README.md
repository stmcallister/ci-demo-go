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

**That 82s is a laptop number, and it does not reproduce on a GitHub runner.**
Measured on `ubuntu-latest`, the same 532 MB downloads in **4–6s** — GitHub
sits on a very fat pipe to `proxy.golang.org`. Tripling the size of this
dependency graph moved the runner's download step from 4s to 6s.

Keep both figures in mind. The laptop number is what a developer feels on a
cold clone, or what a self-hosted runner on a slower link would see. The
runner number is what these workflows actually measure — and it is why the
headline saving here turns out to be **compilation**, not download. See
"Two caches" below.

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

### Two caches, one switch

Go keeps **two** separate caches:

| Cache | Path | Holds |
| --- | --- | --- |
| Module cache | `~/go/pkg/mod` | downloaded dependencies (the 532 MB above) |
| Build cache | `~/.cache/go-build` | compiled package archives |

`actions/setup-go` with `cache: true` restores **both at once**, off a single
key derived from `go.sum`. That is what these workflows use, and it is what
essentially every real Go pipeline uses. There is no separate `actions/cache`
step in this repo.

Worth saying plainly, because it is the counter-intuitive part: on a
GitHub-hosted runner most of the win is the **build cache**, not the module
cache. Dependency download is only ~4–6s uncached, because GitHub's link to
`proxy.golang.org` is very fast. Compilation is the expensive part, and that
is what `cache: true` is really buying back.

That is also why the two caches are not split apart here. Caching
`~/go/pkg/mod` alone was measured on this graph and came out to roughly a
13% delta — the ~5s spent restoring the cache almost exactly cancels the
~6s of download it avoids. Not a demo worth showing.

## The two workflows

| Workflow | File | Difference |
| --- | --- | --- |
| CI (no cache) | `.github/workflows/ci-no-cache.yml` | `cache: false` |
| CI (with cache) | `.github/workflows/ci-cached.yml` | `cache: true` |

The two files are **identical except for the workflow name and one line of
caching config**. Same runner, same Go version, same five steps in the same
order, same build and test commands. The entire diff is two hunks, and one of
them is the name:

```diff
-name: CI (no cache)
+name: CI (with cache)
@@
           go-version: '1.26'
-          cache: false
+          cache: true
```

Every step is explicitly named — **Checkout, Setup, Dependency download,
Build, Test** — and named identically in both files, so step-level timings map
one-to-one onto budget segments instead of collapsing into an "other" bucket.
`go mod download` stays its own step even though it is no longer the headline
number, because you want to *see* it be small rather than take my word for it.

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
