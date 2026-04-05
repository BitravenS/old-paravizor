package ratelimit

// Overdrive is a convenience wrapper that enables/disables overdrive on the pool.
// In overdrive mode all token checks are bypassed and tools dispatch as fast as possible.
type Overdrive struct {
	pool *CreditPool
}

// NewOverdrive creates an overdrive controller for the given pool.
func NewOverdrive(pool *CreditPool) *Overdrive {
	return &Overdrive{pool: pool}
}

// Enable activates overdrive mode.
func (o *Overdrive) Enable() {
	o.pool.EnableOverdrive()
}

// Disable deactivates overdrive mode and restores normal limits.
func (o *Overdrive) Disable() {
	o.pool.DisableOverdrive()
}

// IsActive returns whether overdrive is currently active.
func (o *Overdrive) IsActive() bool {
	return o.pool.IsOverdrive()
}
