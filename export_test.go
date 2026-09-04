package health

// ResetStartupLatchForTest clears the one-way startup latch so tests can
// re-evaluate startup behavior on an existing Probe. The public API keeps the
// latch strictly one-way — this escape hatch exists only in test builds.
func (p *Probe) ResetStartupLatchForTest() {
	p.startupPassed.Store(false)
}
