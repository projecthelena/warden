// warden-probe corroborates connectivity from a second host using registered targets.
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/projecthelena/warden/internal/uptime"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:9097", "Listen address; use a TLS reverse proxy or SSH tunnel for remote access")
	config := flag.String("targets", "", "JSON file mapping monitor IDs to HTTP(S) URLs")
	flag.Parse()
	data, err := os.ReadFile(*config)
	if err != nil {
		log.Fatal("Cannot read probe targets file")
	}
	var targets map[string]string
	if json.Unmarshal(data, &targets) != nil {
		log.Fatal("Invalid probe targets JSON")
	}
	handler, err := uptime.NewProbeHandler(os.Getenv("SECONDARY_PROBE_TOKEN"), targets)
	if err != nil {
		log.Fatal(err)
	}
	server := &http.Server{Addr: *listen, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	log.Fatal(server.ListenAndServe())
}
