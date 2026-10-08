package checks_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health/checks"
)

// FuzzBatteries fuzzes the four battery constructors over untrusted inputs
// (paths, thresholds, URL paths, status codes, DSNs). The pinned invariant is
// the error surface: Memory, HTTP, and Database return nil or exactly their
// documented sentinel — never a raw transport error — and Disk additionally
// passes through the statfs error for inaccessible paths (documented in
// Disk's contract). No input may panic a check.
//
// Signature note: the corpus in testdata/fuzz is positional. Changing any
// parameter here invalidates every saved corpus entry ("mismatched number of
// parameters") — hand-edit the corpus files in the same commit (see AGENTS.md).
func FuzzBatteries(f *testing.F) {
	f.Add("/does-not-exist", uint64(1), uint64(1), "/ok", 200, "ok")
	f.Add("", uint64(0), uint64(0), "/fail", 503, "fail")
	f.Add("/", uint64(1<<50), ^uint64(0), "/", 299, "")
	f.Add("/", ^uint64(0), ^uint64(0), "/deep/path", 200, "fail")
	f.Add("/does-not-exist", uint64(1), uint64(1), "/?x=1", 199, "ok")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The fuzzed status rides in the query so each request carries its
		// own — no shared state with the fuzz body. Only sane codes reach
		// WriteHeader; anything else gets a 200 so the server never panics
		// on fuzzer input.
		code, err := strconv.Atoi(r.URL.Query().Get("status"))
		if err != nil || code < 100 || code > 599 {
			code = http.StatusOK
		}

		w.WriteHeader(code)
	}))
	f.Cleanup(srv.Close)

	f.Fuzz(
		func(t *testing.T, path string, minFree uint64, maxAlloc uint64, httpPath string, status int, dsn string) {
			ctx := t.Context()

			assertDiskSurface(t, path, minFree, ctx)
			assertMemorySurface(t, maxAlloc, ctx)
			assertHTTPSurface(t, srv.URL+httpPath, status, httpPath, ctx)
			assertDatabaseSurface(t, dsn, ctx)
		},
	)
}

// assertDiskSurface pins Disk's contract: nil, ErrDiskLow, or the statfs
// passthrough for inaccessible paths — never any other error.
func assertDiskSurface(t *testing.T, path string, minFree uint64, ctx context.Context) {
	t.Helper()

	diskErr := checks.Disk(path, minFree)(ctx)
	if diskErr == nil {
		return
	}

	if errors.Is(diskErr, checks.ErrDiskLow) || strings.Contains(diskErr.Error(), "statfs") {
		return
	}

	t.Fatalf("Disk(%q, %d): unexpected error surface: %v", path, minFree, diskErr)
}

// assertMemorySurface pins Memory's contract: nil or exactly ErrMemoryHigh.
func assertMemorySurface(t *testing.T, maxAlloc uint64, ctx context.Context) {
	t.Helper()

	if memErr := checks.Memory(
		maxAlloc,
	)(
		ctx,
	); memErr != nil &&
		!errors.Is(memErr, checks.ErrMemoryHigh) {
		t.Fatalf("Memory(%d): unexpected error surface: %v", maxAlloc, memErr)
	}
}

// assertHTTPSurface pins HTTP's contract: nil or exactly ErrHTTPCheckFailed.
func assertHTTPSurface(
	t *testing.T,
	baseURL string,
	status int,
	httpPath string,
	ctx context.Context,
) {
	t.Helper()

	url := baseURL + "?status=" + strconv.Itoa(status)
	if httpErr := checks.HTTP(
		url,
		250*time.Millisecond,
	)(ctx); httpErr != nil &&
		!errors.Is(httpErr, checks.ErrHTTPCheckFailed) {
		t.Fatalf("HTTP(%q): unexpected error surface: %v", httpPath, httpErr)
	}
}

// assertDatabaseSurface pins Database's contract: the "fail" DSN must yield
// ErrDatabaseUnreachable; every other DSN returns nil — never another error.
func assertDatabaseSurface(t *testing.T, dsn string, ctx context.Context) {
	t.Helper()

	db := openDB(t, dsn)
	dbErr := checks.Database(db, 250*time.Millisecond)(ctx)

	switch {
	case dsn == "fail":
		if !errors.Is(dbErr, checks.ErrDatabaseUnreachable) {
			t.Fatalf("Database(%q): want ErrDatabaseUnreachable, got %v", dsn, dbErr)
		}
	case dbErr != nil:
		t.Fatalf("Database(%q): unexpected error surface: %v", dsn, dbErr)
	}
}
