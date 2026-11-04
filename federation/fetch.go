package federation

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"sync"

	health "github.com/larsartmann/go-health"
	merge "github.com/larsartmann/go-health/internal/merge"
)

// fetchResult is the outcome of one remote fetch: either a decoded
// go-health response (ok) or the failure cause (ok false).
type fetchResult struct {
	ok     bool
	resp   health.Response
	errMsg string
}

// fetchedRemote pairs a remote's state with its fetch outcome so the
// merge loop never relies on two slices staying index-aligned: the
// pairing is carried in the data.
type fetchedRemote struct {
	state  *remoteState
	result fetchResult
}

// SourceStatuses folds every remote's own document to its roll-up status:
// one entry per remote name, from the same one-parallel-fetch-per-remote
// pass [Prober.CachedResponse] performs (so calling both costs two passes;
// a caller that needs the merged document should read only that, and fold
// per-remote verdicts from its namespaced keys). It answers the per-remote
// question a hub dashboard
// asks next — "is remote api healthy overall?" — without refolding
// "api/*" check keys.
//
// An unreachable or undecodable remote reports [health.StatusFail] for
// itself, mirroring its synthetic "name/reachable" row; a remote whose
// document carries shutting_down also reports fail, so a draining remote's
// card cannot look greener than the page. The hub's own drain
// ([Prober.MarkShuttingDown]) does not rewrite remote statuses — it is a
// property of this hub, not of any remote.
//
// Every reported value is one of pass/warn/fail/off: a document whose own
// roll-up is outside that set is refused at decode, so this map can never
// carry a status the dashboard would rank as healthy without knowing it.
//
// This accessor mirrors [aggregate.Aggregate.SourceStatuses], so one
// capability interface covers both merge surfaces. See
// docs/federation-source-statuses-design.md.
func (p *Prober) SourceStatuses() map[string]health.Status {
	statuses := make(map[string]health.Status, len(p.states))

	for _, fetched := range p.fetchAll(context.Background()) {
		name := fetched.state.remote.Name
		if !fetched.result.ok {
			statuses[name] = health.StatusFail

			continue
		}

		status := fetched.result.resp.Status
		if fetched.result.resp.ShuttingDown {
			status = health.StatusFail
		}

		statuses[name] = status
	}

	return statuses
}

// fetchAll performs one parallel fetch per remote. Results carry their
// remote's state pointer, so callers never rely on two slices staying
// index-aligned. Latch movement and merging stay with the callers: reads
// that only need per-remote verdicts must not advance startup latches.
func (p *Prober) fetchAll(ctx context.Context) []fetchedRemote {
	//nolint:makezero // pre-sized slice: parallel goroutines write results[i] by index
	results := make([]fetchedRemote, len(p.states))

	var wg sync.WaitGroup

	for i := range p.states {
		state := &p.states[i]

		wg.Go(func() {
			results[i] = fetchedRemote{state: state, result: p.fetch(ctx, state.remote)}
		})
	}

	wg.Wait()

	return results
}

// merge combines fetch results into one response and advances the
// startup latches. Runs single-threaded after the parallel fetches. The
// fold itself is the shared [merge.Responses] primitive; this method owns
// the federation-specific pieces: latch movement is an output of a
// successful fetch (not a merge input), and a failed fetch maps to the
// primitive's FetchErr channel so the dark remote surfaces as its synthetic
// reachable row.
func (p *Prober) merge(results []fetchedRemote) health.Response {
	sources := make([]merge.Source, 0, len(results))

	for _, fetched := range results {
		state, result := fetched.state, fetched.result

		if !result.ok {
			sources = append(
				sources,
				merge.Source{Name: state.remote.Name, FetchErr: result.errMsg},
			)

			continue
		}

		state.latched.Store(true)
		sources = append(sources, merge.Source{Name: state.remote.Name, Response: result.resp})
	}

	return merge.Responses(sources)
}

// fetch retrieves and decodes one remote's health document. Every failure
// mode — request construction, transport, non-200, oversized or
// undecodable body, missing status — returns a non-ok result carrying a
// human-readable cause; none of them panic or retry.
//
// The request-construction branch is unreachable for any configuration [New]
// accepted: validation runs the same net/url parse that
// http.NewRequestWithContext runs again here, so a URL that got through cannot
// fail now. It stays because the constructor returns an error and dropping the
// check would silently discard it, which matters the day validation is
// relaxed. That is the one statement in this package the coverage gate reads as
// uncovered; every other branch is pinned by test (transport, non-200 drain,
// truncated body, oversized and undecodable documents).
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
// without a status, with a roll-up status or a check status that is not
// pass/warn/fail/off, is refused whole. Unlike aggregate, which reads trusted
// in-process probes, federation decodes untrusted wire input — rendering an
// unknown status would let it masquerade as healthy on the merged surface.
// Off is accepted because it is a known status with pass-tier
// [health.Status.Rank]: it can never fail or warn the merged roll-up, only
// annotate a row.
func decodeDocument(body []byte) fetchResult {
	var parsed health.Response
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fetchResult{errMsg: fmt.Sprintf("decode response: %v", err)}
	}

	if parsed.Status == "" {
		return fetchResult{errMsg: "response has no status — is this a go-health endpoint?"}
	}

	// The roll-up status needs the same guard as a check status. The merge
	// recomputes the verdict from the checks, so an unknown roll-up cannot
	// reach the merged surface — but SourceStatuses() reports the document's
	// own claim verbatim, and an unrecognized value there ranks pass-tier in
	// Status.Rank and would render as a healthy source card. Refusing the
	// document keeps one rule for both levels instead of trusting the field
	// nobody validated.
	switch parsed.Status {
	case health.StatusPass, health.StatusWarn, health.StatusFail, health.StatusOff:
	default:
		return fetchResult{
			errMsg: fmt.Sprintf("response has invalid status %q", parsed.Status),
		}
	}

	for name, check := range parsed.Checks {
		switch check.Status {
		case health.StatusPass, health.StatusWarn, health.StatusFail, health.StatusOff:
		default:
			return fetchResult{
				errMsg: fmt.Sprintf("response check %q has invalid status %q", name, check.Status),
			}
		}
	}

	return fetchResult{ok: true, resp: parsed}
}
