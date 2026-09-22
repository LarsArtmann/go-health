package health

// ResetStartupLatchForTest clears the one-way startup latch so tests can
// re-evaluate startup behavior on an existing Probe. The public API keeps the
// latch strictly one-way — this escape hatch exists only in test builds.
func (p *Probe) ResetStartupLatchForTest() {
	p.startupPassed.Store(false)
}

// SetTrackerDisabledForTest toggles the transition tracker's Since stamping
// so benchmarks can measure its marginal cost (A/B within one binary). The
// flag is never set outside test builds; production always stamps.
func (p *Probe) SetTrackerDisabledForTest(disabled bool) {
	p.transitions.disabled = disabled
}
