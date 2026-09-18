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
// frozen at its last state. Check.Since is probe-observed on the remote
// and survives the merge verbatim, keeping "since when" truthful end to
// end. See docs/federation-design.md for the full design.
package federation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	remotes []Remote
	client  *http.Client
	timeout time.Duration
	// startup records, per remote, whether at least one fetch has ever
	// succeeded. One-way latches, like the root probe's startup latch.
	startup []atomic.Bool
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

	return &Prober{
		remotes: remotes,
		client:  cfg.client,
		timeout: cfg.timeout,
		startup: make([]atomic.Bool, len(remotes)),
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

// fetchResult is the outcome of one remote fetch: either a decoded
// go-health response (ok) or the failure cause (ok false).
type fetchResult struct {
	ok     bool
	resp   health.Response
	errMsg string
}

// CachedResponse returns the merged health state of all remotes: one
// parallel fetch per remote, then the merge. The overall status is the
// worst of the remote statuses and every synthetic reachable check; a
// shutting-down remote forces overall fail, mirroring [health.Probe] and
// [aggregate.Aggregate] semantics. Unreachable remotes contribute a
// "name/reachable" fail check with the cause. TotalLatencyMs is the
// slowest remote's reported batch.
func (p *Prober) CachedResponse() health.Response {
	return p.cachedResponse(context.Background())
}

// cachedResponse is CachedResponse with an explicit context: handlers pass
// the request's context so a disconnected caller cancels its fetches.
func (p *Prober) cachedResponse(ctx context.Context) health.Response {
	//nolint:makezero // pre-sized slice: parallel goroutines write results[i] by index
	results := make([]fetchResult, len(p.remotes))

	var wg sync.WaitGroup

	for i, remote := range p.remotes {
		wg.Go(func() {
			results[i] = p.fetch(ctx, remote)
		})
	}

	wg.Wait()

	return p.merge(results)
}

// merge combines fetch results into one response and advances the
// startup latches. Runs single-threaded after the parallel fetches.
func (p *Prober) merge(results []fetchResult) health.Response {
	checks := make(map[string]health.Check)
	status := health.StatusPass
	shuttingDown := false

	var maxLatency int64

	for i, remote := range p.remotes {
		result := results[i]

		if !result.ok {
			checks[remote.Name+"/reachable"] = health.Check{
				Status: health.StatusFail,
				Error:  "fetch: " + result.errMsg,
			}
			status = worst(status, health.StatusFail)

			continue
		}

		if !p.startup[i].Load() {
			p.startup[i].Store(true)
		}

		if result.resp.ShuttingDown {
			shuttingDown = true
		}

		if result.resp.TotalLatencyMs > maxLatency {
			maxLatency = result.resp.TotalLatencyMs
		}

		status = worst(status, result.resp.Status)

		for name, check := range result.resp.Checks {
			checks[remote.Name+"/"+name] = check
		}
	}

	if shuttingDown {
		status = health.StatusFail
	}

	return health.Response{
		Status:         status,
		ShuttingDown:   shuttingDown,
		Checks:         checks,
		TotalLatencyMs: maxLatency,
	}
}

// worst returns the more severe of two statuses by [health.Status.Rank].
func worst(a, b health.Status) health.Status {
	if a.Rank() < b.Rank() {
		return a
	}

	return b
}

// fetch retrieves and decodes one remote's health document. Every failure
// mode — request construction, transport, non-200, oversized or
// undecodable body, missing status — returns a non-ok result carrying a
// human-readable cause; none of them panic or retry.
func (p *Prober) fetch(ctx context.Context, remote Remote) fetchResult {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remote.URL, nil)
	if err != nil {
		return fetchResult{errMsg: fmt.Sprintf("build request: %v", err)}
	}

	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fetchResult{errMsg: err.Error()}
	}

	defer func() { _ = resp.Body.Close() }() //nolint:erraudit // closing an HTTP response body has no useful error to act on

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy( //nolint:erraudit // best-effort drain so the connection can be reused
			io.Discard,
			io.LimitReader(resp.Body, maxResponseBytes),
		)

		return fetchResult{errMsg: "unexpected HTTP status " + resp.Status}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fetchResult{errMsg: fmt.Sprintf("read body: %v", err)}
	}

	return decodeDocument(body)
}

// decodeDocument validates the wire shape beyond JSON syntax: a document
// without a status, or with a check status that is not pass/warn/fail, is
// refused whole. Unlike aggregate, which reads trusted in-process probes,
// federation decodes untrusted wire input — rendering an unknown status
// would let it masquerade as healthy on the merged surface.
func decodeDocument(body []byte) fetchResult {
	var parsed health.Response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fetchResult{errMsg: fmt.Sprintf("decode response: %v", err)}
	}

	if parsed.Status == "" {
		return fetchResult{errMsg: "response has no status — is this a go-health endpoint?"}
	}

	for name, check := range parsed.Checks {
		switch check.Status {
		case health.StatusPass, health.StatusWarn, health.StatusFail:
		default:
			return fetchResult{
				errMsg: fmt.Sprintf("response check %q has invalid status %q", name, check.Status),
			}
		}
	}

	return fetchResult{ok: true, resp: parsed}
}

// RefreshInterval returns zero: federation fetches live and has no
// background cache or cadence of its own, and it cannot know the
// remotes' refresh intervals. Consumers set their own read cadence
// (e.g. the dashboard's WithPushInterval), which then doubles as the
// remote polling cadence.
func (p *Prober) RefreshInterval() time.Duration {
	return 0
}

// StartupComplete reports whether every remote has answered successfully
// at least once. It is the AND of one-way per-remote latches, mirroring
// [health.Probe]'s startup latch: a remote that flaps after latching
// does not re-block startup.
func (p *Prober) StartupComplete() bool {
	for i := range p.startup {
		if !p.startup[i].Load() {
			return false
		}
	}

	return true
}

// LivenessHandler answers the federation's liveness question: "is this
// process alive?" Liveness performs zero remote fetches by design — the
// hub's process being alive says nothing about its remotes, and
// restarting the hub over a remote blip would cause a restart cascade.
// Always 200 with an empty checks map, mirroring [health.Probe].
func (p *Prober) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeResponse(w, http.StatusOK, health.Response{
			Status: health.StatusPass,
			Checks: map[string]health.Check{},
		})
	}
}

// ReadinessHandler answers the federation's readiness question: "can
// this hub serve traffic?" Serves the merged view: 503 when the overall
// status is fail (which includes any unreachable or shutting-down
// remote), 200 otherwise. A dark remote takes the hub out of rotation —
// deliberate: the hub's whole answer is its remotes.
func (p *Prober) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := p.cachedResponse(r.Context())

		code := http.StatusOK
		if resp.Status == health.StatusFail {
			code = http.StatusServiceUnavailable
		}

		writeResponse(w, code, resp)
	}
}

// StartupHandler answers the federation's startup question: "has every
// remote answered successfully at least once?" The fetch itself moves
// the latches, so repeated kubelet polls make progress. Returns 503
// with one failing check per not-yet-latched remote until all latches
// are set, then 200 with an empty checks map.
func (p *Prober) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_ = p.cachedResponse(r.Context())

		if p.StartupComplete() {
			writeResponse(w, http.StatusOK, health.Response{
				Status: health.StatusPass,
				Checks: map[string]health.Check{},
			})

			return
		}

		checks := make(map[string]health.Check)

		for i, remote := range p.remotes {
			if !p.startup[i].Load() {
				checks[remote.Name] = health.Check{
					Status: health.StatusFail,
					Error:  "no successful fetch yet",
				}
			}
		}

		writeResponse(w, http.StatusServiceUnavailable, health.Response{
			Status: health.StatusFail,
			Checks: checks,
		})
	}
}

// RegisterRoutes registers all three federation probe handlers on the
// given mux using the provided routes. Pass [health.DefaultRoutes] for
// the conventional Kubernetes paths.
func (p *Prober) RegisterRoutes(mux *http.ServeMux, routes health.Routes) {
	mux.HandleFunc(routes.Liveness, p.LivenessHandler())
	mux.HandleFunc(routes.Readiness, p.ReadinessHandler())
	mux.HandleFunc(routes.Startup, p.StartupHandler())
}

// marshalResponse is the single serialization seam for federation
// responses. It is a package variable only so tests can force the
// marshal-error branch; production code must never swap it.
//
//nolint:gochecknoglobals // deliberate test seam; mirrors the aggregate's seam
var marshalResponse = func(resp health.Response) ([]byte, error) {
	return json.Marshal(health.SanitizeResponse(resp), json.Deterministic(true))
}

// writeResponse serialises the health response as JSON with the given
// status code. Same wire format as the root package's handlers:
// deterministic key order, no caching.
func writeResponse(w http.ResponseWriter, code int, resp health.Response) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	payload, err := marshalResponse(resp)
	if err != nil {
		// Defensive: Response only contains basic types so json.Marshal
		// cannot fail today. Mirrors the root package's guard, including
		// the underlying cause in the body.
		http.Error(
			w,
			"federation: failed to encode response: "+err.Error(),
			http.StatusInternalServerError,
		)

		return
	}

	w.WriteHeader(code)
	_, _ = w.Write(
		payload,
	) //nolint:erraudit // intentional: status already committed; a library must not log client disconnects
}
