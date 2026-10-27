package health

import (
	"context"

	"github.com/samber/do/v2"
)

// ChainRecorders composes several [HealthRecorder]s into one, in order:
// every batch runs each recorder and the results are merged, with later
// recorders winning name collisions. Nil recorders are skipped.
//
// The chain always satisfies [DetailedHealthRecorder], so per-check
// durations survive when the composing recorder reports them: detailed
// members contribute their [CheckDetail] verbatim, plain members contribute
// their error with zero duration (unknown, omitted from JSON). This is the
// supported way to observe one batch through several sinks — an audit log
// plus a flight recorder, projection health plus injector services —
// without re-implementing the merge in every consumer bridge.
//
// The returned recorder is only usable with [New] (the injector path); a
// function-path constructor rejects [WithHealthRecorder] at
// [Probe.Validate] with [ErrRecorderNotApplicable].
//
// A chain of only nils records nothing: its batches are empty, which the
// probe reports as an all-pass response. That mirrors a recorder returning
// no checks, and is almost always a wiring mistake — prefer letting the
// probe run the injector directly (no recorder option) in that case.
func ChainRecorders(recorders ...HealthRecorder) HealthRecorder {
	chain := make([]HealthRecorder, 0, len(recorders))

	for _, recorder := range recorders {
		if recorder != nil {
			chain = append(chain, recorder)
		}
	}

	return recorderChain{recorders: chain}
}

// recorderChain is the dynamic type behind [ChainRecorders]. It implements
// both recorder interfaces so the probe's construction-time type switch
// ([resolveHealthCheck]) always selects the detailed path; plain members are
// adapted there with zero Duration.
type recorderChain struct {
	recorders []HealthRecorder
}

// RecordHealthCheckWithContext merges each recorder's plain batch in order.
func (c recorderChain) RecordHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]error {
	merged := make(map[string]error)

	for _, recorder := range c.recorders {
		for name, err := range recorder.RecordHealthCheckWithContext(ctx, injector) {
			merged[name] = err
		}
	}

	return merged
}

// RecordDetailedHealthCheckWithContext merges each recorder's detailed batch
// in order, preserving durations from members that report them.
func (c recorderChain) RecordDetailedHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]CheckDetail {
	merged := make(map[string]CheckDetail)

	for _, recorder := range c.recorders {
		if detailed, ok := recorder.(DetailedHealthRecorder); ok {
			for name, detail := range detailed.RecordDetailedHealthCheckWithContext(ctx, injector) {
				merged[name] = detail
			}

			continue
		}

		for name, err := range recorder.RecordHealthCheckWithContext(ctx, injector) {
			merged[name] = CheckDetail{Err: err}
		}
	}

	return merged
}
