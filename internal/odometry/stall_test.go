package odometry

import "testing"

func newTestDebouncer() *StallDebouncer {
	return &StallDebouncer{
		PWMThreshold:   50,
		OnSamples:      3,
		OffSamples:     3,
		EncoderEpsilon: 1,
	}
}

func TestStallSingleNoisySampleDoesNotTrigger(t *testing.T) {
	d := newTestDebouncer()

	// One stall-looking sample, then back to normal motion.
	stalled, rose := d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 0})
	if stalled || rose {
		t.Errorf("single sample should not trigger stall: stalled=%v rose=%v", stalled, rose)
	}
	stalled, rose = d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 10})
	if stalled || rose {
		t.Errorf("recovery sample should not be stalled: stalled=%v rose=%v", stalled, rose)
	}
}

func TestStallSustainedSamplesTriggers(t *testing.T) {
	d := newTestDebouncer()

	var lastStalled, lastRose bool
	for i := 0; i < 3; i++ {
		lastStalled, lastRose = d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 0})
	}
	if !lastStalled {
		t.Fatalf("expected IsStalled after %d sustained samples", 3)
	}
	if !lastRose {
		t.Errorf("expected roseThisTick on the sample that crosses OnSamples")
	}

	// A further stalled sample should stay stalled without another "rose" event.
	stalled, rose := d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 0})
	if !stalled {
		t.Errorf("expected still stalled")
	}
	if rose {
		t.Errorf("rose should only fire once on the transition, not every tick")
	}
}

func TestStallClearsAfterRecoveryWindow(t *testing.T) {
	d := newTestDebouncer()
	for i := 0; i < 3; i++ {
		d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 0})
	}
	if !d.IsStalled {
		t.Fatalf("expected stalled before recovery")
	}

	// Fewer than OffSamples recovery samples: still stalled.
	d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 10})
	d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 10})
	if !d.IsStalled {
		t.Errorf("expected still stalled before OffSamples reached")
	}

	// Third consecutive recovery sample clears it.
	stalled, rose := d.Observe(TelemetrySample{MotorPWM: 100, EncoderDelta: 10})
	if stalled {
		t.Errorf("expected cleared after OffSamples recovery samples")
	}
	if rose {
		t.Errorf("rose should be false on a clearing transition")
	}
}

func TestStallMotorOffNeverTriggers(t *testing.T) {
	d := newTestDebouncer()
	for i := 0; i < 5; i++ {
		stalled, _ := d.Observe(TelemetrySample{MotorPWM: 0, EncoderDelta: 0})
		if stalled {
			t.Errorf("motor off should never register as stall")
		}
	}
}
