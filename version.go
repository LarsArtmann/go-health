package health

import (
	"encoding/json/v2"
	"net/http"
	"strings"
)

// versionResponse is the wire body of [VersionHandler]: the binary's build
// version and nothing else. The field is always present (no omitempty) — an
// unstamped binary answers {"version":""}, which is the truth about the
// build, not a missing field.
type versionResponse struct {
	Version string `json:"version"`
}

// VersionHandler returns an http.HandlerFunc that reports the binary's build
// version — the ldflags/VCS stamp (e.g. built with
// -ldflags "-X main.buildVersion=$(git describe --tags)") or whatever
// identity string the build pipeline produces. It answers "which build am I
// talking to?", a question orthogonal to health: the endpoint never
// evaluates checks, never returns 503, and is unaffected by shutdown.
//
// The wire shape is {"version":"..."} — the same field name and semantics as
// Response.Version ([WithVersion]), served separately so build identity also
// works for deployments that do not route the probe handlers at all. Wire it
// next to your health routes:
//
//	mux.HandleFunc("/version", health.VersionHandler(buildVersion))
//
// The handler accepts GET only; any other method gets 405 with an Allow
// header — the same posture [WithAllowedMethods] offers for the probe
// endpoints, enforced unconditionally here because a version endpoint is
// read-only metadata and there is no configuration worth having. Surfacing
// method misuse early beats silently 200-ing a stray POST.
//
// The payload is marshaled once at construction: the version is immutable
// for the process lifetime, so per-request work is header writes and one
// slice copy.
func VersionHandler(version string) http.HandlerFunc {
	body := versionResponse{Version: strings.ToValidUTF8(version, "\uFFFD")}

	payload, err := json.Marshal(body, json.Deterministic(true))
	if err != nil {
		// Defensive: a single UTF-8-coerced string field cannot fail to
		// marshal today. If a future field reintroduces a failure mode,
		// the endpoint must say so loudly instead of writing nothing
		// after a 200 — same stance as writeResponse.
		return func(w http.ResponseWriter, _ *http.Request) {
			http.Error(
				w,
				"health: failed to encode version: "+err.Error(),
				http.StatusInternalServerError,
			)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "version endpoint only accepts GET", http.StatusMethodNotAllowed)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		// The status line is already committed so a write failure (client
		// disconnect, broken pipe) is genuinely unrecoverable. Silently
		// swallow — a library must not make logging decisions for the host.
		_, _ = w.Write(
			payload,
		) //nolint:erraudit // intentional: status already committed; a library must not log client disconnects
	}
}
