package main

import (
	"log"
	"net/http"
	"os"
	"backend/actionsapi/internal/api"
)

func main() {
	addr := os.Getenv("ACTIONS_ADDR")
	if addr == "" { addr = ":8083" }
	s := api.NewServer()
	log.Printf("[actions-api] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, s.Handler()))
}

