package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"math"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"personal-ledger/internal/ledger"
)

func TestInsightsFiltersBudgetAndTrend(t *testing.T) {
	records := statsTestRecords()
	records = append(records, ledger.Record{Date: "2026-10-31", Type: "expense", AmountCents: 400, Category: "购物"})
	response := httptest.NewRecorder()
	Insights(func() ([]ledger.Record, error) { return records, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/insights?month=2026-11&category=交通", nil))
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	var got insightResult
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Count != 1 || got.Totals.ExpenseCents != 1750 || got.MonthExpenseCents != 3000 || got.Month != "2026-11" {
		t.Fatalf("%+v", got)
	}
	if len(got.Categories) != 1 || got.Categories[0].Category != "交通" || len(got.Trend) != 1 || got.Trend[0].ExpenseCents != 1750 {
		t.Fatalf("%+v", got)
	}
	var many []ledger.Record
	day, _ := time.Parse(time.DateOnly, "2026-11-01")
	for i := 0; i < 20; i++ {
		many = append(many, ledger.Record{Date: day.AddDate(0, 0, i).Format(time.DateOnly), Type: "expense", AmountCents: int64(i + 1), Category: "饮食"})
	}
	result, err := insightsCompute(many, statsFilters{}, "2026-11")
	if err != nil || len(result.Trend) != 14 || result.Trend[0].Date != "2026-11-07" || result.Trend[13].Date != "2026-11-20" {
		t.Fatalf("%+v %v", result, err)
	}
	if result.Categories[0].ExpenseCents != 210 || result.MonthExpenseCents != 210 || result.Count != 20 {
		t.Fatalf("%+v", result)
	}
}
func TestInsightsEmptyErrorsAndOverflow(t *testing.T) {
	empty, err := insightsCompute(nil, statsFilters{}, "2026-11")
	if err != nil || empty.Categories == nil || empty.Trend == nil || empty.MonthExpenseCents != 0 {
		t.Fatalf("%+v %v", empty, err)
	}
	for _, query := range []string{"?month=2026-13", "?month=2026-1", "?month=0000-01", "?month=a&month=b", "?from=2026-02-29", "?minCents=-1", "?category=坏类别", "?month=%ZZ"} {
		response := httptest.NewRecorder()
		Insights(func() ([]ledger.Record, error) { t.Fatal("invalid parameters must not read"); return nil, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/insights"+query, nil))
		if response.Code != 400 {
			t.Fatalf("%s %d %s", query, response.Code, response.Body)
		}
	}
	for _, callback := range []func() ([]ledger.Record, error){nil, func() ([]ledger.Record, error) { return nil, errors.New("C:/private/ledger.json") }} {
		response := httptest.NewRecorder()
		Insights(callback).ServeHTTP(response, httptest.NewRequest("GET", "/api/insights", nil))
		if response.Code != 500 || strings.Contains(response.Body.String(), "private") {
			t.Fatal(response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	Insights(nil).ServeHTTP(response, httptest.NewRequest("POST", "/api/insights", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
	records := []ledger.Record{{Date: "2026-11-07", Type: "expense", Category: "交通", AmountCents: math.MaxInt64}, {Date: "2026-11-07", Type: "expense", Category: "饮食", AmountCents: 1}}
	if _, err := insightsCompute(records, statsFilters{category: "交通"}, "2026-11"); err == nil {
		t.Fatal("unfiltered monthly overflow must be detected")
	}
	if _, err := insightsCompute([]ledger.Record{{Date: "bad", Type: "income", AmountCents: 1, Category: "工资"}}, statsFilters{}, "2026-11"); err == nil {
		t.Fatal("invalid stored record accepted")
	}
}
func TestExportExactJSONAndSafeCSV(t *testing.T) {
	records := []ledger.Record{
		{Date: "2026-11-07", Type: "income", AmountCents: math.MaxInt64, Category: "奖金", Note: "=HYPERLINK(\"bad\")"},
		{Date: "2026-11-07", Type: "expense", AmountCents: 1, Category: "交通", Note: "中文,备注\n保留换行"},
	}
	response := httptest.NewRecorder()
	Export(func() ([]ledger.Record, error) { return records, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/export?format=json", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), "9223372036854775807") || !strings.Contains(response.Header().Get("Content-Disposition"), ".json") {
		t.Fatal(response.Body.String())
	}
	var decoded []ledger.Record
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil || decoded[0] != records[0] {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	Export(func() ([]ledger.Record, error) { return records, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/export?format=csv", nil))
	if response.Code != 200 || !bytes.HasPrefix(response.Body.Bytes(), []byte{0xef, 0xbb, 0xbf}) {
		t.Fatal(response.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(response.Body.String(), "\ufeff"))).ReadAll()
	if err != nil || len(rows) != 3 || rows[1][2] != "92233720368547758.07" || rows[2][2] != "0.01" || rows[1][4] != "'"+records[0].Note || rows[2][4] != records[1].Note {
		t.Fatalf("%#v %v", rows, err)
	}
	for _, text := range []string{"=1+1", "+cmd", "-1+2", "@SUM(A1)", " \t=1", "\ttext", "\rtext"} {
		if !strings.HasPrefix(exportCSVText(text), "'") {
			t.Fatal(text)
		}
	}
	if exportCSVText("普通备注") != "普通备注" {
		t.Fatal("normal note changed")
	}
}
func TestExportErrorAndEmptyCases(t *testing.T) {
	for _, query := range []string{"?format=x", "?format=json&format=csv", "?format=%ZZ"} {
		response := httptest.NewRecorder()
		Export(func() ([]ledger.Record, error) { t.Fatal("invalid format must not read"); return nil, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/export"+query, nil))
		if response.Code != 400 {
			t.Fatal(response.Code)
		}
	}
	for _, callback := range []func() ([]ledger.Record, error){nil, func() ([]ledger.Record, error) { return nil, errors.New("C:/private/records.json") }, func() ([]ledger.Record, error) { return []ledger.Record{{}}, nil }} {
		response := httptest.NewRecorder()
		Export(callback).ServeHTTP(response, httptest.NewRequest("GET", "/api/export", nil))
		if response.Code != 500 || strings.Contains(response.Body.String(), "private") || response.Header().Get("Content-Disposition") != "" {
			t.Fatal(response.Body.String())
		}
	}
	response := httptest.NewRecorder()
	Export(func() ([]ledger.Record, error) { return nil, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/export", nil))
	if response.Code != 200 || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatal(response.Body.String())
	}
	response = httptest.NewRecorder()
	Export(nil).ServeHTTP(response, httptest.NewRequest("POST", "/api/export", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
}
func TestIntegrationInputBoundaries(t *testing.T) {
	valid := `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":""}`
	for _, body := range []string{strings.Replace(valid, `"note":""`, `"note":"","unknown":true`, 1), strings.Replace(valid, `"note":""`, `"note":"`+strings.Repeat("x", 1<<20)+`"`, 1)} {
		response := httptest.NewRecorder()
		Create(func(ledger.Record) error { t.Fatal("invalid body saved"); return nil }).ServeHTTP(response, httptest.NewRequest("POST", "/api/records", strings.NewReader(body)))
		if response.Code != 400 {
			t.Fatalf("%d %s", response.Code, response.Body)
		}
	}
	response := httptest.NewRecorder()
	Create(func(ledger.Record) error { return errors.New("C:/private/records.json: no permission") }).ServeHTTP(response, httptest.NewRequest("POST", "/api/records", strings.NewReader(valid)))
	if response.Code != 500 || strings.Contains(response.Body.String(), "private") {
		t.Fatal(response.Body.String())
	}
	response = httptest.NewRecorder()
	Recent(nil).ServeHTTP(response, httptest.NewRequest("GET", "/api/records/recent", nil))
	if response.Code != 500 {
		t.Fatal(response.Code)
	}
	response = httptest.NewRecorder()
	Recent(nil).ServeHTTP(response, httptest.NewRequest("POST", "/api/records/recent", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
	response = httptest.NewRecorder()
	Recent(func() ([]ledger.Record, error) { t.Fatal("bad limit"); return nil, nil }).ServeHTTP(response, httptest.NewRequest("GET", "/api/records/recent?limit=%2B1", nil))
	if response.Code != 400 {
		t.Fatal(response.Code)
	}
	response = httptest.NewRecorder()
	Create(nil).ServeHTTP(response, httptest.NewRequest("POST", "/api/records", strings.NewReader(valid)))
	if response.Code != 500 {
		t.Fatal(response.Code)
	}
	response = httptest.NewRecorder()
	Create(nil).ServeHTTP(response, httptest.NewRequest("GET", "/api/records", nil))
	if response.Code != 405 {
		t.Fatal(response.Code)
	}
}
