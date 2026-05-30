package adapterclient

import "testing"

func TestStaticProviderRegistryReturnsDiagnosticsAndActiveSlugs(t *testing.T) {
	registry := NewStaticProviderRegistry([]ProviderConfig{
		{
			Name:         "Midjourney",
			Slug:         " midjourney ",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.example.com",
			Capabilities: []string{"image_task"},
		},
		{
			Name:         "OpenAI shadow",
			Slug:         "openai",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.example.com",
			Capabilities: []string{"chat"},
		},
		{
			Name:         "Disabled",
			Slug:         "suno",
			Status:       ProviderStatusDisabled,
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.example.com",
			Capabilities: []string{"audio_task"},
		},
	})

	diagnostics := registry.Diagnostics()
	if len(diagnostics.Providers) != 3 {
		t.Fatalf("Providers length = %d, want 3", len(diagnostics.Providers))
	}
	if diagnostics.EnforcementEnabled {
		t.Fatal("EnforcementEnabled = true, want false")
	}
	if !diagnostics.ObserveOnly {
		t.Fatal("ObserveOnly = false, want true")
	}
	if len(diagnostics.ActiveSlugs) != 1 || diagnostics.ActiveSlugs[0] != "midjourney" {
		t.Fatalf("ActiveSlugs = %#v, want [midjourney]", diagnostics.ActiveSlugs)
	}
	if !diagnostics.Providers[0].Enabled {
		t.Fatal("first provider should be enabled")
	}
	if diagnostics.Providers[1].Valid {
		t.Fatal("core provider slug should be invalid")
	}
	if diagnostics.Providers[2].Enabled {
		t.Fatal("disabled provider should not be enabled")
	}
}

func TestStaticProviderRegistryClonesConfigs(t *testing.T) {
	configs := []ProviderConfig{
		{
			Name:         "Midjourney",
			Slug:         "midjourney",
			Status:       ProviderStatusActive,
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.example.com",
			Credentials:  map[string]string{"api_key": "secret"},
			Capabilities: []string{"image_task"},
		},
	}
	registry := NewStaticProviderRegistry(configs)

	configs[0].Slug = "mutated"
	configs[0].Credentials["api_key"] = "mutated"
	configs[0].Capabilities[0] = "mutated"

	providers := registry.Providers()
	if providers[0].Slug != "midjourney" {
		t.Fatalf("stored Slug = %q, want midjourney", providers[0].Slug)
	}
	if providers[0].Credentials["api_key"] != "secret" {
		t.Fatalf("stored credential = %q, want secret", providers[0].Credentials["api_key"])
	}
	if providers[0].Capabilities[0] != "image_task" {
		t.Fatalf("stored capability = %q, want image_task", providers[0].Capabilities[0])
	}

	providers[0].Slug = "changed outside"
	providers[0].Credentials["api_key"] = "changed outside"
	providers[0].Capabilities[0] = "changed outside"

	again := registry.Providers()
	if again[0].Slug != "midjourney" {
		t.Fatalf("returned Slug mutation leaked: %q", again[0].Slug)
	}
	if again[0].Credentials["api_key"] != "secret" {
		t.Fatalf("returned credential mutation leaked: %q", again[0].Credentials["api_key"])
	}
	if again[0].Capabilities[0] != "image_task" {
		t.Fatalf("returned capability mutation leaked: %q", again[0].Capabilities[0])
	}
}
