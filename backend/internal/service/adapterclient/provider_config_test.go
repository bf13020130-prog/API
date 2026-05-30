package adapterclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProviderConfigValidateAcceptsLongTailProvider(t *testing.T) {
	config := ProviderConfig{
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       ProviderStatusActive,
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/midjourney",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "secret"},
		Capabilities: []string{"image_task"},
		TimeoutMS:    30000,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
}

func TestProviderConfigValidateRejectsCoreProviderSlugs(t *testing.T) {
	for _, slug := range []string{"openai", "anthropic", "claude", "gemini", "codex", "antigravity"} {
		t.Run(slug, func(t *testing.T) {
			config := validProviderConfig()
			config.Slug = slug
			if err := config.Validate(); err == nil {
				t.Fatal("Validate returned nil, want error")
			}
		})
	}
}

func TestProviderConfigValidateRejectsIncompleteConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProviderConfig)
	}{
		{name: "name", mutate: func(c *ProviderConfig) { c.Name = "" }},
		{name: "slug", mutate: func(c *ProviderConfig) { c.Slug = "" }},
		{name: "adapter type", mutate: func(c *ProviderConfig) { c.AdapterType = "" }},
		{name: "base url", mutate: func(c *ProviderConfig) { c.BaseURL = "" }},
		{name: "invalid base url", mutate: func(c *ProviderConfig) { c.BaseURL = "ftp://adapter.internal" }},
		{name: "capabilities", mutate: func(c *ProviderConfig) { c.Capabilities = nil }},
		{name: "blank capability", mutate: func(c *ProviderConfig) { c.Capabilities = []string{" "} }},
		{name: "blank credential key", mutate: func(c *ProviderConfig) { c.Credentials = map[string]string{"": "secret"} }},
		{name: "negative timeout", mutate: func(c *ProviderConfig) { c.TimeoutMS = -1 }},
		{name: "invalid status", mutate: func(c *ProviderConfig) { c.Status = "paused" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := validProviderConfig()
			tt.mutate(&config)
			if err := config.Validate(); err == nil {
				t.Fatal("Validate returned nil, want error")
			}
		})
	}
}

func TestActiveProviderSlugsReturnsOnlyValidActiveLongTailProviders(t *testing.T) {
	active := validProviderConfig()
	active.Slug = " MidJourney "
	active.Status = ProviderStatusActive

	disabled := validProviderConfig()
	disabled.Slug = "suno"
	disabled.Status = ProviderStatusDisabled

	core := validProviderConfig()
	core.Slug = "openai"
	core.Status = ProviderStatusActive

	blankStatus := validProviderConfig()
	blankStatus.Slug = "replicate"
	blankStatus.Status = ""

	got := ActiveProviderSlugs([]ProviderConfig{active, disabled, core, blankStatus})
	want := []string{"midjourney", "replicate"}
	if len(got) != len(want) {
		t.Fatalf("ActiveProviderSlugs len = %d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ActiveProviderSlugs[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestBuildProviderDiagnosticsShowsValidityWithoutCredentials(t *testing.T) {
	active := validProviderConfig()
	active.Slug = " MidJourney "
	active.Credentials = map[string]string{"token": "secret-token"}

	disabled := validProviderConfig()
	disabled.Name = "Suno"
	disabled.Slug = "suno"
	disabled.Status = ProviderStatusDisabled
	disabled.Credentials = map[string]string{"token": "disabled-secret"}

	core := validProviderConfig()
	core.Name = "OpenAI should stay native"
	core.Slug = "openai"

	broken := validProviderConfig()
	broken.Name = "Broken"
	broken.Slug = "broken"
	broken.BaseURL = "ftp://adapter.internal/broken"

	got := BuildProviderDiagnostics([]ProviderConfig{active, disabled, core, broken})
	if len(got) != 4 {
		t.Fatalf("BuildProviderDiagnostics len = %d, want 4", len(got))
	}

	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	if strings.Contains(string(body), "secret-token") || strings.Contains(string(body), "disabled-secret") {
		t.Fatalf("diagnostics exposed credentials: %s", body)
	}

	if got[0].Slug != "MidJourney" || !got[0].Valid || !got[0].Enabled || got[0].Reason != "enabled" {
		t.Fatalf("active diagnostic = %+v", got[0])
	}
	if !got[1].Valid || got[1].Enabled || got[1].Reason != "provider_disabled" {
		t.Fatalf("disabled diagnostic = %+v", got[1])
	}
	if got[2].Valid || got[2].Enabled || !strings.Contains(got[2].Reason, "core provider slug") {
		t.Fatalf("core diagnostic = %+v", got[2])
	}
	if got[3].Valid || got[3].Enabled || !strings.Contains(got[3].Reason, "valid http base url") {
		t.Fatalf("broken diagnostic = %+v", got[3])
	}
}

func validProviderConfig() ProviderConfig {
	return ProviderConfig{
		Name:         "Midjourney",
		Slug:         "midjourney",
		Status:       ProviderStatusActive,
		AdapterType:  "new-api",
		BaseURL:      "https://adapter.internal/midjourney",
		AuthMode:     "bearer",
		Credentials:  map[string]string{"token": "secret"},
		Capabilities: []string{"image_task"},
		TimeoutMS:    30000,
	}
}
