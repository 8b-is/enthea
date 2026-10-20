package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMCPStdoutContainsOnlyResponse(t *testing.T) {
	dir := t.TempDir()
	in, err := os.Create(filepath.Join(dir, "input"))
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	if _, err = in.WriteString("{\"jsonrpc\":\"2.0\",\"id\":0,\"method\":\"initialize\"}\n"); err != nil {
		t.Fatal(err)
	}
	if _, err = in.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = in, out
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
	if err = runMCP(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		t.Fatalf("stdout not a single JSON response: %q: %v", data, err)
	}
	if response.JSONRPC != "2.0" || response.ID != 0 || len(response.Result) == 0 {
		t.Fatalf("unexpected response: %s", data)
	}
}

func TestEmbeddedSynthesizerAndVizOnce(t *testing.T) {
	if len(embeddedSynthesizerHTML) < 100000 {
		t.Fatalf("expected embedded index.html > 100KB, got %d bytes", len(embeddedSynthesizerHTML))
	}
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "viz.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	oldOut := os.Stdout
	os.Stdout = out
	defer func() { os.Stdout = oldOut }()

	if err := runViz(context.Background(), []string{"--once", "--persona", "wraith", "--prompt-length", "256", "--quant", "2"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid json from runViz --once: %v", err)
	}
	if parsed["hue"] != 28.0 || parsed["intensity"] != 0.75 || parsed["complexity"] != 0.5 {
		t.Fatalf("unexpected state: %+v", parsed)
	}
}
