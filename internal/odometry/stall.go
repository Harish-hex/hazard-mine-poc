package odometry

// StallDebouncer implements the core hazard signal: motor commanded on
// (motor_pwm > PWMThreshold) but the wheel encoder shows ~no movement
// (|encoder_delta| <= EncoderEpsilon), debounced over consecutive samples so
// a single noisy sample doesn't false-trigger.
type StallDebouncer struct {
	PWMThreshold   int
	OnSamples      int
	OffSamples     int
	EncoderEpsilon int64

	IsStalled bool

	// internal counters
	stallStreak   int
	recoverStreak int
}

// Observe folds in one telemetry sample and returns the debounced stall
// state plus whether this sample is the one that caused IsStalled to
// transition from false to true (the "rising edge", useful for triggering a
// single hazard report rather than one per sample while stalled).
func (d *StallDebouncer) Observe(s TelemetrySample) (isStalled bool, roseThisTick bool) {
	candidate := s.MotorPWM > d.PWMThreshold && abs64(s.EncoderDelta) <= d.EncoderEpsilon

	if candidate {
		d.stallStreak++
		d.recoverStreak = 0
	} else {
		d.recoverStreak++
		d.stallStreak = 0
	}

	wasStalled := d.IsStalled

	if !d.IsStalled && d.stallStreak >= d.OnSamples {
		d.IsStalled = true
	} else if d.IsStalled && d.recoverStreak >= d.OffSamples {
		d.IsStalled = false
	}

	roseThisTick = !wasStalled && d.IsStalled
	return d.IsStalled, roseThisTick
}

// ConsecutiveSamples returns the current run length of consecutive
// stall-condition samples (resets to 0 on any non-stall-condition sample).
func (d *StallDebouncer) ConsecutiveSamples() int {
	return d.stallStreak
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
