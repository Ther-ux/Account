package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"personal-ledger/internal/ledger"
)

// Recent returns records in reverse insertion order. listRecords must return
// records from oldest insertion to newest insertion.
func Recent(listRecords func() ([]ledger.Record, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			recentWriteError(w, 405, "请使用 GET 请求")
			return
		}
		if listRecords == nil {
			recentWriteError(w, 500, "账本读取服务未配置")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			recentWriteError(w, http.StatusBadRequest, "查询参数格式无效")
			return
		}

		limit := 10
		if values, provided := query["limit"]; provided {
			if len(values) != 1 {
				recentWriteError(w, http.StatusBadRequest, "limit 必须是单个 1～100 的整数")
				return
			}
			for _, digit := range values[0] {
				if digit < '0' || digit > '9' {
					recentWriteError(w, 400, "limit 必须为 1～100 的整数")
					return
				}
			}
			limit, err = strconv.Atoi(values[0])
			if err != nil || limit < 1 || limit > 100 {
				recentWriteError(w, http.StatusBadRequest, "limit 必须是 1～100 的整数")
				return
			}
		}

		records, err := listRecords()
		if err != nil {
			recentWriteError(w, http.StatusInternalServerError, "读取账目失败")
			return
		}
		if limit > len(records) {
			limit = len(records)
		}

		// Allocate even for an empty ledger so JSON encodes [] instead of null.
		// Copy backwards without changing the slice supplied by the store.
		result := make([]ledger.Record, limit)
		for i := range result {
			result[i] = records[len(records)-1-i]
		}
		_ = json.NewEncoder(w).Encode(result)
	}
}

func recentWriteError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
