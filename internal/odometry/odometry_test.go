package odometry

import "testing"

func TestIntegratorDistance(t *testing.T) {
	o := NewIntegrator(Config{MetersPerTick: 0.01})

	samples := []TelemetrySample{
		{EncoderDelta: 10, GyroZ: 0, TS: 1000},
		{EncoderDelta: 20, GyroZ: 0, TS: 1100},
		{EncoderDelta: 5, GyroZ: 0, TS: 1200},
	}
	for _, s := range samples {
		o.Update(s)
	}

	got := o.Snapshot().CumulativeDistanceM
	want := 0.35 // (10+20+5)*0.01
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("CumulativeDistanceM = %v, want %v", got, want)
	}
}

func TestIntegratorHeadingIndependentOfDistance(t *testing.T) {
	o := NewIntegrator(Config{MetersPerTick: 0.01})

	// First sample only primes LastTS; no heading delta yet.
	o.Update(TelemetrySample{EncoderDelta: 0, GyroZ: 10, TS: 1000})
	if got := o.Snapshot().HeadingDeg; got != 0 {
		t.Errorf("HeadingDeg after first sample = %v, want 0", got)
	}

	// gyro_z = 10 deg/s over 1s => +10 deg.
	o.Update(TelemetrySample{EncoderDelta: 0, GyroZ: 10, TS: 2000})
	got := o.Snapshot().HeadingDeg
	if got != 10 {
		t.Errorf("HeadingDeg = %v, want 10", got)
	}

	// Distance should remain 0 even though heading changed (independent scalars).
	if d := o.Snapshot().CumulativeDistanceM; d != 0 {
		t.Errorf("CumulativeDistanceM = %v, want 0", d)
	}
}

func TestIntegratorHeadingWraps(t *testing.T) {
	o := NewIntegrator(Config{MetersPerTick: 0.01})
	o.Update(TelemetrySample{GyroZ: 0, TS: 0})
	// 350 deg/s over 1s => +350; then again => +350 => 700 => wraps to 340.
	o.Update(TelemetrySample{GyroZ: 350, TS: 1000})
	got := o.Snapshot().HeadingDeg
	if got != 350 {
		t.Errorf("HeadingDeg = %v, want 350", got)
	}
	o.Update(TelemetrySample{GyroZ: 350, TS: 2000})
	got = o.Snapshot().HeadingDeg
	if got != 340 {
		t.Errorf("HeadingDeg after wrap = %v, want 340", got)
	}
	if got < 0 || got >= 360 {
		t.Errorf("HeadingDeg out of [0,360) range: %v", got)
	}
}

func TestIntegratorTracksLastFields(t *testing.T) {
	o := NewIntegrator(Config{MetersPerTick: 1})
	o.Update(TelemetrySample{EncoderDelta: 1, UltrasonicCM: 42.5, MotorPWM: 128, TS: 500})
	snap := o.Snapshot()
	if snap.LastTS != 500 {
		t.Errorf("LastTS = %v, want 500", snap.LastTS)
	}
	if snap.LastUltrasonicCM != 42.5 {
		t.Errorf("LastUltrasonicCM = %v, want 42.5", snap.LastUltrasonicCM)
	}
	if snap.LastMotorPWM != 128 {
		t.Errorf("LastMotorPWM = %v, want 128", snap.LastMotorPWM)
	}
}
