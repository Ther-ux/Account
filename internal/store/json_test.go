package store_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"

	"personal-ledger/internal/ledger"
	"personal-ledger/internal/store"
)

func storeRecord(note string) ledger.Record {
	return ledger.Record{
		Date: "2026-11-07", Type: "expense", AmountCents: 1250,
		Category: "饮食", Note: note,
	}
}

func storeReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func storeAssertOnlyLedger(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Errorf("unexpected files or leftover temporary ledger: %v", entries)
	}
}

func TestJSONStoreMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-created", "nested", "records.json")
	s := store.NewJSONStore(path)
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("constructor created directories or unexpected error: %v", err)
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("List = %#v, want non-nil empty slice", got)
	}
	encoded, err := json.Marshal(got)
	if err != nil || string(encoded) != "[]" {
		t.Fatalf("empty ledger JSON = %s, error %v; want []", encoded, err)
	}
	if _, err := os.Stat(filepath.Dir(path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("List created directories or unexpected error: %v", err)
	}
}

func TestJSONStoreAppendAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new", "nested", "records.json")
	s := store.NewJSONStore(path)
	want := []ledger.Record{
		{Date: "2026-11-07", Type: "income", AmountCents: 300000, Category: "工资"},
		{Date: "2026-01-01", Type: "expense", AmountCents: 1250, Category: "饮食", Note: "  补记午饭\n中文备注 🍜  "},
	}
	for i, record := range want {
		if err := ledger.Validate(record); err != nil {
			t.Fatalf("fixture violates the real ledger model: %v", err)
		}
		if err := s.Append(record); err != nil {
			t.Fatal(err)
		}
		got, err := s.List()
		if err != nil || !reflect.DeepEqual(got, want[:i+1]) {
			t.Fatalf("after append %d: got %#v, error %v", i+1, got, err)
		}
	}
	var onDisk []ledger.Record
	if err := json.Unmarshal(storeReadBytes(t, path), &onDisk); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(onDisk, want) {
		t.Fatalf("file records = %#v, want %#v", onDisk, want)
	}
	reopened := store.NewJSONStore(path)
	got, err := reopened.List()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("new instance List = %#v, error %v", got, err)
	}
	storeAssertOnlyLedger(t, path)
}

func TestJSONStoreListReadsFileAndReturnsIndependentSlice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	want := []ledger.Record{storeRecord("先录入"), storeRecord("后补记旧账")}
	want[1].Date = "2024-02-29"
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	data = append([]byte(" \n"), append(data, '\n')...)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	s := store.NewJSONStore(path)
	got, err := s.List()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("List = %#v, error %v", got, err)
	}
	got[0].Note = "调用者修改"
	got[0], got[1] = got[1], got[0]
	again, err := s.List()
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatalf("caller mutated stored records: %#v, error %v", again, err)
	}
	if !bytes.Equal(storeReadBytes(t, path), data) {
		t.Fatal("List modified the file")
	}
	// Editing the file between calls must be observed, rather than returning
	// a cached in-memory ledger.
	if err := os.WriteFile(path, []byte("[]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	again, err = s.List()
	if err != nil || again == nil || len(again) != 0 {
		t.Fatalf("did not observe external file change: %#v, error %v", again, err)
	}
	if err := s.Append(storeRecord("空数组之后")); err != nil {
		t.Fatal(err)
	}
	again, err = s.List()
	if err != nil || !reflect.DeepEqual(again, []ledger.Record{storeRecord("空数组之后")}) {
		t.Fatalf("append to existing empty array: %#v, error %v", again, err)
	}
}

func TestJSONStorePreservesInt64Cents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	record := ledger.Record{Date: "2026-11-07", Type: "income", AmountCents: math.MaxInt64, Category: "奖金", Note: "最大整数分"}
	if err := ledger.Validate(record); err != nil {
		t.Fatal(err)
	}
	if err := store.NewJSONStore(path).Append(record); err != nil {
		t.Fatal(err)
	}
	got, err := store.NewJSONStore(path).List()
	if err != nil || !reflect.DeepEqual(got, []ledger.Record{record}) {
		t.Fatalf("int64 round trip = %#v, error %v", got, err)
	}
	if !bytes.Contains(storeReadBytes(t, path), []byte("9223372036854775807")) {
		t.Fatal("amount was not saved as an exact integer JSON number")
	}
}

func TestJSONStoreCorruptFileIsPreserved(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"zero bytes", ""},
		{"whitespace", " \r\n\t"},
		{"broken JSON", `[{"note":"原始中文"}`},
		{"object", `{}`},
		{"null", `null`},
		{"string", `"不是数组"`},
		{"number", `123`},
		{"boolean", `false`},
		{"null element", `[null]`},
		{"non-record element", `[42]`},
		{"fractional cents", `[{"amountCents":12.5}]`},
		{"string cents", `[{"amountCents":"1250"}]`},
		{"overflow cents", `[{"amountCents":9223372036854775808}]`},
		{"trailing JSON", `[] []`},
		{"trailing garbage", "[]\n原始内容"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "records.json")
			before := []byte(tt.data)
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			s := store.NewJSONStore(path)
			if _, err := s.List(); err == nil {
				t.Fatal("List accepted corrupt data")
			}
			if !bytes.Equal(storeReadBytes(t, path), before) {
				t.Fatal("List changed corrupt data")
			}
			if err := s.Append(storeRecord("不能覆盖损坏账本")); err == nil {
				t.Fatal("Append accepted corrupt data")
			}
			if !bytes.Equal(storeReadBytes(t, path), before) {
				t.Fatal("Append changed corrupt data")
			}
			storeAssertOnlyLedger(t, path)
		})
	}
}

func TestJSONStoreReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(path, "keep.txt")
	if err := os.WriteFile(marker, []byte("保留原内容"), 0600); err != nil {
		t.Fatal(err)
	}
	s := store.NewJSONStore(path)
	if _, err := s.List(); err == nil {
		t.Fatal("List swallowed the directory read error")
	} else {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			t.Fatalf("underlying read error was lost: %v", err)
		}
	}
	if err := s.Append(storeRecord("失败")); err == nil {
		t.Fatal("Append swallowed the read error")
	}
	if string(storeReadBytes(t, marker)) != "保留原内容" {
		t.Fatal("failed append modified the original directory")
	}
}

func TestJSONStoreCannotCreateParent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent-is-a-file")
	before := []byte("不能删除这个文件")
	if err := os.WriteFile(parent, before, 0600); err != nil {
		t.Fatal(err)
	}
	s := store.NewJSONStore(filepath.Join(parent, "records.json"))
	if err := s.Append(storeRecord("无法创建目录")); err == nil {
		t.Fatal("Append swallowed the filesystem error")
	}
	if !bytes.Equal(storeReadBytes(t, parent), before) {
		t.Fatal("Append changed the blocking parent file")
	}
}

func TestJSONStoreWindowsReplacementFailurePreservesFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("a read-only target blocks replacement on Windows")
	}
	path := filepath.Join(t.TempDir(), "records.json")
	s := store.NewJSONStore(path)
	first := storeRecord("原有账目")
	if err := s.Append(first); err != nil {
		t.Fatal(err)
	}
	before := storeReadBytes(t, path)
	if err := os.Chmod(path, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(path, 0600); err != nil {
			t.Error(err)
		}
	})
	err := s.Append(storeRecord("替换应失败"))
	if err == nil {
		t.Fatal("expected replacement of a read-only Windows file to fail")
	}
	var linkErr *os.LinkError
	if !errors.As(err, &linkErr) || linkErr.Op != "rename" {
		t.Fatalf("wanted a propagated rename error, got %v", err)
	}
	if !bytes.Equal(storeReadBytes(t, path), before) {
		t.Fatal("replacement failure destroyed the original bytes")
	}
	storeAssertOnlyLedger(t, path)
	// A failed save must release the lock and leave the store usable.
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	second := storeRecord("恢复可写后追加")
	if err := s.Append(second); err != nil {
		t.Fatal(err)
	}
	got, err := s.List()
	if err != nil || !reflect.DeepEqual(got, []ledger.Record{first, second}) {
		t.Fatalf("store did not recover after failure: %#v, error %v", got, err)
	}
	storeAssertOnlyLedger(t, path)
}

func TestJSONStoreEmptyPath(t *testing.T) {
	s := store.NewJSONStore("")
	if _, err := s.List(); err == nil {
		t.Fatal("List accepted an empty path")
	}
	if err := s.Append(storeRecord("无效路径")); err == nil {
		t.Fatal("Append accepted an empty path")
	}
}

func TestJSONStoreConcurrentAppendsAndLists(t *testing.T) {
	const writers = 100
	const readers = 8
	path := filepath.Join(t.TempDir(), "records.json")
	s := store.NewJSONStore(path)
	start := make(chan struct{})
	results := make(chan error, writers+readers)
	var workers sync.WaitGroup
	for i := 0; i < writers; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			results <- s.Append(storeRecord(fmt.Sprintf("并发账目-%03d", i)))
		}(i)
	}
	for i := 0; i < readers; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for j := 0; j < 10; j++ {
				records, err := s.List()
				if err != nil {
					results <- err
					return
				}
				if records == nil || len(records) > writers {
					results <- fmt.Errorf("invalid concurrent snapshot: %#v", records)
					return
				}
			}
			results <- nil
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Errorf("concurrent operation: %v", err)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	got, err := store.NewJSONStore(path).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != writers {
		t.Fatalf("got %d records after %d successful appends", len(got), writers)
	}
	want := make(map[string]ledger.Record, writers)
	for i := 0; i < writers; i++ {
		record := storeRecord(fmt.Sprintf("并发账目-%03d", i))
		want[record.Note] = record
	}
	for _, record := range got {
		expected, ok := want[record.Note]
		if !ok || record != expected {
			t.Fatalf("unexpected, duplicate, or changed record: %#v", record)
		}
		delete(want, record.Note)
	}
	if len(want) != 0 {
		t.Fatalf("missing records: %#v", want)
	}
	storeAssertOnlyLedger(t, path)
}
