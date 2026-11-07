package ledger_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"personal-ledger/internal/ledger"
)

func TestValidateValidRecords(t *testing.T) {
	tests := []struct {
		name   string
		record ledger.Record
	}{
		{"salary", ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: 300000, Category: "工资"}},
		{"bonus", ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: math.MaxInt64, Category: "奖金", Note: "年度奖金"}},
		{"other income", ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: 1, Category: "其他收入"}},
		{"food with Chinese note", ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1250, Category: "饮食", Note: "午饭"}},
		{"transport", ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1750, Category: "交通"}},
		{"shopping", ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 100, Category: "购物"}},
		{"other expense preserves note", ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1, Category: "其他支出", Note: "  中文备注\n保持原样  "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.record
			if err := ledger.Validate(tt.record); err != nil {
				t.Errorf("Validate(%#v): %v", tt.record, err)
			}
			if tt.record != before {
				t.Errorf("Validate changed input: got %#v, want %#v", tt.record, before)
			}
		})
	}
}

func TestValidateDates(t *testing.T) {
	tests := []struct {
		date string
		ok   bool
	}{
		{"2026-11-07", true},
		{"2024-02-29", true},
		{"2000-02-29", true},
		{"2026-02-28", true},
		{"2026-04-30", true},
		{"2026-01-01", true},
		{"2026-12-31", true},
		{"0001-01-01", true},
		{"0000-01-01", false},
		{"9999-12-31", true},
		{"", false},
		{"2026-02-29", false},
		{"1900-02-29", false},
		{"2100-02-29", false},
		{"2024-02-30", false},
		{"2026-11-31", false},
		{"2026-04-31", false},
		{"2026-00-07", false},
		{"2026-13-07", false},
		{"2026-11-00", false},
		{"2026-11-32", false},
		{"2026-1-7", false},
		{"2026-01-7", false},
		{"2026-1-07", false},
		{"26-11-07", false},
		{"10000-01-01", false},
		{"-001-01-01", false},
		{"2026/11/07", false},
		{"2026-11- 7", false},
		{" 2026-11-07", false},
		{"2026-11-07 ", false},
		{"2026-11-07\n", false},
		{"2026-11-07T00:00:00Z", false},
		{"２０２６-１１-０７", false},
	}
	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			record := ledger.Record{Date: tt.date, Type: "expense", AmountCents: 1, Category: "饮食"}
			before := record
			err := ledger.Validate(record)
			if tt.ok && err != nil {
				t.Errorf("valid date rejected: %v", err)
			}
			if !tt.ok && (err == nil || !strings.Contains(err.Error(), "date")) {
				t.Errorf("invalid date: got error %v, want readable date error", err)
			}
			if record != before {
				t.Errorf("Validate changed input: got %#v, want %#v", record, before)
			}
		})
	}
}

func TestValidateRejectsInvalidFields(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*ledger.Record)
		wantError string
	}{
		{"empty record", func(r *ledger.Record) { *r = ledger.Record{} }, "date"},
		{"zero cents", func(r *ledger.Record) { r.AmountCents = 0 }, "amountCents"},
		{"negative cents", func(r *ledger.Record) { r.AmountCents = -1 }, "amountCents"},
		{"minimum int64 cents", func(r *ledger.Record) { r.AmountCents = math.MinInt64 }, "amountCents"},
		{"empty type", func(r *ledger.Record) { r.Type = "" }, "type"},
		{"unknown type", func(r *ledger.Record) { r.Type = "transfer" }, "type"},
		{"uppercase type", func(r *ledger.Record) { r.Type = "EXPENSE" }, "type"},
		{"type whitespace", func(r *ledger.Record) { r.Type = " expense " }, "type"},
		{"empty category", func(r *ledger.Record) { r.Category = "" }, "category"},
		{"unknown category", func(r *ledger.Record) { r.Category = "旅游" }, "category"},
		{"category whitespace", func(r *ledger.Record) { r.Category = " 饮食 " }, "category"},
		{"income category with expense", func(r *ledger.Record) { r.Category = "工资" }, "category"},
		{"bonus category with expense", func(r *ledger.Record) { r.Category = "奖金" }, "category"},
		{"other income with expense", func(r *ledger.Record) { r.Category = "其他收入" }, "category"},
		{"food category with income", func(r *ledger.Record) { r.Type = "income" }, "category"},
		{"transport with income", func(r *ledger.Record) { r.Type = "income"; r.Category = "交通" }, "category"},
		{"shopping with income", func(r *ledger.Record) { r.Type = "income"; r.Category = "购物" }, "category"},
		{"other expense with income", func(r *ledger.Record) { r.Type = "income"; r.Category = "其他支出" }, "category"},
		{"empty income category", func(r *ledger.Record) { r.Type = "income"; r.Category = "" }, "category"},
		{"unknown income category", func(r *ledger.Record) { r.Type = "income"; r.Category = "旅游" }, "category"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1250, Category: "饮食", Note: "午饭"}
			tt.change(&record)
			before := record
			err := ledger.Validate(record)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Errorf("Validate(%#v) = %v, want error containing %q", record, err, tt.wantError)
			}
			if record != before {
				t.Errorf("Validate changed input: got %#v, want %#v", record, before)
			}
		})
	}
}

func TestRecordJSONContract(t *testing.T) {
	record := ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1250, Category: "饮食", Note: "午饭"}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"date": "2026-11-07", "type": "expense", "amountCents": float64(1250), "category": "饮食", "note": "午饭",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON fields = %#v, want %#v", got, want)
	}

	tests := []struct {
		name string
		body string
		want ledger.Record
	}{
		{"Chinese note", `{"date":"2026-11-07","type":"expense","amountCents":1250,"category":"饮食","note":"午饭"}`, record},
		{"omitted note", `{"date":"2026-11-07","type":"income","amountCents":300000,"category":"工资"}`, ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: 300000, Category: "工资"}},
		{"empty note", `{"date":"2026-11-07","type":"expense","amountCents":1,"category":"交通","note":""}`, ledger.Record{Date: "2026-11-07", Type: "expense", AmountCents: 1, Category: "交通"}},
		{"maximum int64 cents", `{"date":"2026-11-07","type":"income","amountCents":9223372036854775807,"category":"奖金","note":"奖金"}`, ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: math.MaxInt64, Category: "奖金", Note: "奖金"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var decoded ledger.Record
			if err := json.Unmarshal([]byte(tt.body), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded != tt.want {
				t.Errorf("decoded record = %#v, want %#v", decoded, tt.want)
			}
			if err := ledger.Validate(decoded); err != nil {
				t.Fatalf("decoded record rejected: %v", err)
			}
			encoded, err := json.Marshal(decoded)
			if err != nil {
				t.Fatal(err)
			}
			var roundTrip ledger.Record
			if err := json.Unmarshal(encoded, &roundTrip); err != nil {
				t.Fatal(err)
			}
			if roundTrip != tt.want {
				t.Errorf("JSON round trip = %#v, want %#v", roundTrip, tt.want)
			}
			if !strings.Contains(string(encoded), `"note":`) {
				t.Errorf("JSON must retain the note field even when empty: %s", encoded)
			}
		})
	}
}
