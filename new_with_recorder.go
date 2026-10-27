package health

import "context"

// NewWithRecorder creates a [Probe] whose health-check batches come from the
// given [HealthRecorder], with no samber/do injector involved. It is the
// injector-free spelling of [New] for applications that do not use samber/do
// for dependency injection (or whose composition root deliberately keeps the
// container out of the health path):
//
//	probe := health.NewWithRecorder(myRecorder,
//	    health.WithCriticalServices(checkSQLite),
//	    health.WithVersion(version),
//	)
//
// The recorder receives a nil injector on every call. Implementations that
// ignore the parameter, the documented shape for a recorder that owns its own
// check set, are unaffected. Use [NewWithHealthCheck], [NewWithDetailedCheck],
// or [NewChecks] when a plain function or a named-check map is a better fit.
//
// A nil recorder, and no [WithHealthRecorder] option either, yields a probe
// with no checks: every batch is empty and reports pass. That is the same
// contract as [New] over an empty container, and it is almost always a wiring
// mistake, so pass the recorder explicitly.
//
// The option supplied recorder is used when the argument is nil, so the two
// spellings compose:
//
//	health.NewWithRecorder(nil, health.WithHealthRecorder(r))
func NewWithRecorder(recorder HealthRecorder, opts ...Option) *Probe {
	cfg := buildConfig(opts)

	if recorder == nil {
		recorder = cfg.recorder
	}

	if recorder == nil {
		return assemble(emptyHealthCheck, cfg)
	}

	return assemble(resolveHealthCheck(recorder, nil), cfg)
}

// emptyHealthCheck is the batch capability of a probe with no check source:
// an empty result set, which the classifier reports as pass.
func emptyHealthCheck(context.Context) map[string]CheckDetail {
	return map[string]CheckDetail{}
}
