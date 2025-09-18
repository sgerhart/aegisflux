package main

import (
	"log"
	"net/http"
	"os"
)

func main(){
	addr := os.Getenv("SEG_HTTP_ADDR"); if addr == "" { addr=":8086" }
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })
	// TODO: POST /segment/propose and /segment/plan
	log.Printf("[segmenter] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
