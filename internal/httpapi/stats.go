package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"personal-ledger/internal/ledger"
)

type statsFilters struct {
	from, to, category string
	min, max           int64
	hasMin, hasMax     bool
}

type statsResult struct {
	IncomeCents  int64 `json:"incomeCents"`
	ExpenseCents int64 `json:"expenseCents"`
	BalanceCents int64 `json:"balanceCents"`
}

// Stats returns a handler with the exact callback contract agreed in README.
// It does not read files or depend on a particular store implementation.
func Stats(listRecords func() ([]ledger.Record, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			statsWriteError(w, http.StatusMethodNotAllowed, "请使用 GET 请求")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			statsWriteError(w, http.StatusBadRequest, "查询参数编码无效")
			return
		}
		filters, err := statsParseFilters(query)
		if err != nil {
			statsWriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if listRecords == nil {
			statsWriteError(w, http.StatusInternalServerError, "账本读取服务未配置")
			return
		}
		records, err := listRecords()
		if err != nil {
			statsWriteError(w, http.StatusInternalServerError, "读取账本失败，请稍后重试")
			return
		}
		result, err := statsCompute(records, filters)
		if err != nil {
			statsWriteError(w, 500, err.Error())
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	}
}

func statsParseFilters(values url.Values) (statsFilters, error) {
	var filters statsFilters
	for _, key := range []string{"from", "to", "category", "minCents", "maxCents"} {
		if len(values[key]) > 1 {
			return filters, errors.New("查询参数不能重复：" + key)
		}
	}
	filters.from, filters.to, filters.category = values.Get("from"), values.Get("to"), values.Get("category")
	for _, date := range []string{filters.from, filters.to} {
		if date == "" {
			continue
		}
		parsed, err := time.Parse("2006-01-02", date)
		if err != nil || parsed.Year() < 1 || parsed.Format("2006-01-02") != date {
			return filters, errors.New("日期必须为真实有效的 YYYY-MM-DD")
		}
	}
	if filters.from != "" && filters.to != "" && filters.from > filters.to {
		return filters, errors.New("起始日期不能晚于结束日期")
	}
	if filters.category != "" {
		valid := false
		for _, category := range []string{"工资", "奖金", "其他收入", "饮食", "交通", "购物", "其他支出"} {
			if filters.category == category {
				valid = true
				break
			}
		}
		if !valid {
			return filters, errors.New("类别无效")
		}
	}
	var err error
	if filters.min, filters.hasMin, err = statsParseCents(values.Get("minCents")); err != nil {
		return filters, err
	}
	if filters.max, filters.hasMax, err = statsParseCents(values.Get("maxCents")); err != nil {
		return filters, err
	}
	if filters.hasMin && filters.hasMax && filters.min > filters.max {
		return filters, errors.New("最低金额不能大于最高金额")
	}
	return filters, nil
}

func statsParseCents(text string) (int64, bool, error) {
	if text == "" {
		return 0, false, nil
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			return 0, false, errors.New("金额条件必须为非负整数分")
		}
	}
	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, false, errors.New("金额条件超出可计算范围")
	}
	return value, true, nil
}

func (filters statsFilters) matches(record ledger.Record) bool {
	return (filters.from == "" || record.Date >= filters.from) &&
		(filters.to == "" || record.Date <= filters.to) &&
		(filters.category == "" || record.Category == filters.category) &&
		(!filters.hasMin || record.AmountCents >= filters.min) &&
		(!filters.hasMax || record.AmountCents <= filters.max)
}

func statsWriteError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func statsCompute(records []ledger.Record, filters statsFilters) (statsResult, error) {
	result := statsResult{}
	for _, record := range records {
		if !filters.matches(record) {
			continue
		}
		if record.AmountCents <= 0 {
			return statsResult{}, errors.New("账本包含无效金额")
		}
		var total *int64
		switch record.Type {
		case "income":
			total = &result.IncomeCents
		case "expense":
			total = &result.ExpenseCents
		default:
			return statsResult{}, errors.New("账本包含无效收支类型")
		}
		if *total > math.MaxInt64-record.AmountCents {
			return statsResult{}, errors.New("统计金额超出可计算范围")
		}
		*total += record.AmountCents
	}
	result.BalanceCents = result.IncomeCents - result.ExpenseCents
	return result, nil
}
