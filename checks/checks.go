// Package checks provides batteries-included health-check functions for
// common dependency and resource probes: disk headroom, memory pressure,
// HTTP endpoints, and database ping. Every function is a plain
// [health.CheckFunc]-compatible `func(ctx context.Context) error` with zero
// dependencies beyond the standard library, so it composes with
// [health.NewChecks], [health.NewWithHealthCheck], or any recorder path.
//
// Resource checks are warn-by-default: their errors grade as non-critical
// unless you also list the check name in WithCriticalServices. A full disk
// should alert, not restart the pod (see docs/batteries-ownership-decision.md).
package checks

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"syscall"
	"time"
)

// Disk returns a check that fails when fewer than minFreeBytes are available
// on the filesystem containing path. Inaccessible paths fail with the
// syscall error.
func Disk(path string, minFreeBytes uint64) func(ctx context.Context) error {
	return func(_ context.Context) error {
		var stat syscall.Statfs_t

		if err := syscall.Statfs(path, &stat); err != nil {
			return fmt.Errorf("checks: statfs %s: %w", path, err)
		}

		available := stat.Bavail * uint64(stat.Bsize)

		if available < minFreeBytes {
			return fmt.Errorf(
				"%w: %s has %d bytes available, want at least %d",
				ErrDiskLow,
				path,
				available,
				minFreeBytes,
			)
		}

		return nil
	}
}

// Memory returns a check that fails when Go heap allocation exceeds
// maxAllocBytes. It reads runtime.MemStats once per evaluation.
func Memory(maxAllocBytes uint64) func(ctx context.Context) error {
	return func(_ context.Context) error {
		var stats runtime.MemStats

		runtime.ReadMemStats(&stats)

		if stats.Alloc > maxAllocBytes {
			return fmt.Errorf(
				"%w: %d bytes allocated, want at most %d",
				ErrMemoryHigh,
				stats.Alloc,
				maxAllocBytes,
			)
		}

		return nil
	}
}

// ErrDiskLow is wrapped by the Disk check when available space is below the
// threshold. Match with errors.Is.
var ErrDiskLow = errors.New("checks: disk space low")

// ErrMemoryHigh is wrapped by the Memory check when heap allocation exceeds
// the threshold. Match with errors.Is.
var ErrMemoryHigh = errors.New("checks: memory usage high")

// ErrHTTPCheckFailed is wrapped by the HTTP check on non-2xx responses or
// failed requests. Match with errors.Is.
var ErrHTTPCheckFailed = errors.New("checks: http endpoint unhealthy")

// HTTP returns a check that GETs url with the given per-request timeout and
// fails on transport errors or non-2xx status codes. The client is created
// once at construction (no cookie jar, no redirects followed beyond the
// default policy).
func HTTP(url string, timeout time.Duration) func(ctx context.Context) error {
	client := &http.Client{Timeout: timeout}

	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrHTTPCheckFailed, url, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %s: %w", ErrHTTPCheckFailed, url, err)
		}

		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return fmt.Errorf("%w: %s: status %d", ErrHTTPCheckFailed, url, resp.StatusCode)
		}

		return nil
	}
}

// ErrDatabaseUnreachable is wrapped by the Database check when the ping
// fails. Match with errors.Is.
var ErrDatabaseUnreachable = errors.New("checks: database unreachable")

// Database returns a check that pings the database with the given timeout.
// It works with any database/sql driver; the driver import stays with the
// consumer.
func Database(db *sql.DB, pingTimeout time.Duration) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, pingTimeout)
		defer cancel()

		if err := db.PingContext(ctx); err != nil {
			return fmt.Errorf("%w: %w", ErrDatabaseUnreachable, err)
		}

		return nil
	}
}
