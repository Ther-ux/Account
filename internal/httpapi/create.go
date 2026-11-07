package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"personal-ledger/internal/ledger"
)

// Create validates a single JSON record and saves it through appendRecord.
func Create(appendRecord func(ledger.Record) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			createWriteError(w, 405, "请使用 POST 请求")
			return
		}
		if appendRecord == nil {
			createWriteError(w, 500, "账本保存服务未配置")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		// A pointer distinguishes a top-level null from a record object.
		var record *ledger.Record
		if err := decoder.Decode(&record); err != nil {
			if err == io.EOF {
				createWriteError(w, http.StatusBadRequest, "请求体不能为空")
			} else {
				createWriteError(w, http.StatusBadRequest, "无效的账目 JSON: "+err.Error())
			}
			return
		}
		if record == nil {
			createWriteError(w, http.StatusBadRequest, "账目必须是 JSON 对象，不能为 null")
			return
		}

		// Only whitespace may follow the first JSON document.
		var trailing json.RawMessage
		if err := decoder.Decode(&trailing); err != io.EOF {
			if err == nil {
				createWriteError(w, http.StatusBadRequest, "请求体只能包含一个 JSON 对象")
			} else {
				createWriteError(w, http.StatusBadRequest, "无效的 JSON 尾随内容: "+err.Error())
			}
			return
		}
		if err := ledger.Validate(*record); err != nil {
			createWriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := appendRecord(*record); err != nil {
			createWriteError(w, http.StatusInternalServerError, "保存账目失败，请检查磁盘空间或文件权限后重试")
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(*record)
	}
}

func createWriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
