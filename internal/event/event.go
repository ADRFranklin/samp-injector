// Package event defines the machine-readable injector lifecycle protocol.
package event

import (
	"encoding/json"
	"io"
)

const (
	// ProtocolVersion is the JSON-lines event protocol version.
	ProtocolVersion = 1

	Launched   = "launched"
	Waiting    = "waiting"
	Injecting  = "injecting"
	Injected   = "injected"
	GameExited = "game-exited"
	Error      = "error"

	PhaseArguments = "arguments"
	PhaseLifecycle = "lifecycle"
	PhaseReadiness = "readiness"
	PhaseInjection = "injection"
	PhaseRuntime   = "runtime"
)

// Event is one versioned lifecycle record written in JSON-lines mode.
type Event struct {
	Version int    `json:"version"`
	Name    string `json:"event"`
	Module  string `json:"module,omitempty"`
	Code    *int   `json:"code,omitempty"`
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

// Sink receives lifecycle events.
type Sink interface {
	Emit(Event) error
}

type jsonSink struct {
	encoder *json.Encoder
}

// NewJSONSink returns a sink that writes one JSON object per line.
func NewJSONSink(writer io.Writer) Sink {
	return &jsonSink{encoder: json.NewEncoder(writer)}
}

func (s *jsonSink) Emit(record Event) error {
	record.Version = ProtocolVersion
	return s.encoder.Encode(record)
}

// DiscardSink ignores lifecycle events.
type DiscardSink struct{}

// Emit implements Sink.
func (DiscardSink) Emit(Event) error { return nil }
