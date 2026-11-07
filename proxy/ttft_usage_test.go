package proxy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codex2api/auth"
	"github.com/codex2api/config"
	"github.com/codex2api/database"
	"github.com/gin-gonic/gin"
)

// Exercise HTTP ingress with WS upstream, including the persisted usage metric.
// The structural frames must not end TTFT before delayed content arrives.
func TestWSUpstreamUsageWaitsForContent(t *testing.T) {
	for _, endpoint := range []string{"/v1/responses", "/v1/chat/completions", "/v1/messages"} {
		for _, stream := range []bool{true, false} {
			name := endpoint + "/stream"
			if !stream {
				name = endpoint + "/nonstream"
			}
			t.Run(name, func(t *testing.T) {
				previous := CurrentRuntimeSettings()
				next := previous
				next.CodexForceWebsocket = true
				next.FirstTokenExcludesWsAcquire = false
				next.CodexContinueThinking = false
				ApplyRuntimeSettings(next)
				t.Cleanup(func() { ApplyRuntimeSettings(previous) })
				oldExec := WebsocketExecuteFunc
				t.Cleanup(func() { WebsocketExecuteFunc = oldExec })
				called := false
				WebsocketExecuteFunc = func(ctx context.Context, account *auth.Account, body []byte, sessionID, proxyURL, apiKey string, cfg *DeviceProfileConfig, headers http.Header, route string) (*http.Response, error) {
					called = true
					reader, writer := io.Pipe()
					go func() {
						defer writer.Close()
						io.WriteString(writer, "data: "+`{"type":"response.created","response":{"id":"resp_ttft"}}`+"\n\n")
						io.WriteString(writer, "data: "+`{"type":"codex.rate_limits"}`+"\n\n")
						io.WriteString(writer, "data: "+`{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_ttft","type":"message","role":"assistant","content":[]}}`+"\n\n")
						io.WriteString(writer, "data: "+`{"type":"response.content_part.added","output_index":0,"content_index":0,"part":{"type":"output_text","text":""}}`+"\n\n")
						io.WriteString(writer, "data: "+`{"type":"response.output_text.delta","delta":""}`+"\n\n")
						time.Sleep(150 * time.Millisecond)
						io.WriteString(writer, "data: "+`{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`+"\n\n")
						time.Sleep(150 * time.Millisecond)
						io.WriteString(writer, "data: "+`{"type":"response.completed","response":{"id":"resp_ttft","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`+"\n\n")
					}()
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}, nil
				}
				db, err := database.New("sqlite", filepath.Join(t.TempDir(), "usage.db"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { db.Close() })
				db.SetUsageLogConfig(database.UsageLogModeFull, 100, 60)
				store := auth.NewStore(nil, nil, &database.SystemSettings{MaxConcurrency: 2, MaxRetries: 0})
				t.Cleanup(store.Stop)
				store.AddAccount(&auth.Account{DBID: 1, AccessToken: "test", PlanType: "pro", AccountID: "test-ttft"})
				h := NewHandler(store, db, &config.Config{AllowAnonymousV1: true}, nil)
				r := gin.New()
				r.POST("/v1/responses", h.Responses)
				r.POST("/v1/chat/completions", h.ChatCompletions)
				r.POST("/v1/messages", h.Messages)
				streamFlag := "true"
				if !stream {
					streamFlag = "false"
				}
				body := `{"model":"gpt-5.5","input":"hello","max_tokens":128,"messages":[{"role":"user","content":"hello"}],"stream":` + streamFlag + `}`
				req := httptest.NewRequest("POST", endpoint, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, req)
				if !called || rec.Code != 200 {
					t.Fatalf("WS called=%v status=%d body=%s", called, rec.Code, rec.Body.String())
				}
				entry := assertGeminiStreamUsageStatus(t, db, 200, stream)
				if entry.FirstTokenMs < 150 || entry.DurationMs-entry.FirstTokenMs < 140 {
					t.Fatalf("first_token_ms=%d duration_ms=%d; want delayed first content, before completion", entry.FirstTokenMs, entry.DurationMs)
				}
			})
		}
	}
}
