package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"time"

	"personal-ledger/internal/ledger"
)

type insightCategory struct {
	Category     string `json:"category"`
	ExpenseCents int64  `json:"expenseCents"`
}
type insightDay struct {
	Date         string `json:"date"`
	IncomeCents  int64  `json:"incomeCents"`
	ExpenseCents int64  `json:"expenseCents"`
}
type insightResult struct {
	Totals            statsResult       `json:"totals"`
	Count             int               `json:"count"`
	Categories        []insightCategory `json:"categories"`
	Trend             []insightDay      `json:"trend"`
	Month             string            `json:"month"`
	MonthExpenseCents int64             `json:"monthExpenseCents"`
}

// Insights shares the exact filters and integer aggregation used by Stats.
// Monthly expense is deliberately unfiltered so budget progress cannot be
// understated by selecting a single category in the statistics form.
func Insights(listRecords func() ([]ledger.Record, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			statsWriteError(w, 405, "请使用 GET 请求")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			statsWriteError(w, 400, "查询参数编码无效")
			return
		}
		filters, err := statsParseFilters(query)
		if err != nil {
			statsWriteError(w, 400, err.Error())
			return
		}
		month := query.Get("month")
		if month == "" {
			month = time.Now().Format("2006-01")
		}
		parsed, err := time.Parse("2006-01", month)
		if len(query["month"]) > 1 || err != nil || parsed.Year() < 1 || parsed.Format("2006-01") != month {
			statsWriteError(w, 400, "月份必须为 YYYY-MM")
			return
		}
		if listRecords == nil {
			statsWriteError(w, 500, "账本读取服务未配置")
			return
		}
		records, err := listRecords()
		if err != nil {
			statsWriteError(w, 500, "读取账本失败，请稍后重试")
			return
		}
		result, err := insightsCompute(records, filters, month)
		if err != nil {
			statsWriteError(w, 500, err.Error())
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}
func insightsCompute(records []ledger.Record, filters statsFilters, month string) (insightResult, error) {
	totals, err := statsCompute(records, filters)
	if err != nil {
		return insightResult{}, err
	}
	result := insightResult{Totals: totals, Categories: []insightCategory{}, Trend: []insightDay{}, Month: month}
	categories := map[string]int64{}
	days := map[string]*insightDay{}
	for _, record := range records {
		if err := ledger.Validate(record); err != nil {
			return insightResult{}, fmt.Errorf("账本包含无效记录")
		}
		if record.Type == "expense" && len(record.Date) >= 7 && record.Date[:7] == month {
			if result.MonthExpenseCents > math.MaxInt64-record.AmountCents {
				return insightResult{}, fmt.Errorf("月度金额超出可计算范围")
			}
			result.MonthExpenseCents += record.AmountCents
		}
		if !filters.matches(record) {
			continue
		}
		result.Count++
		day := days[record.Date]
		if day == nil {
			day = &insightDay{Date: record.Date}
			days[record.Date] = day
		}
		if record.Type == "income" {
			day.IncomeCents += record.AmountCents
		} else {
			day.ExpenseCents += record.AmountCents
			categories[record.Category] += record.AmountCents
		}
	}
	for category, cents := range categories {
		result.Categories = append(result.Categories, insightCategory{Category: category, ExpenseCents: cents})
	}
	sort.Slice(result.Categories, func(i, j int) bool {
		if result.Categories[i].ExpenseCents == result.Categories[j].ExpenseCents {
			return result.Categories[i].Category < result.Categories[j].Category
		}
		return result.Categories[i].ExpenseCents > result.Categories[j].ExpenseCents
	})
	dates := make([]string, 0, len(days))
	for date := range days {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	if len(dates) > 14 {
		dates = dates[len(dates)-14:]
	}
	for _, date := range dates {
		result.Trend = append(result.Trend, *days[date])
	}
	return result, nil
}
