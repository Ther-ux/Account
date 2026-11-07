package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"personal-ledger/internal/ledger"
)

func TestCreateSuccess(t *testing.T) {
	tests := []struct {
		name string
		body string
		want ledger.Record
	}{
		{
			name: "expense with Chinese note",
			body: `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":"午饭"}`,
			want: ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1250, Category: "饮食", Note: "午饭"},
		},
		{
			name: "income with omitted note",
			body: `{"date":"2026-11-07","type":"income","amountCents":300000,"category":"工资"}`,
			want: ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: 300000, Category: "工资"},
		},
		{
			name: "one cent and trailing whitespace",
			body: " \n" + `{"date":"2026-11-07","type":"expense","amountCents":1,"category":"交通","note":""}` + "\r\n\t ",
			want: ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1, Category: "交通"},
		},
		{
			name: "maximum int64 cents",
			body: `{"date":"2026-11-07","type":"income","amountCents":9223372036854775807,"category":"奖金","note":"<收款> & 奖金"}`,
			want: ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: 9223372036854775807, Category: "奖金", Note: "<收款> & 奖金"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			calls := 0
			handler := Create(func(record ledger.Record) error {
				calls++
				if record != tt.want {
					t.Errorf("appended record = %#v, want %#v", record, tt.want)
				}
				if rr.Flushed || rr.Body.Len() != 0 || len(rr.Header()) != 0 {
					t.Error("response started before append completed")
				}
				return nil
			})
			handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/records", strings.NewReader(tt.body)))
			if calls != 1 {
				t.Fatalf("append calls = %d, want 1", calls)
			}
			createCheckResponse(t, rr, http.StatusCreated)
			var got ledger.Record
			decoder := json.NewDecoder(rr.Body)
			if err := decoder.Decode(&got); err != nil {
				t.Fatalf("decode success response: %v", err)
			}
			if got != tt.want {
				t.Errorf("response record = %#v, want %#v", got, tt.want)
			}
			createCheckResponseEnd(t, decoder)
		})
	}
}

func TestCreateRejectsInvalidJSON(t *testing.T) {
	const valid = `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":"午饭"}`
	tests := []struct{ name, body string }{
		{"empty body", ""},
		{"whitespace body", " \r\n\t"},
		{"malformed object", `{"date":`},
		{"null", "null"},
		{"array", "[]"},
		{"string", `"record"`},
		{"number", "1250"},
		{"boolean", "true"},
		{"fractional cents", strings.Replace(valid, "1250", "12.5", 1)},
		{"decimal integer cents", strings.Replace(valid, "1250", "1250.0", 1)},
		{"exponent cents", strings.Replace(valid, "1250", "1e3", 1)},
		{"string cents", strings.Replace(valid, "1250", `"1250"`, 1)},
		{"boolean cents", strings.Replace(valid, "1250", "true", 1)},
		{"object cents", strings.Replace(valid, "1250", "{}", 1)},
		{"array cents", strings.Replace(valid, "1250", "[]", 1)},
		{"int64 overflow", strings.Replace(valid, "1250", "9223372036854775808", 1)},
		{"int64 underflow", strings.Replace(valid, "1250", "-9223372036854775809", 1)},
		{"second object", valid + "\n" + valid},
		{"trailing null", valid + " null"},
		{"trailing scalar", valid + " true"},
		{"trailing junk", valid + " invalid"},
		{"truncated trailing object", valid + " {"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			rr := httptest.NewRecorder()
			Create(func(ledger.Record) error {
				calls++
				return nil
			}).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/records", strings.NewReader(tt.body)))
			if calls != 0 {
				t.Fatalf("append calls = %d, want 0", calls)
			}
			createCheckError(t, rr, http.StatusBadRequest)
		})
	}
}

func TestCreateRejectsInvalidRecord(t *testing.T) {
	const valid = `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":"午饭"}`
	tests := []struct{ name, body string }{
		{"empty record", "{}"},
		{"missing date", strings.Replace(valid, `"date":"2026-11-07",`, "", 1)},
		{"invalid calendar date", strings.Replace(valid, "2026-11-07", "2026-02-30", 1)},
		{"invalid date format", strings.Replace(valid, "2026-11-07", "2026-1-7", 1)},
		{"invalid type", strings.Replace(valid, "expense", "transfer", 1)},
		{"missing amount", strings.Replace(valid, `"amountCents":1250,`, "", 1)},
		{"zero cents", strings.Replace(valid, "1250", "0", 1)},
		{"negative cents", strings.Replace(valid, "1250", "-1", 1)},
		{"null cents", strings.Replace(valid, "1250", "null", 1)},
		{"missing category", strings.Replace(valid, `"category":"饮食",`, "", 1)},
		{"unknown category", strings.Replace(valid, "饮食", "旅游", 1)},
		{"mismatched category", strings.Replace(valid, "饮食", "工资", 1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var record ledger.Record
			if err := json.Unmarshal([]byte(tt.body), &record); err != nil {
				t.Fatalf("test fixture must be valid JSON: %v", err)
			}
			validationErr := ledger.Validate(record)
			if validationErr == nil {
				t.Fatal("ledger.Validate must reject test record")
			}
			calls := 0
			rr := httptest.NewRecorder()
			Create(func(ledger.Record) error {
				calls++
				return nil
			}).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/records", strings.NewReader(tt.body)))
			if calls != 0 {
				t.Fatalf("append calls = %d, want 0", calls)
			}
			if message := createCheckError(t, rr, http.StatusBadRequest); message != validationErr.Error() {
				t.Errorf("error = %q, want validation error %q", message, validationErr.Error())
			}
		})
	}
}

func TestCreateAppendFailure(t *testing.T) {
	const body = `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":"午饭"}`
	rr := httptest.NewRecorder()
	calls := 0
	Create(func(record ledger.Record) error {
		calls++
		if record.AmountCents != 1250 || record.Note != "午饭" {
			t.Errorf("unexpected record: %#v", record)
		}
		if rr.Flushed || rr.Body.Len() != 0 || len(rr.Header()) != 0 {
			t.Error("response started before append failed")
		}
		return errors.New("disk full")
	}).ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/records", strings.NewReader(body)))
	if calls != 1 {
		t.Fatalf("append calls = %d, want 1", calls)
	}
	if message := createCheckError(t, rr, http.StatusInternalServerError); strings.Contains(message, "disk full") || !strings.Contains(message, "保存") {
		t.Errorf("error = %q, want readable error without raw storage details", message)
	}
}

func createCheckResponse(t *testing.T, rr *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rr.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, status, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want JSON with UTF-8", got)
	}
}

func createCheckError(t *testing.T, rr *httptest.ResponseRecorder, status int) string {
	t.Helper()
	createCheckResponse(t, rr, status)
	var body map[string]string
	decoder := json.NewDecoder(rr.Body)
	if err := decoder.Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if len(body) != 1 || body["error"] == "" {
		t.Fatalf("want a single nonempty error field, got %#v", body)
	}
	createCheckResponseEnd(t, decoder)
	return body["error"]
}

func createCheckResponseEnd(t *testing.T, decoder *json.Decoder) {
	t.Helper()
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatalf("want one JSON response document, got trailing decode result: %v", err)
	}
}
