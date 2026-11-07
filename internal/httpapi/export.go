package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"personal-ledger/internal/ledger"
)

// Export downloads a consistent complete snapshot, without applying screen filters.
func Export(listRecords func() ([]ledger.Record, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			createWriteError(w, 405, "请使用 GET 请求")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(query["format"]) > 1 {
			createWriteError(w, 400, "导出格式无效")
			return
		}
		format := query.Get("format")
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "csv" {
			createWriteError(w, 400, "仅支持 JSON 或 CSV 导出")
			return
		}
		if listRecords == nil {
			createWriteError(w, 500, "账本读取服务未配置")
			return
		}
		records, err := listRecords()
		if err != nil {
			createWriteError(w, 500, "读取账本失败，未导出任何数据")
			return
		}
		if records == nil {
			records = []ledger.Record{}
		}
		for _, record := range records {
			if err := ledger.Validate(record); err != nil {
				createWriteError(w, 500, "账本包含无效记录，未导出任何数据")
				return
			}
		}
		var data []byte
		if format == "json" {
			data, err = json.MarshalIndent(records, "", "  ")
			data = append(data, '\n')
		} else {
			var buffer bytes.Buffer
			buffer.WriteString("\xef\xbb\xbf")
			writer := csv.NewWriter(&buffer)
			_ = writer.Write([]string{"日期", "类型", "金额（元）", "类别", "备注"})
			for _, record := range records {
				kind := "支出"
				if record.Type == "income" {
					kind = "收入"
				}
				amount := strconv.FormatInt(record.AmountCents/100, 10) + "." + fmtExportFraction(record.AmountCents%100)
				_ = writer.Write([]string{record.Date, kind, amount, record.Category, exportCSVText(record.Note)})
			}
			writer.Flush()
			err = writer.Error()
			data = buffer.Bytes()
		}
		if err != nil {
			createWriteError(w, 500, "生成导出文件失败")
			return
		}
		contentType := "application/json; charset=utf-8"
		if format == "csv" {
			contentType = "text/csv; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "attachment; filename=\"ledger-"+time.Now().Format("20060102")+"."+format+"\"")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	}
}
func fmtExportFraction(value int64) string {
	text := strconv.FormatInt(value, 10)
	if value < 10 {
		return "0" + text
	}
	return text
}
func exportCSVText(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") {
		return "'" + value
	}
	return value
}
