package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/codex2api/auth"
	"github.com/codex2api/config"
	"github.com/codex2api/database"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestAntigravityMessagesPreserveSystemContextOnWire(t *testing.T) {
	raw := []byte(`{
		"model":"claude-opus-5-5", "max_tokens":128, "output_config":{"effort":"medium"},
		"system":[{"type":"text","text":"<system-reminder># Environment\nKeep the project working directory.</system-reminder>"}],
		"messages":[
			{"role":"user","content":"What skills are available?"},
			{"role":"system","content":"You are powered by the model named Opus 5.5.\nThe following skills are available for use with the Skill tool:\n- example-skill: inspect project documents."},
			{"role":"system","content":[{"type":"text","text":"<system-reminder>Available agent types for the Agent tool:\n- example-agent: inspect project files.</system-reminder>"}]}
		]
	}`)
	checkAntigravityMessagesSystemOnWire(t, raw)
}

// Replay a local request without sending its contents or credentials to a server.
func TestAntigravityMessagesLocalReplay(t *testing.T) {
	path := os.Getenv("ANTIGRAVITY_TEST_MESSAGES_FILE")
	if path == "" {
		t.Skip("set ANTIGRAVITY_TEST_MESSAGES_FILE to replay a local request")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	checkAntigravityMessagesSystemOnWire(t, raw)
}

func checkAntigravityMessagesSystemOnWire(t *testing.T, raw []byte) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv(clientScaffoldingStripEnv, "true")
	t.Setenv(clientScaffoldingStripRemindersEnv, "true")
	t.Setenv(antigravitySensitivePhrasesEnv, "")
	var original anthropicRequest
	if err := json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	for _, enterprise := range []bool{false, true} {
		name := "personal"
		if enterprise {
			name = "enterprise"
		}
		t.Run(name, func(t *testing.T) {
			store := auth.NewStore(nil, nil, &database.SystemSettings{MaxConcurrency: 1, MaxRetries: 1})
			t.Cleanup(store.Stop)
			account := directModelTestAntigravityAccount("claude-opus-5-5-low", "claude-opus-5-5-medium", "claude-opus-5-5-high")
			account.AntigravityProjectID = "test-project"
			if enterprise {
				account.AntigravityOAuthClientKey = auth.AntigravityEnterpriseOAuthClientKey
			}
			store.AddAccount(account)
			var sent []byte
			key := antigravityHTTPClientKey{accountID: account.ID()}
			client := &http.Client{Transport: claudeBoundaryRoundTripper(func(r *http.Request) (*http.Response, error) {
				var err error
				sent, err = io.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}]}\n\n"))}, nil
			})}
			previous, existed := antigravityHTTPClients.Load(key)
			antigravityHTTPClients.Store(key, client)
			t.Cleanup(func() {
				if existed {
					antigravityHTTPClients.Store(key, previous)
				} else {
					antigravityHTTPClients.Delete(key)
				}
			})
			handler := NewHandler(store, nil, &config.Config{AllowAnonymousV1: true}, nil)
			recorder := invokeAntigravityTransport(t, handler, "/v1/messages", string(raw), (*Handler).Messages)
			if recorder.Code != 200 || len(sent) == 0 {
				t.Fatalf("status=%d, request captured=%v, response=%s", recorder.Code, len(sent) > 0, recorder.Body.String())
			}
			request := gjson.ParseBytes(sent)
			if !enterprise {
				request = request.Get("request")
			}
			text := request.Get("systemInstruction.parts.0.text").String()
			text = strings.ReplaceAll(text, antigravityZeroWidthSpace, "")
			var expectedParts []string
			if top := parseAnthropicSystem(original.System); top != "" {
				expectedParts = append(expectedParts, top)
			}
			for i, message := range original.Messages {
				if message.Role != "system" {
					continue
				}
				for j, block := range parseAnthropicContent(message.Content) {
					if block.Type != "text" {
						continue
					}
					if strings.TrimSpace(block.Text) == "" {
						continue
					}
					expectedParts = append(expectedParts, block.Text)
					present := strings.Contains(text, strings.TrimSpace(block.Text))
					t.Logf("messages[%d].content[%d]: input_bytes=%d, full_text_present=%v", i, j, len(block.Text), present)
					if !present {
						t.Errorf("system message %d block %d lost or rewritten", i, j)
					}
				}
			}
			expected := strings.ReplaceAll(strings.Join(expectedParts, "\n\n"), antigravityZeroWidthSpace, "")
			if text != expected {
				t.Errorf("system content differs: got %d bytes, want %d (ignoring phrase-obfuscation zero-width characters)", len(text), len(expected))
			}
			t.Logf("final system bytes=%d, contains skills=%v", len(text), strings.Contains(text, "The following skills are available"))
		})
	}
}
