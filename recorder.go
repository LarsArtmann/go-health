package health

import (
	"context"
	"maps"

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
// The returned recorder composes with both recorder-capable constructors:
// [New] (the injector path) and [NewWithRecorder] (injector-free). The three
// function-path constructors — [NewWithHealthCheck], [NewWithDetailedCheck],
// and [NewChecks] — reject [WithHealthRecorder] at [Probe.Validate] with
// [ErrRecorderNotApplicable].
//
// A chain of only nils records nothing: its batches are empty, which the
// probe reports as an all-pass response. That mirrors a recorder returning
// no checks, and is almost always a wiring mistake — prefer letting the
// probe run the injector directly (no recorder option) in that case.
//
//nolint:ireturn // returning the composed recorder interface is the point
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
		maps.Copy(merged, recorder.RecordHealthCheckWithContext(ctx, injector))
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
			maps.Copy(merged, detailed.RecordDetailedHealthCheckWithContext(ctx, injector))

			continue
		}

		for name, err := range recorder.RecordHealthCheckWithContext(ctx, injector) {
			merged[name] = CheckDetail{Err: err}
		}
	}

	return merged
}

// HealthRecorderFunc adapts a plain function to [HealthRecorder], so a
// one-off check batch needs no named struct. It is the recorder-side analog
// of [http.HandlerFunc]:
//
//	probe := health.NewWithRecorder(health.HealthRecorderFunc(
//	    func(ctx context.Context, _ do.Injector) map[string]error {
//	        return map[string]error{"sqlite": db.PingContext(ctx)}
//	    },
//	))
//
// A nil function panics when the first batch runs.
type HealthRecorderFunc func(ctx context.Context, injector do.Injector) map[string]error

// RecordHealthCheckWithContext implements [HealthRecorder] by calling f.
func (f HealthRecorderFunc) RecordHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]error {
	return f(ctx, injector)
}

// DetailedHealthRecorderFunc adapts a metadata-rich function to both
// [HealthRecorder] and [DetailedHealthRecorder], so per-check durations reach
// the wire without a named struct. It is the detailed sibling of
// [HealthRecorderFunc]:
//
//	probe := health.NewWithRecorder(health.DetailedHealthRecorderFunc(
//	    func(ctx context.Context, _ do.Injector) map[string]health.CheckDetail {
//	        start := time.Now()
//	        err := ping(ctx)
//
//	        return map[string]health.CheckDetail{
//	            "sqlite": {Err: err, Duration: time.Since(start)},
//	        }
//	    },
//	))
//
// The plain [HealthRecorder] view projects each detail to its error, dropping
// durations. A nil function panics when the first batch runs.
type DetailedHealthRecorderFunc func(
	ctx context.Context,
	injector do.Injector,
) map[string]CheckDetail

// RecordHealthCheckWithContext implements [HealthRecorder] by projecting the
// detailed batch to its plain error view.
func (f DetailedHealthRecorderFunc) RecordHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]error {
	return errorsOf(f(ctx, injector))
}

// RecordDetailedHealthCheckWithContext implements [DetailedHealthRecorder] by
// calling f.
func (f DetailedHealthRecorderFunc) RecordDetailedHealthCheckWithContext(
	ctx context.Context,
	injector do.Injector,
) map[string]CheckDetail {
	return f(ctx, injector)
}
