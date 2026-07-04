package agent

import "testing"

func TestEmitterSubscribe(t *testing.T) {
	e := NewEmitter()
	var got []EventType
	unsub := e.Subscribe(func(ev Event) {
		got = append(got, ev.Type)
	})
	e.Emit(Event{Type: EventRunStart})
	e.Emit(Event{Type: EventRunEnd})
	unsub()
	e.Emit(Event{Type: EventWarn, Text: "after unsub"})

	want := []EventType{EventRunStart, EventRunEnd}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestEmitterNilSafe(t *testing.T) {
	var e *Emitter
	e.Emit(Event{Type: EventRunStart})
	unsub := e.Subscribe(func(Event) {})
	unsub()
}
