package event

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONSink_writesOneVersionedEventPerLine(t *testing.T) {
	var output bytes.Buffer
	sink := NewJSONSink(&output)
	if err := sink.Emit(Event{Name: Launched}); err != nil {
		t.Fatalf("Emit(launched) error: %v", err)
	}
	if err := sink.Emit(Event{Name: Waiting, Module: "vorbisFile.dll"}); err != nil {
		t.Fatalf("Emit(waiting) error: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if got, want := len(lines), 2; got != want {
		t.Fatalf("Emit() line count = %d, want %d", got, want)
	}
	for i, line := range lines {
		var got Event
		if err := json.Unmarshal([]byte(line), &got); err != nil {
			t.Errorf("json.Unmarshal(line %d) error: %v", i, err)
			continue
		}
		if got.Version != ProtocolVersion {
			t.Errorf("event %d version = %d, want %d", i, got.Version, ProtocolVersion)
		}
	}
}
