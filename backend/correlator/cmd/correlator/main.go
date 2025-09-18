package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

type Finding struct {
	ID        string                 `json:"id"`
	HostID    string                 `json:"host_id"`
	Severity  string                 `json:"severity"`
	Type      string                 `json:"type"`
	Evidence  map[string]any         `json:"evidence"`
	TS        string                 `json:"ts"`
}

func main(){
	natsURL := getenv("NATS_URL","nats://localhost:4222")
	nc, _ := nats.Connect(natsURL)
	defer nc.Drain()

	nc.Subscribe("etl.enriched", func(m *nats.Msg){
		var in map[string]any
		if err := json.Unmarshal(m.Data, &in); err!=nil { return }
		sev := "info"
		if cs, ok := in["candidates"].([]any); ok && len(cs)>0 { sev="medium" }
		f := Finding{
			ID: randID(), HostID: toStr(in["host_id"]), Severity: sev,
			Type: "pkg_cve_risk", Evidence: in, TS: time.Now().UTC().Format(time.RFC3339),
		}
		b, _ := json.Marshal(f)
		nc.Publish("correlator.findings", b)
		log.Printf("[correlator] finding %s sev=%s host=%s", f.ID, f.Severity, f.HostID)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })
	log.Printf("[correlator] up on :8085; NATS=%s", natsURL)
	log.Fatal(http.ListenAndServe(":8085", nil))
}

func getenv(k,d string) string { if v:=os.Getenv(k); v!=""{return v}; return d }

func toStr(v any) string {
	switch t := v.(type) {
	case string: return t
	default: return ""
	}
}

func randID() string { return time.Now().UTC().Format("20060102T150405.000000000Z") }
