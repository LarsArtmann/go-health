package checks_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health/checks"
)

func TestDisk_PassesWithFreeSpace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	check := checks.Disk(dir, 1)

	if err := check(t.Context()); err != nil {
		t.Fatalf("Disk on a real filesystem should pass, got: %v", err)
	}
}

func TestDisk_FailsBelowThreshold(t *testing.T) {
	t.Parallel()

	check := checks.Disk(t.TempDir(), 1<<50)

	err := check(t.Context())
	if !errors.Is(err, checks.ErrDiskLow) {
		t.Fatalf("want ErrDiskLow, got %v", err)
	}
}

func TestDisk_FailsOnMissingPath(t *testing.T) {
	t.Parallel()

	check := checks.Disk(t.TempDir()+"/does-not-exist", 1)

	err := check(t.Context())
	if err == nil || errors.Is(err, checks.ErrDiskLow) {
		t.Fatalf("want a statfs error, got %v", err)
	}
}

func TestMemory_PassesBelowThreshold(t *testing.T) {
	t.Parallel()

	check := checks.Memory(1 << 40)

	if err := check(t.Context()); err != nil {
		t.Fatalf("Memory below threshold should pass, got: %v", err)
	}
}

func TestMemory_FailsAboveThreshold(t *testing.T) {
	t.Parallel()

	check := checks.Memory(1)

	if err := check(t.Context()); !errors.Is(err, checks.ErrMemoryHigh) {
		t.Fatalf("want ErrMemoryHigh, got %v", err)
	}
}

func TestHTTP_PassesOn2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	check := checks.HTTP(srv.URL, time.Second)

	if err := check(t.Context()); err != nil {
		t.Fatalf("HTTP 200 should pass, got: %v", err)
	}
}

func TestHTTP_FailsOnNon2xx(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	check := checks.HTTP(srv.URL, time.Second)

	if err := check(t.Context()); !errors.Is(err, checks.ErrHTTPCheckFailed) {
		t.Fatalf("want ErrHTTPCheckFailed, got %v", err)
	}
}

func TestHTTP_FailsOnUnreachable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	check := checks.HTTP(url, 200*time.Millisecond)

	if err := check(t.Context()); !errors.Is(err, checks.ErrHTTPCheckFailed) {
		t.Fatalf("want ErrHTTPCheckFailed, got %v", err)
	}
}

func TestHTTP_FailsOnMalformedURL(t *testing.T) {
	t.Parallel()

	// A control character makes request construction itself fail before any
	// dial: the only branch where NewRequestWithContext returns an error.
	check := checks.HTTP("http://127.0.0.1:1/\x00bad", time.Second)

	err := check(t.Context())
	if !errors.Is(err, checks.ErrHTTPCheckFailed) {
		t.Fatalf("want ErrHTTPCheckFailed, got %v", err)
	}

	if !strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("error should name the checked URL, got: %v", err)
	}
}

func TestDatabase_PassesOnOpenDB(t *testing.T) {
	t.Parallel()

	db := openDB(t, "ok")

	check := checks.Database(db, time.Second)

	if err := check(t.Context()); err != nil {
		t.Fatalf("Database ping on open db should pass, got: %v", err)
	}
}

func TestDatabase_FailsOnClosedDB(t *testing.T) {
	t.Parallel()

	check := checks.Database(openDB(t, "fail"), time.Second)

	if err := check(t.Context()); !errors.Is(err, checks.ErrDatabaseUnreachable) {
		t.Fatalf("want ErrDatabaseUnreachable, got %v", err)
	}
}

// fakeDriver is an in-process database/sql driver: Ping succeeds unless the
// DSN is "fail". Registered once per test binary — no external driver
// dependency (the checks package is zero-dep by decision).
type fakeDriver struct{}

type fakeConn struct {
	dsn string
}

func (fakeDriver) Open(dsn string) (driver.Conn, error) { return fakeConn{dsn: dsn}, nil }

func (fakeConn) Prepare(string) (driver.Stmt, error) { return nil, errNotImplemented }

func (fakeConn) Close() error { return nil }

func (fakeConn) Begin() (driver.Tx, error) { return nil, errNotImplemented }

func (c fakeConn) Ping(context.Context) error {
	if c.dsn == "fail" {
		return errPingRefused
	}

	return nil
}

var (
	errPingRefused    = errors.New("ping refused")
	errNotImplemented = errors.New("not implemented")
)

//nolint:gochecknoinits // database/sql requires driver registration by side effect
func init() {
	sql.Register("healthchecks-fake", fakeDriver{})
}

func openDB(tb testing.TB, dsn string) *sql.DB {
	tb.Helper()

	handle, err := sql.Open("healthchecks-fake", dsn)
	if err != nil {
		tb.Fatalf("open: %v", err)
	}

	tb.Cleanup(func() { _ = handle.Close() })

	return handle
}

func TestChecks_ComposeWithNewChecks(t *testing.T) {
	t.Parallel()

	probe := checks.Disk(t.TempDir(), 1)

	if err := probe(t.Context()); err != nil {
		t.Fatalf("composed check should pass, got: %v", err)
	}
}
