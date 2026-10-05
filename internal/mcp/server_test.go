package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type testStream struct {
	*strings.Reader
	bytes.Buffer
	fail error
}

func (s *testStream) Read(p []byte) (int, error) { return s.Reader.Read(p) }
func (s *testStream) Write(p []byte) (int, error) {
	if s.fail != nil {
		return 0, s.fail
	}
	return s.Buffer.Write(p)
}
func (s *testStream) Close() error { return nil }
func TestNotificationsAreSilent(t *testing.T) {
	server, _ := New()
	stream := &testStream{Reader: strings.NewReader("{\"jsonrpc\":\"2.0\",\"method\":\"notifications/initialized\"}\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n")}
	if err := server.Serve(context.Background(), stream); err != nil {
		t.Fatal(err)
	}
	if strings.Count(stream.Buffer.String(), "\n") != 1 {
		t.Fatalf("unexpected responses: %s", stream.Buffer.String())
	}
}
func TestWriteFailuresPropagate(t *testing.T) {
	for _, input := range []string{"{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}\n", "invalid\n"} {
		server, _ := New()
		sentinel := errors.New("writer closed")
		stream := &testStream{Reader: strings.NewReader(input), fail: sentinel}
		if err := server.Serve(context.Background(), stream); !errors.Is(err, sentinel) {
			t.Fatalf("got %v, want write failure", err)
		}
	}
}

func TestResponsePreservesRequestID(t *testing.T) {
	for _, id := range []string{`0`, `"request-one"`, `9007199254740993`, `null`} {
		t.Run(id, func(t *testing.T) {
			server, _ := New()
			stream := &testStream{Reader: strings.NewReader(`{"jsonrpc":"2.0","id":` + id + `,"method":"tools/list"}` + "\n")}
			if err := server.Serve(context.Background(), stream); err != nil {
				t.Fatal(err)
			}
			var response struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
			}
			if err := json.Unmarshal(stream.Buffer.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if string(response.ID) != id || len(response.Result) == 0 {
				t.Fatalf("unexpected response: %s", stream.Buffer.String())
			}
		})
	}
}
