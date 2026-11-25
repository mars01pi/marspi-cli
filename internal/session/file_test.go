package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mars/marspi-core/agentctx"
	"github.com/mars/marspi-core/llm"
)

func TestLegacySessionCompatibility(t *testing.T) {
	file := File{Path: filepath.Join(t.TempDir(), "session.json")}
	legacy := agentctx.New(100_000, nil, nil, nil)
	legacy.AppendSystem("existing rules")
	legacy.AppendUser("previous question")
	if err := legacy.Save(file.Path); err != nil {
		t.Fatal(err)
	}
	messages, err := file.Load()
	if err != nil || len(messages) != 2 || messages[1]["content"] != "previous question" {
		t.Fatalf("messages=%v err=%v", messages, err)
	}
	messages = append(messages, llm.Message{"role": "assistant", "content": "new answer"})
	if err := file.Save(messages); err != nil {
		t.Fatal(err)
	}
	resumed := agentctx.New(100_000, nil, nil, nil)
	resumed.Load(file.Path)
	if resumed.Len() != 3 || resumed.Messages[2]["content"] != "new answer" {
		t.Fatalf("legacy reader cannot resume saved messages: %v", resumed.Messages)
	}
}

func TestRecoverTemporarySession(t *testing.T) {
	for _, primary := range []string{"missing", "broken", "wrong_shape"} {
		t.Run(primary, func(t *testing.T) {
			file := File{Path: filepath.Join(t.TempDir(), "session.json")}
			if primary != "missing" {
				data := "{broken"
				if primary == "wrong_shape" {
					data = `{ "role": "user" }`
				}
				if err := os.WriteFile(file.Path, []byte(data), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(file.Path+".tmp", []byte(`[{"role":"user","content":"recovered"}]`), 0o644); err != nil {
				t.Fatal(err)
			}
			messages, err := file.Load()
			if err != nil || len(messages) != 1 || messages[0]["content"] != "recovered" {
				t.Fatalf("messages=%v err=%v", messages, err)
			}
			if _, err := os.Stat(file.Path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary file was not promoted: %v", err)
			}
			if _, err := file.Load(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCorruptSessionIsPreserved(t *testing.T) {
	file := File{Path: filepath.Join(t.TempDir(), "session.json")}
	corrupt := []byte("{broken")
	if err := os.WriteFile(file.Path, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Load(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("expected corruption warning, got %v", err)
	}
	backups, err := filepath.Glob(file.Path + ".*.backup")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil || string(data) != string(corrupt) {
		t.Fatalf("backup=%s err=%v", data, err)
	}
	if messages, err := file.Load(); err != nil || messages != nil {
		t.Fatalf("fresh session: %v %v", messages, err)
	}
}

func TestSaveFailurePreservesPreviousSession(t *testing.T) {
	file := File{Path: filepath.Join(t.TempDir(), "session.json")}
	if err := file.Save([]llm.Message{{"role": "user", "content": "keep me"}}); err != nil {
		t.Fatal(err)
	}
	if err := file.Save([]llm.Message{{"role": "user", "content": make(chan string)}}); err == nil {
		t.Fatal("unserializable content must fail")
	}
	messages, err := file.Load()
	if err != nil || len(messages) != 1 || messages[0]["content"] != "keep me" {
		t.Fatalf("previous session changed: messages=%v err=%v", messages, err)
	}
}

func TestLoadDoesNotReplaceUnreadablePath(t *testing.T) {
	file := File{Path: filepath.Join(t.TempDir(), "session.json")}
	if err := os.Mkdir(file.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file.Path+".tmp", []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Load(); err == nil || errors.Is(err, ErrCorrupt) {
		t.Fatalf("expected filesystem error, got %v", err)
	}
	info, err := os.Stat(file.Path)
	if err != nil || !info.IsDir() {
		t.Fatalf("unreadable path was changed: %v", err)
	}
}
