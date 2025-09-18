package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nats-io/nats.go"
	"aegisflux/backend/orchestrator/internal/api"
)

func main(){
	// Connect to NATS
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}
	
	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer nc.Close()
	
	// Create API server with NATS connection
	s, err := api.New(nc)
	if err != nil {
		log.Fatalf("Failed to create API server: %v", err)
	}
	
	addr := os.Getenv("ORCH_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	
	log.Printf("[orchestrator] listening on %s", addr)
	log.Printf("[orchestrator] connected to NATS at %s", natsURL)
	log.Fatal(http.ListenAndServe(addr, s.Handler()))
}
