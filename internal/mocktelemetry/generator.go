// Package mocktelemetry is a pure sample generator standing in for the
// ESP32-S3 firmware during backend development: steady forward motion with
// light jitter, a scripted stall window (exercises the debounce end-to-end),
// and a graceful stop once the scripted route distance is reached. It knows
// nothing about WebSockets — cmd/mockclient wraps it and dials the real
// /ws/telemetry endpoint.
package mocktelemetry

import (
	"math/rand"
	"time"

	"hazard-mine-poc/internal/odometry"
)

// Config controls the generator's scripted behavior. Encoder ticks are
// treated 1:1 as meters here, matching the MetersPerTick: 1 calibration
// cmd/server uses by default — keep the two in sync if either changes.
type Config struct {
	TotalRouteM          float64       // cumulative distance at which the cart "arrives"
	TicksPerSample       int64         // nominal encoder ticks per sample under normal motion
	JitterTicks          int64         // +/- random jitter applied to TicksPerSample
	MotorPWM             int           // commanded motor power while moving/stalling
	SampleInterval       time.Duration // ~10Hz => 100ms
	StallAfterM          float64       // cumulative distance at which the scripted stall window begins
	StallDurationSamples int           // consecutive samples the stall window lasts
	GyroZ                float64       // constant yaw-rate signal, for demo purposes
	UltrasonicCM         float64       // constant ultrasonic reading, for demo purposes
}

// DefaultConfig returns sane defaults for a demo run against the seeded
// 5-node graph. TotalRouteM is intentionally well above the 55m default
// (unblocked) route length: the generator has no visibility into the
// backend's graph/routing (it just streams raw ticks, like a real ESP32
// would), so it can't know a hazard-triggered reroute made the cart's
// actual path longer than the default. Since a stall hazard blocks
// whatever edge the cart is currently on, the scripted stall window in
// this same generator will very likely trigger exactly that reroute during
// a normal demo run — cutting movement off at exactly 55m would freeze the
// cart mid-route, short of EXIT, for the rest of the demo. 150m safely
// covers every reroute this 5-node/7-edge topology can produce (worst-case
// simple path is well under 150m) while still finishing in well under a
// minute at the default sample rate; the backend is idempotent about
// receiving "extra" ticks after the cart's route is actually exhausted
// (ApplyTelemetry simply stops advancing once Arrived), so overshoot here
// is harmless.
func DefaultConfig() Config {
	return Config{
		TotalRouteM:          150,
		TicksPerSample:       5,
		JitterTicks:          1,
		MotorPWM:             180,
		SampleInterval:       100 * time.Millisecond,
		StallAfterM:          10,
		StallDurationSamples: 15,
		GyroZ:                0.5,
		UltrasonicCM:         120,
	}
}

// Generator produces a scripted sequence of odometry.TelemetrySample values.
// Not safe for concurrent use.
type Generator struct {
	cfg Config
	rng *rand.Rand

	cumulativeM      float64
	tsMs             int64
	stallTriggered   bool // the one scripted stall window has already fired
	stalling         bool
	stallSamplesLeft int
	arrived          bool
}

// NewGenerator builds a Generator for the given config.
func NewGenerator(cfg Config) *Generator {
	return &Generator{
		cfg: cfg,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Arrived reports whether the generator has reached TotalRouteM and stopped
// producing forward motion.
func (g *Generator) Arrived() bool { return g.arrived }

// Next produces the next TelemetrySample in the script.
func (g *Generator) Next() odometry.TelemetrySample {
	g.tsMs += g.cfg.SampleInterval.Milliseconds()

	if g.arrived {
		// Graceful stop: keep sending idle telemetry (motor off, no
		// movement) rather than stopping the stream outright.
		return odometry.TelemetrySample{
			EncoderDelta: 0,
			GyroZ:        0,
			MotorPWM:     0,
			UltrasonicCM: g.cfg.UltrasonicCM,
			TS:           g.tsMs,
		}
	}

	if !g.stallTriggered && !g.stalling && g.cumulativeM >= g.cfg.StallAfterM {
		g.stalling = true
		g.stallTriggered = true
		g.stallSamplesLeft = g.cfg.StallDurationSamples
	}

	if g.stalling {
		g.stallSamplesLeft--
		if g.stallSamplesLeft <= 0 {
			g.stalling = false
		}
		// Motor commanded on, wheel encoder shows no movement: the core
		// hazard signal the backend's stall debouncer is watching for.
		return odometry.TelemetrySample{
			EncoderDelta: 0,
			GyroZ:        0,
			MotorPWM:     g.cfg.MotorPWM,
			UltrasonicCM: g.cfg.UltrasonicCM,
			TS:           g.tsMs,
		}
	}

	delta := g.cfg.TicksPerSample
	if g.cfg.JitterTicks > 0 {
		delta += g.rng.Int63n(2*g.cfg.JitterTicks+1) - g.cfg.JitterTicks
	}
	if delta < 0 {
		delta = 0
	}

	g.cumulativeM += float64(delta)
	if g.cumulativeM >= g.cfg.TotalRouteM {
		g.arrived = true
	}

	return odometry.TelemetrySample{
		EncoderDelta: delta,
		GyroZ:        g.cfg.GyroZ,
		MotorPWM:     g.cfg.MotorPWM,
		UltrasonicCM: g.cfg.UltrasonicCM,
		TS:           g.tsMs,
	}
}
