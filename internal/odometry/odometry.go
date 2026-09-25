// Package odometry integrates raw single-wheel-encoder + gyro telemetry into
// cumulative distance and heading. It has no dependency on internal/graph:
// distance and heading are independent scalars, never fused into a 2D
// position (single wheel encoder, no differential drive).
package odometry

// TelemetrySample is one raw packet from the ESP32-S3, as received over the
// telemetry WebSocket at ~10Hz.
type TelemetrySample struct {
	EncoderDelta int64   `json:"encoder_delta"`
	GyroZ        float64 `json:"gyro_z"`
	MotorPWM     int     `json:"motor_pwm"`
	UltrasonicCM float64 `json:"ultrasonic_cm"`
	TS           int64   `json:"ts"`
}

// Config holds calibration constants for translating raw ticks into meters.
type Config struct {
	// MetersPerTick is the placeholder calibration constant: distance
	// traveled per single encoder tick.
	MetersPerTick float64
}

// State is the integrator's current output snapshot.
type State struct {
	CumulativeDistanceM float64
	HeadingDeg          float64 // wrapped to [0, 360)
	LastTS              int64
	LastUltrasonicCM    float64
	LastMotorPWM        int
}

// Integrator accumulates TelemetrySample updates into cumulative distance
// and heading. Not safe for concurrent use — callers (internal/state) must
// serialize access.
type Integrator struct {
	cfg    Config
	st     State
	primed bool // true once at least one sample has set LastTS; distinguishes
	// "no previous sample yet" from a legitimate first TS of 0.
}

// NewIntegrator creates an Integrator with the given calibration config.
func NewIntegrator(cfg Config) *Integrator {
	return &Integrator{cfg: cfg}
}

// Update folds one telemetry sample into the integrator's running state:
// encoder_delta * MetersPerTick adds to cumulative distance, gyro_z is
// treated as degrees/sec and integrated over the elapsed time since the
// previous sample (using each sample's own ts field; the first sample only
// primes LastTS and contributes no heading delta).
func (o *Integrator) Update(s TelemetrySample) {
	distDelta := float64(s.EncoderDelta) * o.cfg.MetersPerTick
	o.st.CumulativeDistanceM += distDelta

	if o.primed {
		dtSec := float64(s.TS-o.st.LastTS) / 1000.0
		if dtSec > 0 {
			heading := o.st.HeadingDeg + s.GyroZ*dtSec
			heading = wrap360(heading)
			o.st.HeadingDeg = heading
		}
	}

	o.st.LastTS = s.TS
	o.st.LastUltrasonicCM = s.UltrasonicCM
	o.st.LastMotorPWM = s.MotorPWM
	o.primed = true
}

// Snapshot returns a copy of the current integrator state.
func (o *Integrator) Snapshot() State {
	return o.st
}

func wrap360(deg float64) float64 {
	const full = 360.0
	d := deg
	for d < 0 {
		d += full
	}
	for d >= full {
		d -= full
	}
	return d
}
