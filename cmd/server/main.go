// Command server is the ProofMuse HTTP server: the packing-evidence
// assistant that Indonesian online sellers use at the packing bench.
//
// Day 1 of the Meta Global AI Developer Hackathon (2026-10-26): only
// /health exists. The pipeline lands in later commits; HACKATHON.md
// records what was built during the event and what was ported.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	log.Printf("proofmuse listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
