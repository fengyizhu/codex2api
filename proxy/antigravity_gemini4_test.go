package proxy

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestAntigravityGemini4NativeBaseModelsSelectThinkingBacking(t *testing.T) {
	for _, family := range []string{"gemini-4-carbon", "gemini-4-hydrogen"} {
		for _, level := range []string{"low", "medium", "high"} {
			body := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"thinkingConfig":{"thinkingLevel":"` + level + `"}}}`)
			payload, wire, _, err := geminiNativeToAntigravityEnvelope(body, "project", family)
			if err != nil {
				t.Fatal(err)
			}
			want := family + "-" + level
			if wire != want || gjson.GetBytes(payload, "model").String() != want {
				t.Fatalf("%s/%s wire=%q payload=%s", family, level, wire, payload)
			}
			if got := gjson.GetBytes(payload, "request.generationConfig.thinkingConfig.thinkingLevel").String(); got != level {
				t.Fatalf("%s/%s thinking level=%q", family, level, got)
			}
		}
	}
}
