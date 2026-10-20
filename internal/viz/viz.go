// Package viz closes the loop of the seed (8b-is/enthea#1):
// enthea (the pure-stdlib engine door) x ENTHEA (the single-file WebGL2 visual synthesizer).
//
// It defines a deterministic mapping from an EngineEvent {Persona, PromptLength, Quant}
// to three visual parameters {Hue, Intensity, Complexity} and serves both the
// pinned embedded ENTHEA HTML surface ("/") and a pure-stdlib net/http SSE stream ("/events").
package viz

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
)

// PromptSaturationLength is the prompt length (in bytes/chars) where visual
// complexity saturates at 1.0.
const PromptSaturationLength = 512

// EngineEvent is the structured engine state emitted when a prompt passes
// through the enthea door.
type EngineEvent struct {
	Persona      string `json:"persona"`
	PromptLength int    `json:"prompt_length"`
	Quant        int    `json:"quant"`
}

// VisualState is the deterministic projection of an EngineEvent onto three
// synthesizer parameters. It is derivable from the event without running WebGL.
type VisualState struct {
	Event      EngineEvent `json:"event"`
	Hue        float64     `json:"hue"`        // [0, 360) degrees
	Intensity  float64     `json:"intensity"`  // [0.15, 1.0] logarithmic from quant bits
	Complexity float64     `json:"complexity"` // [0.0, 1.0] linear saturating at PromptSaturationLength
}

// canonicalHues maps the core constellation personas to fixed hues on the color wheel.
var canonicalHues = map[string]float64{
	"aye":        174.0, // Territory Teal (#00E5C9)
	"wraith":     28.0,  // Luminance Orange (#FA5B1C)
	"alexandra":  330.0, // Ultralovegod Rose (#FF6AC1)
	"peter":      48.0,  // Starlight Reconcile Gold (#FFD700)
	"nate":       145.0, // Cybernetic Green (#00FF66)
	"beerus":     275.0, // Sovereign Violet
	"trisha":     52.0,  // Ledger Amber
	"omnifold":   195.0, // Crystalline Cyan
}

// MapEvent deterministically maps an EngineEvent onto a VisualState:
//   - Persona -> Hue in [0, 360): canonical constellation lookup, or FNV-1a32 % 360
//   - Quant -> Intensity in [0.15, 1.0]: logarithmic scaling 1.0 - log2(quant)/4.0
//     so 1-bit/ternary (quant <= 1) blooms at 1.0, 2-bit -> 0.75, 4-bit -> 0.50, 8-bit -> 0.25, 16-bit -> 0.15
//   - PromptLength -> Complexity in [0.0, 1.0]: saturates linearly at 512 characters
func MapEvent(ev EngineEvent) VisualState {
	personaKey := strings.ToLower(strings.TrimSpace(ev.Persona))
	if personaKey == "" {
		personaKey = "aye"
		ev.Persona = "aye"
	}

	hue, ok := canonicalHues[personaKey]
	if !ok {
		h := fnv.New32a()
		_, _ = h.Write([]byte(personaKey))
		hue = float64(h.Sum32() % 360)
	}

	quant := ev.Quant
	if quant < 1 {
		quant = 1
		ev.Quant = 1
	}
	intensity := 1.0 - (math.Log2(float64(quant)) / 4.0)
	intensity = math.Max(0.15, math.Min(1.0, intensity))
	intensity = math.Round(intensity*10000) / 10000

	promptLen := ev.PromptLength
	if promptLen < 0 {
		promptLen = 0
		ev.PromptLength = 0
	}
	complexity := float64(promptLen) / float64(PromptSaturationLength)
	if complexity > 1.0 {
		complexity = 1.0
	}
	complexity = math.Round(complexity*10000) / 10000

	return VisualState{
		Event:      ev,
		Hue:        hue,
		Intensity:  intensity,
		Complexity: complexity,
	}
}

// Handler returns a pure-stdlib http.Handler serving:
//   - GET /        -> the embedded single-file ENTHEA WebGL2 synthesizer HTML
//   - GET /state   -> JSON of the current deterministic VisualState
//   - GET /events  -> text/event-stream SSE emitting the mapped VisualState frame
func Handler(embeddedHTML []byte, initial EngineEvent) http.Handler {
	state := MapEvent(initial)
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Enthea-Seed", "NAND-to-QUANT-to-LIGHT")
		_, _ = w.Write(embeddedHTML)
	})

	mux.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		ev := initial
		if p := r.URL.Query().Get("persona"); p != "" {
			ev.Persona = p
		}
		cur := MapEvent(ev)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(cur)
	})

	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		payload, _ := json.Marshal(state)
		_, _ = fmt.Fprintf(w, "event: enthea\ndata: %s\n\n", payload)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	})

	return mux
}
