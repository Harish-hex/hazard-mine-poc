// Command mockclient dials /ws/telemetry as a fake ESP32-S3, streaming a
// scripted mocktelemetry.Generator sequence at ~10Hz: steady forward motion
// with light jitter, a scripted stall window, then a graceful stop once the
// route completes. This is the backend-first verification path from the
// build plan — no Pi/hardware needed.
package main

import (
	"flag"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"hazard-mine-poc/internal/mocktelemetry"
)

func main() {
	url := flag.String("url", "ws://localhost:8080/ws/telemetry", "telemetry WS URL to dial")
	flag.Parse()

	conn, _, err := websocket.DefaultDialer.Dial(*url, nil)
	if err != nil {
		log.Fatalf("mockclient: dial %s: %v", *url, err)
	}
	defer conn.Close()

	log.Printf("mockclient: connected to %s", *url)

	cfg := mocktelemetry.DefaultConfig()
	gen := mocktelemetry.NewGenerator(cfg)

	ticker := time.NewTicker(cfg.SampleInterval)
	defer ticker.Stop()

	announcedArrival := false
	for range ticker.C {
		sample := gen.Next()
		if err := conn.WriteJSON(sample); err != nil {
			log.Fatalf("mockclient: write error: %v", err)
		}
		if gen.Arrived() && !announcedArrival {
			announcedArrival = true
			log.Printf("mockclient: route complete, cart arrived — continuing to send idle telemetry")
		}
	}
}
