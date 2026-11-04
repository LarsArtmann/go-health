// Package aggregate merges multiple in-process [health.Probe] instances into
// a single health surface: one [health.Response], one set of kubelet probe
// handlers, one thing to point a dashboard or monitoring stack at.
//
// The intended topology is one process that embeds several independently
// configured probes — one per logical service or tenant — and needs to expose
// their combined state as if it were a single go-health deployment. Merging
// happens on read: every [Aggregate.CachedResponse] call performs one atomic
// [health.Probe.CachedResponse] load per source, so the aggregate adds no
// locks, no goroutines, and no staleness of its own. Freshness is bounded by
// the slowest source's refresh interval.
//
// Sources must have unique, non-empty names without "/": the name becomes the
// prefix of every check key it contributes, and a slash in it would blur the
// "source/check" grouping axis and could alias another source's namespace.
// [New] rejects such names at construction. Check names may contain "/";
// everything before the first slash in a merged key is always the source name.
//
// Draining flows through the sources: mark each source probe
// ([health.Probe.MarkShuttingDown] or [health.Probe.Shutdown]) and the merged
// readiness flips to 503 via the per-source shutdown overlay. The one drain a
// caller cannot reach that way — a federation hub draining above its remotes —
// is why the federation package carries its own mark; the aggregate
// deliberately does not.
package aggregate

import (
	"errors"
	"fmt"
	"strings"
	"time"

	health "github.com/larsartmann/go-health"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// ErrNoSources is returned by [New] when called without any sources. An
// aggregate of nothing has no meaningful state to serve.
var ErrNoSources = errors.New("aggregate: at least one source is required")

// ErrInvalidSource is returned by [New] for a source with an empty name, a
// name containing "/", a nil probe, or a name that collides with an earlier
// source. Names become check-key prefixes ("name/check"), so duplicates or
// slashes would break the documented grouping axis or silently overwrite
// check entries.
var ErrInvalidSource = errors.New("aggregate: invalid source")

// Source is one named probe contributing to an [Aggregate]. The name becomes
// the prefix of every check key the probe contributes ("name/check") and the
// key under which its startup state is reported.
type Source struct {
	// Name namespaces this source's checks as "name/check". Must be non-empty,
	// unique across all sources of an aggregate, and free of "/".
	Name string
	// Probe is the in-process probe whose cached state is merged.
	Probe *health.Probe
}

// Aggregate presents N probes as a single go-health-compatible surface. It is
// passive (no Start, no goroutines): handlers and accessors read the sources'
// atomic caches on demand. Aggregate is safe for concurrent use.
type Aggregate struct {
	sources         []Source
	refreshInterval time.Duration
}

// New creates an [Aggregate] over the given sources. Construction validates
// the invariants that reads would otherwise have to enforce silently: at
// least one source, and unique non-empty slash-free names with non-nil probes.
//
// Every invalid source is reported, not just the first: the returned error
// joins one wrapped [ErrInvalidSource] per problem, so [errors.Is] matching
// is unchanged while the message lists each offending source on its own line.
func New(sources ...Source) (*Aggregate, error) {
	if len(sources) == 0 {
		return nil, ErrNoSources
	}

	seen := make(map[string]struct{}, len(sources))
	maxInterval := time.Duration(0)

	var problems []error

	for _, src := range sources {
		if err := validateSource(src, seen); err != nil {
			problems = append(problems, err)

			continue
		}

		if d := src.Probe.RefreshInterval(); d > maxInterval {
			maxInterval = d
		}
	}

	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	return &Aggregate{sources: sources, refreshInterval: maxInterval}, nil
}

// validateSource reports the first invariant src violates as a wrapped
// [ErrInvalidSource], registering src's name as seen when it is valid.
// Invalid sources are not registered, so a later source repeating an invalid
// name fails on the same invariant instead of as a duplicate.
func validateSource(src Source, seen map[string]struct{}) error {
	switch {
	case src.Probe == nil:
		return fmt.Errorf("%w: source %q has a nil Probe", ErrInvalidSource, src.Name)
	case src.Name == "":
		return fmt.Errorf("%w: source name must not be empty", ErrInvalidSource)
	case strings.Contains(src.Name, "/"):
		return fmt.Errorf(
			"%w: source name %q must not contain '/' (names become \"name/check\" key prefixes)",
			ErrInvalidSource,
			src.Name,
		)
	}

	if _, dup := seen[src.Name]; dup {
		return fmt.Errorf("%w: duplicate source name %q", ErrInvalidSource, src.Name)
	}

	seen[src.Name] = struct{}{}

	return nil
}

// CachedResponse returns the merged health state of all sources. The overall
// status is the worst of the source statuses (fail beats warn beats pass);
// checks are namespaced "source/check"; a shutting-down source forces the
// aggregate to report ShuttingDown with overall fail, mirroring
// [health.Probe] semantics. TotalLatencyMs is the slowest source's batch.
// Version, Uptime, and InstanceID are zero: they are per-process scalars and
// do not survive a merge.
//
// The fold itself is the shared [merge.Responses] primitive; this method owns
// only the acquisition (one atomic cache load per source, FetchErr always
// empty — in-process probes are trusted) and hands the views over.
func (a *Aggregate) CachedResponse() health.Response {
	sources := make([]merge.Source, 0, len(a.sources))
	for _, src := range a.sources {
		sources = append(sources, merge.Source{
			Name:     src.Name,
			Response: src.Probe.CachedResponse(),
		})
	}

	return merge.Responses(sources)
}

// SourceStatuses folds each source's cached view to its own roll-up status:
// one map entry per source name, one atomic load per source, no evaluation.
// A source that is shutting down reports fail, mirroring its contribution to
// the merged view. The merged [Aggregate.CachedResponse] status is the worst
// of these values; this accessor answers the per-source question a dashboard
// or CLI asks next ("is source api healthy overall?") without refolding the
// namespaced check keys. It shares [Aggregate.CachedResponse]'s source loads,
// so calling both performs two passes (the loads are atomic, not network);
// docs/accessor-read-cost-design.md records the measurement and why no merged
// accessor exists. See docs/aggregate-per-source-visibility-design.md.
func (a *Aggregate) SourceStatuses() map[string]health.Status {
	statuses := make(map[string]health.Status, len(a.sources))

	for _, src := range a.sources {
		resp := src.Probe.CachedResponse()

		status := resp.Status
		if resp.ShuttingDown {
			status = health.StatusFail
		}

		statuses[src.Name] = status
	}

	return statuses
}

// RefreshInterval returns the slowest source's refresh interval: the merged
// view can never be fresher than that. Zero means at least one source
// evaluates live and no source uses a background cache.
func (a *Aggregate) RefreshInterval() time.Duration {
	return a.refreshInterval
}

// StartupComplete reports whether every source has flipped its startup latch.
// It is the AND of the sources' [health.Probe.StartupComplete] values and is
// one-way per source: once all latches are set, it stays true.
func (a *Aggregate) StartupComplete() bool {
	for _, src := range a.sources {
		if !src.Probe.StartupComplete() {
			return false
		}
	}

	return true
}
