package agentctx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	msgs := []map[string]any{{"role": "user", "content": "hi"}}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("content mismatch")
	}
	if _, err := os.Stat(path + sessionTmpSuffix); !os.IsNotExist(err) {
		t.Fatalf("tmp file should be removed, err=%v", err)
	}
}

func TestLoadRecoversFromTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	valid := []byte(`[{"role":"user","content":"from tmp"}]`)
	if err := os.WriteFile(path+".tmp", valid, 0o644); err != nil {
		t.Fatal(err)
	}
	// 模拟崩溃：主文件不存在或损坏
	_ = os.WriteFile(path, []byte("{broken"), 0o644)

	m := New(1000, nil, nil, nil)
	m.Load(path)
	if m.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", m.Len())
	}
	if m.Messages[0]["content"] != "from tmp" {
		t.Fatalf("unexpected content: %v", m.Messages[0]["content"])
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp should be promoted and removed")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	m := New(1000, nil, nil, nil)
	m.AppendUser("hello")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}

	m2 := New(1000, nil, nil, nil)
	m2.Load(path)
	if m2.Len() != 1 {
		t.Fatalf("expected 1 message, got %d", m2.Len())
	}
}
