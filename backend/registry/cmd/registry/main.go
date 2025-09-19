package main

import (
	"log"
	"net/http"
	"os"

	"backend/registry/internal/api"
)

func main() {
	addr := os.Getenv("REGISTRY_ADDR")
	if addr == "" { addr = ":8090" }
	s := api.NewServer()
	log.Printf("[registry] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, s.Handler()))
}

