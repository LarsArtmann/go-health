// Package federation merges remote go-health instances into a single
// health surface over HTTP: one [health.Response], one set of kubelet
// probe handlers, one thing to point a dashboard or monitoring stack at.
// It is the network sibling of [aggregate]: where aggregate merges
// in-process probes, federation pulls the full JSON document of each
// remote and namespaces its checks as "name/check".
//
// The intended topology is a hub (say, health.home.lan) that fans out to
// every service's existing go-health endpoint — a bare probe's readiness
// handler, or a go-health-dashboard route (which serves the same document
// under Accept: application/json). Remotes stay unmodified: any deployed
// go-health instance federates with zero upgrade.
//
// Like aggregate, the merge is on read: every [Prober.CachedResponse]
// performs one HTTP fetch per remote, in parallel, each bounded by a
// per-fetch timeout. There is no background loop, no scheduler, and no
// staleness of its own; the cost is one request per remote per read,
// which the hub's own read cadence (push interval, probe polling) bounds.
// Freshness is live. Scalars (Version, Uptime, InstanceID, Timestamp) do
// not survive the merge — they are per-process and would lie in a
// federated view — mirroring aggregate's rule.
//
// A remote that cannot be fetched (network error, timeout, non-200
// status, undecodable body, or a document without a status) contributes
// one synthetic "name/reachable" fail check carrying the cause, so a
// dark remote is visible on the merged surface instead of silently
// frozen at its last state. Check.StatusSince is probe-observed on the remote
// and survives the merge verbatim, keeping "since when" truthful end to
// end. See docs/federation-design.md for the full design.
//
// A hub being gracefully shut down must leave its load balancer's
// rotation before the listener closes. [Prober.MarkShuttingDown] flips a
// one-way drain state: readiness answers 503 with shutting_down set
// while the fetches keep running, so drained bodies stay fresh, and
// liveness and startup are unaffected — the same two-phase contract
// [health.Probe] already gives a single probe.
package federation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	health "github.com/larsartmann/go-health"
)

// ErrNoRemotes is returned by [New] when called without any remotes. A
// federation of nothing has no meaningful state to serve.
var ErrNoRemotes = errors.New("federation: at least one remote is required")

// ErrInvalidRemote is returned by [New] for a remote with an empty name, a
// name containing "/", a duplicate name, or a URL that is not an absolute
// http(s) URL. Names become check-key prefixes ("name/check") and URLs are
// fetched on every read, so both invariants are enforced at construction.
var ErrInvalidRemote = errors.New("federation: invalid remote")

// ErrInvalidTimeout is returned by [New] for a non-positive fetch timeout
// (see [WithTimeout]). A zero deadline would wait forever on a hung remote.
var ErrInvalidTimeout = errors.New("federation: invalid fetch timeout")

const (
	// defaultFetchTimeout is the per-fetch deadline for one remote request:
	// connect, response headers, and body read together. It matches the root
	// package's default health-check batch timeout.
	defaultFetchTimeout = 5 * time.Second
	// maxResponseBytes caps how much of a remote's body is read. A health
	// document is kilobytes; the cap bounds a misbehaving remote without
	// penalizing honest ones.
	maxResponseBytes = 1 << 20
)

// Remote is one upstream go-health instance contributing to a [Prober].
type Remote struct {
	// Name namespaces this remote's checks as "name/check". Must be
	// non-empty, unique across all remotes of a federation, and free of
	// "/". The same rules as aggregate source names apply, for the same
	// reason: the name is the grouping axis every consumer keys on.
	Name string
	// URL is the absolute http(s) URL of the remote's health document: a
	// bare probe's readiness handler, or any endpoint that answers a GET
	// with a go-health Response JSON document (a go-health-dashboard route
	// does when the request carries Accept: application/json, which
	// federation always sends).
	URL string
}

// Option configures a [Prober] at construction. Options write to a
// construction-only config, mirroring the root package's pattern.
type Option func(*config)

type config struct {
	client  *http.Client
	timeout time.Duration
}

// WithClient supplies the HTTP client used for every remote fetch. Use it
// to control transport pooling, TLS, or proxying. The per-fetch timeout
// ([WithTimeout]) is applied per request regardless of the client.
func WithClient(client *http.Client) Option {
	return func(c *config) { c.client = client }
}

// WithTimeout sets the per-fetch deadline for one remote request —
// connect, response headers, and body read together. Defaults to
// [defaultFetchTimeout] (5s, the root package's batch timeout). Must be
// positive; [New] rejects non-positive values with [ErrInvalidTimeout].
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// Prober presents N remote go-health instances as a single
// go-health-compatible surface. It satisfies the same structural surface
// the go-health-dashboard consumes as its Prober interface, so a
// federated view renders like any local probe. It is passive (no Start,
// no goroutines): reads fetch, handlers fetch, liveness does not.
// Prober is safe for concurrent use.
type Prober struct {
	// states holds one entry per remote. Fetch results carry their entry's
	// pointer (fetchedRemote), so no cross-slice length invariant exists
	// anywhere.
	states  []remoteState
	client  *http.Client
	timeout time.Duration
	// shuttingDown is the one-way drain flag set by
	// [Prober.MarkShuttingDown], overlaid onto every merged read.
	shuttingDown atomic.Bool
}

// remoteState pairs one remote with its one-way startup latch so both
// per-remote axes live in the same slice.
type remoteState struct {
	remote Remote
	// latched records whether at least one fetch has ever succeeded for
	// this remote. One-way, like the root probe's startup latch.
	latched atomic.Bool
}

// New creates a [Prober] over the given remotes. Construction validates
// the invariants reads would otherwise have to enforce silently: at least
// one remote, unique non-empty slash-free names, absolute http(s) URLs,
// and a positive fetch timeout.
func New(remotes []Remote, opts ...Option) (*Prober, error) {
	cfg := config{
		client:  &http.Client{},
		timeout: defaultFetchTimeout,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.timeout <= 0 {
		return nil, fmt.Errorf(
			"%w: %v, must be positive (e.g. WithTimeout(5 * time.Second))",
			ErrInvalidTimeout,
			cfg.timeout,
		)
	}

	if err := validateRemotes(remotes); err != nil {
		return nil, err
	}

	states := make([]remoteState, 0, len(remotes))
	for _, remote := range remotes {
		states = append(states, remoteState{remote: remote})
	}

	return &Prober{
		states:  states,
		client:  cfg.client,
		timeout: cfg.timeout,
	}, nil
}

// validateRemotes enforces the aggregate source-name contract plus URL
// sanity, with a cause naming the offending value.
func validateRemotes(remotes []Remote) error {
	if len(remotes) == 0 {
		return ErrNoRemotes
	}

	seen := make(map[string]struct{}, len(remotes))

	for _, remote := range remotes {
		switch {
		case remote.Name == "":
			return fmt.Errorf("%w: remote name must not be empty", ErrInvalidRemote)
		case strings.Contains(remote.Name, "/"):
			return fmt.Errorf(
				"%w: remote name %q must not contain '/' (names become \"name/check\" key prefixes)",
				ErrInvalidRemote,
				remote.Name,
			)
		case remote.URL == "":
			return fmt.Errorf("%w: remote %q has an empty URL", ErrInvalidRemote, remote.Name)
		}

		parsed, err := url.Parse(remote.URL)
		if err != nil {
			return fmt.Errorf("%w: remote %q URL: %w", ErrInvalidRemote, remote.Name, err)
		}

		switch {
		case parsed.Scheme != "http" && parsed.Scheme != "https":
			return fmt.Errorf(
				"%w: remote %q URL %q must be absolute with scheme http or https",
				ErrInvalidRemote,
				remote.Name,
				remote.URL,
			)
		case parsed.Host == "":
			return fmt.Errorf(
				"%w: remote %q URL %q must have a host",
				ErrInvalidRemote,
				remote.Name,
				remote.URL,
			)
		}

		if _, dup := seen[remote.Name]; dup {
			return fmt.Errorf("%w: duplicate remote name %q", ErrInvalidRemote, remote.Name)
		}

		seen[remote.Name] = struct{}{}
	}

	return nil
}

// CachedResponse returns the merged health state of all remotes: one
// parallel fetch per remote, then the merge. The overall status is the
// worst of the remote statuses and every synthetic reachable check; a
// shutting-down remote forces overall fail, mirroring [health.Probe] and
// [aggregate.Aggregate] semantics. A hub marked via
// [Prober.MarkShuttingDown] forces fail on its own merged view too, so
// programmatic readers (a dashboard) see the drain. Unreachable remotes
// contribute a "name/reachable" fail check with the cause.
// TotalLatencyMs is the slowest remote's reported batch.
func (p *Prober) CachedResponse() health.Response {
	return p.cachedResponse(context.Background())
}

// cachedResponse is CachedResponse with an explicit context: handlers pass
// the request's context so a disconnected caller cancels its fetches.
func (p *Prober) cachedResponse(ctx context.Context) health.Response {
	resp := p.merge(p.fetchAll(ctx))

	// Drain overlay, mirroring [health.Probe.CachedResponse]: the flag is
	// consulted live on every read, so a hub marked mid-flight flips to 503
	// on the next request even with every remote still healthy. The
	// StartupHandler discards this response and LivenessHandler never
	// fetches, so the overlay reaches exactly the surfaces it must.
	if p.shuttingDown.Load() {
		resp.ShuttingDown = true
		resp.Status = health.StatusFail
	}

	return resp
}

// RefreshInterval returns zero: federation fetches live and has no
// background cache or cadence of its own, and it cannot know the
// remotes' refresh intervals. Consumers set their own read cadence
// (e.g. the dashboard's WithPushInterval), which then doubles as the
// remote polling cadence.
func (p *Prober) RefreshInterval() time.Duration {
	return 0
}

// MarkShuttingDown flips the prober into its one-way drain state: from
// the call on, [Prober.CachedResponse] and [Prober.ReadinessHandler]
// report ShuttingDown with overall fail (readiness answers 503), while
// the merge-on-read fetches keep running so drained bodies stay fresh —
// the two-phase contract [health.Probe.MarkShuttingDown] gives a single
// probe. Liveness and startup are unaffected. There is no unmark and no
// [health.Probe.Shutdown] counterpart: the prober runs no background
// loop, so phase two of the drain is the host process draining its own
// HTTP server.
func (p *Prober) MarkShuttingDown() {
	p.shuttingDown.Store(true)
}

// StartupComplete reports whether every remote has answered successfully
// at least once. It is the AND of one-way per-remote latches, mirroring
// [health.Probe]'s startup latch: a remote that flaps after latching
// does not re-block startup.
func (p *Prober) StartupComplete() bool {
	for i := range p.states {
		if !p.states[i].latched.Load() {
			return false
		}
	}

	return true
}
