package viz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMapEventDeterministic(t *testing.T) {
	s1 := MapEvent(EngineEvent{Persona: "aye", PromptLength: 256, Quant: 2})
	if s1.Hue != 174.0 {
		t.Fatalf("got hue %v, want 174.0", s1.Hue)
	}
	if s1.Intensity != 0.75 {
		t.Fatalf("got intensity %v, want 0.75 for 2-bit quant", s1.Intensity)
	}
	if s1.Complexity != 0.5 {
		t.Fatalf("got complexity %v, want 0.5 for 256/512 prompt length", s1.Complexity)
	}

	// Saturation at PromptSaturationLength (512) and floor at 16-bit quant
	s2 := MapEvent(EngineEvent{Persona: "wraith", PromptLength: 2048, Quant: 16})
	if s2.Hue != 28.0 {
		t.Fatalf("got hue %v, want 28.0", s2.Hue)
	}
	if s2.Intensity != 0.15 {
		t.Fatalf("got intensity %v, want floor 0.15 for 16-bit quant", s2.Intensity)
	}
	if s2.Complexity != 1.0 {
		t.Fatalf("got complexity %v, want saturated 1.0", s2.Complexity)
	}

	// Arbitrary persona hashes deterministically
	a := MapEvent(EngineEvent{Persona: "custom-voice", PromptLength: 128, Quant: 1})
	b := MapEvent(EngineEvent{Persona: "custom-voice", PromptLength: 128, Quant: 1})
	if a != b || a.Intensity != 1.0 {
		t.Fatalf("non-deterministic or unexpected mapping: %+v vs %+v", a, b)
	}
}

func TestHandlerServesEmbeddedHTMLAndSSE(t *testing.T) {
	html := []byte("<!doctype html><title>ENTHEA</title>")
	h := Handler(html, EngineEvent{Persona: "alexandra", PromptLength: 512, Quant: 1})

	// 1. Root HTML
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	h.ServeHTTP(recRoot, reqRoot)
	if recRoot.Code != http.StatusOK || recRoot.Body.String() != string(html) {
		t.Fatalf("unexpected root response: %d %q", recRoot.Code, recRoot.Body.String())
	}

	// 2. SSE /events
	reqSSE := httptest.NewRequest(http.MethodGet, "/events", nil)
	recSSE := httptest.NewRecorder()
	h.ServeHTTP(recSSE, reqSSE)
	if !strings.HasPrefix(recSSE.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("unexpected content-type: %s", recSSE.Header().Get("Content-Type"))
	}
	body := recSSE.Body.String()
	if !strings.Contains(body, "event: enthea\ndata: ") {
		t.Fatalf("missing SSE frame in %q", body)
	}
	dataLine := strings.TrimPrefix(strings.Split(strings.TrimSpace(body), "\n")[1], "data: ")
	var state VisualState
	if err := json.Unmarshal([]byte(dataLine), &state); err != nil {
		t.Fatalf("invalid JSON in SSE data: %v", err)
	}
	if state.Hue != 330.0 || state.Intensity != 1.0 || state.Complexity != 1.0 {
		t.Fatalf("unexpected SSE state: %+v", state)
	}
}
