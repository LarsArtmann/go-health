package checks_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/larsartmann/go-health/checks"
)

// Benchmarks close the measurement gap on the batteries package (plan R13):
// until 2026-10-08 the package was tested but never benched.

func BenchmarkDisk(b *testing.B) {
	check := checks.Disk(b.TempDir(), 1<<20)

	b.ReportAllocs()
	for b.Loop() {
		if err := check(b.Context()); err != nil {
			b.Fatalf("Disk: %v", err)
		}
	}
}

func BenchmarkMemory(b *testing.B) {
	check := checks.Memory(1 << 40)

	b.ReportAllocs()
	for b.Loop() {
		if err := check(b.Context()); err != nil {
			b.Fatalf("Memory: %v", err)
		}
	}
}

func BenchmarkHTTP(b *testing.B) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	b.Cleanup(srv.Close)

	check := checks.HTTP(srv.URL, time.Second)

	b.ReportAllocs()
	for b.Loop() {
		if err := check(b.Context()); err != nil {
			b.Fatalf("HTTP: %v", err)
		}
	}
}

func BenchmarkDatabase(b *testing.B) {
	db := openDB(b, "ok")
	check := checks.Database(db, time.Second)

	b.ReportAllocs()
	for b.Loop() {
		if err := check(b.Context()); err != nil {
			b.Fatalf("Database: %v", err)
		}
	}
}
