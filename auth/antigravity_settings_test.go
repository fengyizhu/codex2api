package auth

import "testing"

func TestAntigravitySettingsExposeThoughtsRoundTrip(t *testing.T) {
	encoded, err := EncodeAntigravitySettings(AntigravitySettings{ExposeThoughts: true, UserAgent: "ua-test", APIClient: "api-test"})
	if err != nil {
		t.Fatal(err)
	}
	if encoded != `{"expose_thoughts":true,"user_agent":"ua-test","api_client":"api-test"}` {
		t.Fatalf("encoded = %q, want only expose_thoughts", encoded)
	}
	parsed, err := ParseAntigravitySettings(encoded)
	if err != nil || !parsed.ExposeThoughts || parsed.UserAgent != "ua-test" || parsed.APIClient != "api-test" {
		t.Fatalf("parsed = %+v err=%v", parsed, err)
	}
	if empty, _ := EncodeAntigravitySettings(AntigravitySettings{}); empty != "{}" {
		t.Fatalf("empty settings encoded = %q", empty)
	}

	previous := ConfiguredAntigravitySettings()
	t.Cleanup(func() { SetConfiguredAntigravitySettings(previous) })
	SetConfiguredAntigravitySettings(parsed)
	if !AntigravityExposeThoughts() || !ConfiguredAntigravitySettings().ExposeThoughts {
		t.Fatal("runtime settings lost expose_thoughts")
	}
}

func TestAntigravityIdentitySettingsOverrideDefaults(t *testing.T) {
	previous := ConfiguredAntigravitySettings()
	t.Cleanup(func() { SetConfiguredAntigravitySettings(previous) })
	t.Setenv(antigravityUserAgentEnv, "")
	t.Setenv(antigravityAPIClientEnv, "")
	SetConfiguredAntigravitySettings(AntigravitySettings{UserAgent: "ua-from-settings", APIClient: "api-from-settings"})
	if got := AntigravityUserAgent(); got != "ua-from-settings" {
		t.Fatalf("configured User-Agent = %q", got)
	}
	if got := AntigravityAPIClientHeader(); got != "api-from-settings" {
		t.Fatalf("configured API client = %q", got)
	}
}
