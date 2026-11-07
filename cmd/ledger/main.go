package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"personal-ledger/internal/httpapi"
	"personal-ledger/internal/store"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	flag.Parse()

	server := &http.Server{
		Addr:              *addr,
		Handler:           newHandler("web"),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("个人记账本 http://%s", *addr)
	log.Fatal(server.ListenAndServe())
}

func newHandler(webDir string) http.Handler {
	api := http.NewServeMux()
	records := store.NewJSONStore("data/records.json")
	api.HandleFunc("POST /api/records", httpapi.Create(records.Append))
	api.HandleFunc("GET /api/records/recent", httpapi.Recent(records.List))
	api.HandleFunc("GET /api/stats", httpapi.Stats(records.List))

	mux := http.NewServeMux()
	mux.Handle("/api/", api)
	mux.Handle("/", http.FileServer(http.Dir(webDir)))
	return mux
}
