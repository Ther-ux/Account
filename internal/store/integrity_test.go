package store_test

import (
	"bytes"
	"os"
	"path/filepath"
	"personal-ledger/internal/ledger"
	"personal-ledger/internal/store"
	"testing"
)

func TestJSONStoreRejectsSemanticCorruptionAndInvalidAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	records := store.NewJSONStore(path)
	if err := records.Append(ledger.Record{}); err == nil {
		t.Fatal("invalid append accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid append created file")
	}
	for _, body := range []string{`[{}]`, `[{"date":"2026-02-29","type":"income","amountCents":1,"category":"工资"}]`, `[{"date":"2026-11-07","type":"expense","amountCents":0,"category":"饮食"}]`} {
		before := []byte(body)
		if err := os.WriteFile(path, before, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := records.List(); err == nil {
			t.Fatal("semantic corruption accepted")
		}
		if err := records.Append(storeRecord("保留原账本")); err == nil {
			t.Fatal("corrupt file overwritten")
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("original bytes changed")
		}
	}
}
