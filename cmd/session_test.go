package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mars/marspi-cli/internal/config"
	"github.com/mars/marspi-cli/internal/session"
	"github.com/mars/marspi-cli/internal/ui"
	"github.com/mars/marspi-core/agent"
	"github.com/mars/marspi-core/llm"
	"github.com/mars/marspi-core/tool"
)

func sessionTestApp(endpoint string) *App {
	provider := llm.NewProvider("test", endpoint, "test-key")
	registry := tool.NewEmptyRegistry(nil)
	printer := ui.NewPrinter()
	printer.SetHooks(&ui.Hooks{Silent: true})
	return &App{
		cfg:      &config.Config{MaxContext: 100_000},
		provider: provider,
		registry: registry,
		console:  printer,
		runner: &agent.Runner{
			Provider: provider,
			Registry: registry,
			Events:   agent.NewEmitter(),
			MaxIter:  3,
		},
	}
}

func TestRunSessionResumesHistoryAndClearsInjections(t *testing.T) {
	requests := make(chan []llm.Message, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var body struct {
			Messages []llm.Message `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		requests <- body.Messages
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message":       llm.Message{"role": "assistant", "content": "answer"},
			}},
		})
	}))
	defer server.Close()
	app := sessionTestApp(server.URL)
	file := session.File{Path: filepath.Join(t.TempDir(), "session.json")}
	if err := file.Save([]llm.Message{{"role": "system", "content": "old rules"}, {"role": "user", "content": "previous question"}}); err != nil {
		t.Fatal(err)
	}
	history, err := app.newSession(file.Path, "new rules")
	if err != nil {
		t.Fatal(err)
	}
	history.InjectUser("temporary instructions")
	if err := app.runSession(context.Background(), history, file.Path, "new question"); err != nil {
		t.Fatal(err)
	}
	first := <-requests
	if first[0]["content"] != "old rules" || first[len(first)-1]["content"] != "temporary instructions" {
		t.Fatalf("request lost history or injection: %v", first)
	}
	resumed, err := app.newSession(file.Path, "unused rules")
	if err != nil || resumed.Len() != 4 || resumed.Messages[3]["content"] != "answer" {
		t.Fatalf("session not saved: history=%v err=%v", resumed, err)
	}
	if err := app.runSession(context.Background(), history, file.Path, "follow up"); err != nil {
		t.Fatal(err)
	}
	for _, message := range <-requests {
		if message["content"] == "temporary instructions" {
			t.Fatal("runtime injection leaked into the next run")
		}
	}
}

func TestRunSessionSavesAfterInFlightCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(started)
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	app := sessionTestApp(server.URL)
	path := filepath.Join(t.TempDir(), "session.json")
	history, err := app.newSession(path, "rules")
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.runSession(runCtx, history, path, "interrupted question") }()
	select {
	case <-started:
		cancel()
	case <-runCtx.Done():
		t.Fatal("model request did not start")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	resumed, err := app.newSession(path, "rules")
	if err != nil || resumed.Len() != 2 || resumed.Messages[1]["content"] != "interrupted question" {
		t.Fatalf("canceled run was not saved: history=%v err=%v", resumed, err)
	}
}

func TestRunSessionPreservesRunAndSaveErrors(t *testing.T) {
	app := sessionTestApp("http://unused.invalid")
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	history, err := app.newSession(filepath.Join(t.TempDir(), "session.json"), "rules")
	if err != nil {
		t.Fatal(err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = app.runSession(runCtx, history, path, "question")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "save session") {
		t.Fatalf("run or save failure lost: %v", err)
	}
}

func TestNewSessionCommandPersistsResetAndBacksUpHistory(t *testing.T) {
	app := sessionTestApp("http://unused.invalid")
	path := filepath.Join(t.TempDir(), "session.json")
	history, err := app.newSession(path, "rules")
	if err != nil {
		t.Fatal(err)
	}
	history.AppendUser("previous question")
	if err := (session.File{Path: path}).Save(history.Messages); err != nil {
		t.Fatal(err)
	}
	handled, quit := app.handleCommand("/new", history, path, "new rules")
	if !handled || quit {
		t.Fatalf("handled=%v quit=%v", handled, quit)
	}
	resumed, err := app.newSession(path, "unused rules")
	if err != nil || resumed.Len() != 1 || resumed.Messages[0]["content"] != "new rules" {
		t.Fatalf("reset was not persisted: history=%v err=%v", resumed, err)
	}
	backups, err := filepath.Glob(path + ".*.backup")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups=%v err=%v", backups, err)
	}
	previous, err := (session.File{Path: backups[0]}).Load()
	if err != nil || len(previous) != 2 {
		t.Fatalf("previous session was not backed up: %v %v", previous, err)
	}
}
