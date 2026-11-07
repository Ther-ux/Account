package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"personal-ledger/internal/ledger"
	"personal-ledger/internal/store"
	webassets "personal-ledger/web"
)

func TestApplicationWiringAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	handler := newApplication(store.NewJSONStore(path), webassets.Files, false)
	for _, body := range []string{
		`{"date":"2026-11-07","type":"income","amountCents":300000,"category":"工资"}`,
		`{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食"}`,
		`{"date":"2026-11-07","type":"expense","amountCents":1750,"category":"交通"}`,
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/records", strings.NewReader(body)))
		if response.Code != 201 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
	}
	handler = newApplication(store.NewJSONStore(path), webassets.Files, false)
	for _, endpoint := range []string{"/api/records/recent", "/api/stats", "/api/insights?month=2026-11", "/api/export?format=json", "/api/export?format=csv", "/api/meta", "/", "/app.js", "/app.css", "/record.js", "/recent.js", "/stats.js", "/insights.js"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest("GET", endpoint, nil))
		if response.Code != 200 {
			t.Fatalf("%s: %d %s", endpoint, response.Code, response.Body)
		}
		if !strings.HasPrefix(endpoint, "/api/") && strings.Contains(response.Body.String(), "待接入") {
			t.Fatal("placeholder left")
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/stats", nil))
	if response.Body.String() != "{\"incomeCents\":300000,\"expenseCents\":3000,\"balanceCents\":297000}\n" {
		t.Fatal(response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/data/records.json", nil))
	if response.Code != 404 {
		t.Fatal("private data served as static asset")
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "/api/not-found", nil))
	if response.Code != 404 || !strings.Contains(response.Header().Get("Content-Type"), "json") {
		t.Fatal("API error not JSON")
	}
	request := httptest.NewRequest("POST", "/api/records", strings.NewReader("{}"))
	request.Header.Set("Origin", "https://unrelated.example")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 403 {
		t.Fatal("cross-origin write accepted")
	}
}
func TestDemoSeedNeverResetsExistingLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.json")
	records := store.NewJSONStore(path)
	now := time.Date(2026, 11, 7, 12, 0, 0, 0, time.Local)
	if err := seedDemo(records, path, now); err != nil {
		t.Fatal(err)
	}
	first, err := records.List()
	if err != nil || len(first) != 9 {
		t.Fatalf("%d %v", len(first), err)
	}
	before, _ := os.ReadFile(path)
	if err := seedDemo(records, path, now.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("demo reseeded existing data")
	}
	for _, record := range first {
		if err := ledger.Validate(record); err != nil {
			t.Fatal(err)
		}
	}
	response := httptest.NewRecorder()
	newApplication(records, webassets.Files, true).ServeHTTP(response, httptest.NewRequest("GET", "/api/meta", nil))
	var meta map[string]any
	_ = json.Unmarshal(response.Body.Bytes(), &meta)
	if meta["demo"] != true {
		t.Fatal(response.Body.String())
	}
}
