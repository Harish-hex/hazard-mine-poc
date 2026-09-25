// Command wsdump is a tiny dev helper: it dials /ws/dashboard and prints
// every frame it receives. Stands in for `websocat`, which isn't installed
// in this environment.
package main

import (
	"flag"
	"log"

	"github.com/gorilla/websocket"
)

func main() {
	url := flag.String("url", "ws://localhost:8080/ws/dashboard", "dashboard WS URL to dial")
	flag.Parse()

	conn, _, err := websocket.DefaultDialer.Dial(*url, nil)
	if err != nil {
		log.Fatalf("wsdump: dial %s: %v", *url, err)
	}
	defer conn.Close()

	log.Printf("wsdump: connected to %s", *url)

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Fatalf("wsdump: read error: %v", err)
		}
		log.Printf("%s", msg)
	}
}
