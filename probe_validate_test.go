package health_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// --- Validate tests ---.

func TestValidate_DefaultConfig_IsValid(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New())

	if err := probe.Validate(); err != nil {
		t.Errorf("default config: want nil error, got %v", err)
	}
}

func TestValidate_ZeroTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithTimeout(0))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidTimeout) {
		t.Errorf("zero timeout: want ErrInvalidTimeout, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "0s") || !strings.Contains(msg, "WithTimeout") {
		t.Errorf(
			"zero timeout: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_NegativeTimeout_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithTimeout(-1*time.Second))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidTimeout) {
		t.Errorf("negative timeout: want ErrInvalidTimeout, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "-1s") || !strings.Contains(msg, "WithTimeout") {
		t.Errorf(
			"negative timeout: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_NegativeRefreshInterval_ReturnsError(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(-1))

	err := probe.Validate()
	if !errors.Is(err, health.ErrInvalidRefreshInterval) {
		t.Errorf("negative refresh interval: want ErrInvalidRefreshInterval, got %v", err)
	}

	if msg := err.Error(); !strings.Contains(msg, "-1ns") ||
		!strings.Contains(msg, "WithRefreshInterval") {
		t.Errorf(
			"negative refresh interval: error should include the offending value and remediation, got %q",
			msg,
		)
	}
}

func TestValidate_ZeroRefreshInterval_IsValid(t *testing.T) {
	t.Parallel()

	probe := health.New(do.New(), health.WithRefreshInterval(0))

	if err := probe.Validate(); err != nil {
		t.Errorf("zero refresh interval (live mode): want nil error, got %v", err)
	}
}
