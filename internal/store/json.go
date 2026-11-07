// Package store persists ledger records in a local JSON file.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"personal-ledger/internal/ledger"
)

// JSONStore serializes file access through a single shared instance.
// Separate instances and separate processes do not share this lock.
type JSONStore struct {
	path string
	mu   sync.Mutex
}

// NewJSONStore saves the path without opening or creating any files.
// File errors are reported by List and Append.
func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

// Append validates and saves a record after all existing records.
func (s *JSONStore) Append(record ledger.Record) error {
	if err := ledger.Validate(record); err != nil {
		return fmt.Errorf("校验账目: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	records, err := s.readRecords()
	if err != nil {
		return err
	}
	return s.writeRecords(append(records, record))
}

// List returns an independent slice in file order, oldest insertion first.
// A missing file is an empty ledger; an existing unreadable or corrupt file
// is an error and is never reset automatically.
func (s *JSONStore) List() ([]ledger.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readRecords()
}

// readRecords is called only while s.mu is held, including from Append.
func (s *JSONStore) readRecords() ([]ledger.Record, error) {
	if s.path == "" {
		return nil, errors.New("账本文件路径不能为空")
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []ledger.Record{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取账本 %q: %w", s.path, err)
	}

	// Pointers also distinguish a null element from a record object.
	var decoded []*ledger.Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("解析账本 %q: %w", s.path, err)
	}
	if decoded == nil {
		return nil, fmt.Errorf("解析账本 %q: 顶层必须为 Record 数组，不能为 null", s.path)
	}
	records := make([]ledger.Record, len(decoded))
	for i, record := range decoded {
		if record == nil {
			return nil, fmt.Errorf("解析账本 %q: 第 %d 条记录不能为 null", s.path, i+1)
		}
		if err := ledger.Validate(*record); err != nil {
			return nil, fmt.Errorf("账本第 %d 条记录无效: %w", i+1, err)
		}
		records[i] = *record
	}
	return records, nil
}

// writeRecords is called only while s.mu is held. It never truncates or
// removes the old ledger before replacement succeeds.
func (s *JSONStore) writeRecords(records []ledger.Record) error {
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("编码账本 %q: %w", s.path, err)
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建账本目录 %q: %w", dir, err)
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(s.path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时账本: %w", err)
	}
	tempPath := temp.Name()
	// Removal is best effort on failure; after a successful rename the
	// temporary path no longer exists. The original ledger is never removed.
	defer os.Remove(tempPath)

	n, writeErr := temp.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = temp.Sync()
	}
	// Close even after Write or Sync fails, before attempting cleanup.
	closeErr := temp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("写入或关闭临时账本 %q: %w", tempPath, err)
	}
	// Close first for Windows compatibility. Rename replaces an existing
	// regular file; no delete-and-retry fallback is used on failure.
	// Go does not guarantee an atomic rename on non-Unix platforms.
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("替换账本 %q: %w", s.path, err)
	}
	return nil
}
