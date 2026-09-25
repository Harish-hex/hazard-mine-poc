// Command server wires graph + state + httpapi + wsserver together and
// starts the real Hazard Mine POC backend.
package main

import (
	"flag"
	"log"
	"net/http"

	"hazard-mine-poc/internal/graph"
	"hazard-mine-poc/internal/httpapi"
	"hazard-mine-poc/internal/odometry"
	"hazard-mine-poc/internal/state"
	"hazard-mine-poc/internal/wsserver"
)

func main() {
	port := flag.String("port", "8080", "HTTP listen port")
	flag.Parse()

	g := graph.Seed()
	defaultPath := []graph.NodeID{"N1", "N2", "N4", "EXIT"}

	hub := wsserver.NewHub()

	odoCfg := odometry.Config{MetersPerTick: 1}
	stallCfg := odometry.StallDebouncer{
		PWMThreshold:   50,
		OnSamples:      5,
		OffSamples:     5,
		EncoderEpsilon: 1,
	}

	store := state.NewStore(g, defaultPath, odoCfg, stallCfg, hub)

	telemetryHandler := wsserver.NewTelemetryHandler(store)
	dashboardHandler := wsserver.NewDashboardHandler(hub)

	router := httpapi.NewRouter(httpapi.Deps{
		Store:     store,
		Graph:     g,
		Telemetry: telemetryHandler,
		Dashboard: dashboardHandler,
	})

	addr := ":" + *port
	log.Printf("hazard-mine-poc server listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
