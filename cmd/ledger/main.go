package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"personal-ledger/internal/httpapi"
	"personal-ledger/internal/ledger"
	"personal-ledger/internal/store"
	webassets "personal-ledger/web"
)

const version = "1.1.0"

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	data := flag.String("data", "data/records.json", "local JSON ledger path")
	demo := flag.Bool("demo", false, "use a separate demo ledger, leaving real records untouched")
	open := flag.Bool("open", false, "open the local app in the default browser")
	flag.Parse()
	dataExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "data" {
			dataExplicit = true
		}
	})
	if *demo && !dataExplicit {
		*data = "data/demo-records.json"
	}
	records := store.NewJSONStore(*data)
	if *demo {
		if err := seedDemo(records, *data, time.Now()); err != nil {
			log.Fatal(err)
		}
	}
	if _, err := records.List(); err != nil {
		log.Printf("账本读取异常，原文件已保留：%v", err)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("启动失败，端口可能已占用，可用 -addr 127.0.0.1:8081：%v", err)
	}
	address := listener.Addr().String()
	host, port, _ := net.SplitHostPort(address)
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	publicURL := "http://" + net.JoinHostPort(host, port)
	server := &http.Server{
		Handler:           newApplication(records, webassets.Files, *demo),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
	}
	log.Printf("个人记账本 v%s  %s", version, publicURL)
	absolute, _ := filepath.Abs(*data)
	log.Printf("账本文件：%s；Ctrl+C 停止服务", absolute)
	if *open {
		go openBrowser(publicURL)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(done)
	go func() {
		<-done
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newApplication(records *store.JSONStore, assets fs.FS, demo bool) http.Handler {
	api := http.NewServeMux()
	api.HandleFunc("POST /api/records", httpapi.Create(records.Append))
	api.HandleFunc("GET /api/records/recent", httpapi.Recent(records.List))
	api.HandleFunc("GET /api/stats", httpapi.Stats(records.List))
	api.HandleFunc("GET /api/insights", httpapi.Insights(records.List))
	api.HandleFunc("GET /api/export", httpapi.Export(records.List))
	api.HandleFunc("GET /api/meta", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"version": version, "demo": demo})
	})
	api.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		methods := map[string]string{"/api/records": "POST", "/api/records/recent": "GET, HEAD", "/api/stats": "GET, HEAD", "/api/insights": "GET, HEAD", "/api/export": "GET", "/api/meta": "GET, HEAD"}
		if allowed, ok := methods[r.URL.Path]; ok {
			w.Header().Set("Allow", allowed)
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "请求方式不匹配"})
			return
		}
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "接口不存在或请求方式不匹配"})
	})
	files := http.FileServer(http.FS(assets))
	root := http.NewServeMux()
	root.Handle("/api/", api)
	root.Handle("/", files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method == http.MethodPost {
			if origin := r.Header.Get("Origin"); origin != "" {
				parsed, err := url.Parse(origin)
				if err != nil || parsed.Host != r.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(403)
					_ = json.NewEncoder(w).Encode(map[string]string{"error": "请从当前记账页面提交请求"})
					return
				}
			}
		}
		root.ServeHTTP(w, r)
	})
}

// Keep the original entry's helper for local compatibility and existing callers.
func newHandler(webDir string) http.Handler {
	return newApplication(store.NewJSONStore("data/records.json"), os.DirFS(webDir), false)
}

func seedDemo(records *store.JSONStore, path string, now time.Time) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entries := []struct {
		days           int
		kind, category string
		cents          int64
		note           string
	}{
		{-7, "income", "工资", 880000, "演示：本月工资"},
		{-6, "expense", "饮食", 2850, "演示：午餐"},
		{-5, "expense", "交通", 1200, "演示：通勤"},
		{-4, "expense", "购物", 15900, "演示：生活用品"},
		{-3, "expense", "饮食", 4200, "演示：周末聚餐"},
		{-2, "income", "奖金", 60000, "演示：项目奖金"},
		{-2, "expense", "交通", 3600, "演示：出行"},
		{-1, "expense", "其他支出", 6800, "演示：宽带费用"},
		{0, "expense", "饮食", 1850, "演示：早餐和咖啡"},
	}
	for _, entry := range entries {
		record := ledger.Record{Date: now.AddDate(0, 0, entry.days).Format(time.DateOnly), Type: entry.kind, AmountCents: entry.cents, Category: entry.category, Note: entry.note}
		if err := records.Append(record); err != nil {
			return fmt.Errorf("创建演示账本：%w", err)
		}
	}
	return nil
}

func openBrowser(address string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", address)
	case "darwin":
		command = exec.Command("open", address)
	default:
		command = exec.Command("xdg-open", address)
	}
	if err := command.Start(); err != nil {
		log.Printf("请在浏览器打开 %s", strings.TrimSpace(address))
		return
	}
	_ = command.Wait()
}
