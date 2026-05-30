package capabilityrouter

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

func TestRouterRoutesCoreProvidersToSub2APINative(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})

	tests := []struct {
		name         string
		input        Input
		wantPlatform string
	}{
		{
			name: "codex responses stays native openai even when adapter is allowed",
			input: Input{
				Path:               "/backend-api/codex/responses",
				Model:              "codex-mini-latest",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformOpenAI,
		},
		{
			name: "claude model stays native anthropic",
			input: Input{
				Path:               "/v1/messages",
				Model:              "claude-sonnet-4-6",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformAnthropic,
		},
		{
			name: "gemini v1beta route stays native gemini",
			input: Input{
				Path:               "/v1beta/models/gemini-2.5-pro:generateContent",
				Model:              "gemini-2.5-pro",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformGemini,
		},
		{
			name: "antigravity route stays native antigravity",
			input: Input{
				Path:               "/antigravity/v1/messages",
				Model:              "claude-sonnet-4-6",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformAntigravity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.Decide(tt.input)
			if got.Target != TargetSub2APINative {
				t.Fatalf("Target = %q, want %q", got.Target, TargetSub2APINative)
			}
			if got.Platform != tt.wantPlatform {
				t.Fatalf("Platform = %q, want %q", got.Platform, tt.wantPlatform)
			}
			if got.Target == TargetNewAPIAdapter {
				t.Fatal("core provider unexpectedly routed to new-api adapter")
			}
		})
	}
}

func TestRouterRoutesOpenAICompatibleUpstreamToSub2APIUpstream(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})

	got := router.Decide(Input{
		Path:               "/v1/chat/completions",
		Model:              "gpt-4.1",
		AccountPlatform:    domain.PlatformOpenAI,
		AccountType:        domain.AccountTypeUpstream,
		AdapterProvider:    "midjourney",
		AllowNewAPIAdapter: true,
	})

	if got.Target != TargetSub2APIUpstream {
		t.Fatalf("Target = %q, want %q", got.Target, TargetSub2APIUpstream)
	}
	if got.Platform != domain.PlatformOpenAI {
		t.Fatalf("Platform = %q, want %q", got.Platform, domain.PlatformOpenAI)
	}
}

func TestRouterRequiresExplicitLongTailAdapterEnablement(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})

	tests := []struct {
		name  string
		input Input
		want  Target
	}{
		{
			name: "enabled long-tail provider can use new-api adapter",
			input: Input{
				AdapterProvider:    "midjourney",
				Capability:         "image_task",
				AllowNewAPIAdapter: true,
			},
			want: TargetNewAPIAdapter,
		},
		{
			name: "adapter provider without allow flag is unsupported",
			input: Input{
				AdapterProvider: "midjourney",
				Capability:      "image_task",
			},
			want: TargetUnsupported,
		},
		{
			name: "allow flag without configured provider is unsupported",
			input: Input{
				AdapterProvider:    "suno",
				Capability:         "audio_task",
				AllowNewAPIAdapter: true,
			},
			want: TargetUnsupported,
		},
		{
			name: "unknown provider is unsupported",
			input: Input{
				AdapterProvider:    "mystery-provider",
				Capability:         "task",
				AllowNewAPIAdapter: true,
			},
			want: TargetUnsupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.Decide(tt.input)
			if got.Target != tt.want {
				t.Fatalf("Target = %q, want %q; reason=%s", got.Target, tt.want, got.Reason)
			}
		})
	}
}

func TestRouterRejectsCoreProviderAdapterSlugs(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{
			"openai",
			"anthropic",
			"claude",
			"gemini",
			"codex",
			"antigravity",
			"midjourney",
		},
	})

	for _, provider := range []string{"openai", "anthropic", "claude", "gemini", "codex", "antigravity"} {
		t.Run(provider, func(t *testing.T) {
			got := router.Decide(Input{
				AdapterProvider:    provider,
				Capability:         "adapter_task",
				AllowNewAPIAdapter: true,
			})
			if got.Target != TargetUnsupported {
				t.Fatalf("Target = %q, want %q", got.Target, TargetUnsupported)
			}
		})
	}

	got := router.Decide(Input{
		AdapterProvider:    "midjourney",
		Capability:         "image_task",
		AllowNewAPIAdapter: true,
	})
	if got.Target != TargetNewAPIAdapter {
		t.Fatalf("long-tail Target = %q, want %q", got.Target, TargetNewAPIAdapter)
	}
}

func TestRouterInfersNativePlatformFromGroupBeforeAdapter(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})

	got := router.Decide(Input{
		Path:               "/v1/chat/completions",
		GroupPlatform:      domain.PlatformOpenAI,
		Model:              "custom-openai-compatible-model",
		AdapterProvider:    "midjourney",
		AllowNewAPIAdapter: true,
	})

	if got.Target != TargetSub2APINative {
		t.Fatalf("Target = %q, want %q", got.Target, TargetSub2APINative)
	}
	if got.Platform != domain.PlatformOpenAI {
		t.Fatalf("Platform = %q, want %q", got.Platform, domain.PlatformOpenAI)
	}
}

func TestRouterUsesGroupPlatformForSharedNativeRoutes(t *testing.T) {
	router := New(Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})

	tests := []struct {
		name         string
		input        Input
		wantPlatform string
	}{
		{
			name: "openai group can own messages route",
			input: Input{
				Path:               "/v1/messages",
				GroupPlatform:      domain.PlatformOpenAI,
				Model:              "gpt-4.1",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformOpenAI,
		},
		{
			name: "anthropic group can own chat completions compatibility route",
			input: Input{
				Path:               "/v1/chat/completions",
				GroupPlatform:      domain.PlatformAnthropic,
				Model:              "claude-sonnet-4-6",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformAnthropic,
		},
		{
			name: "model signal can own shared responses route without group",
			input: Input{
				Path:               "/v1/responses",
				Model:              "claude-sonnet-4-6",
				AdapterProvider:    "midjourney",
				AllowNewAPIAdapter: true,
			},
			wantPlatform: domain.PlatformAnthropic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.Decide(tt.input)
			if got.Target != TargetSub2APINative {
				t.Fatalf("Target = %q, want %q", got.Target, TargetSub2APINative)
			}
			if got.Platform != tt.wantPlatform {
				t.Fatalf("Platform = %q, want %q", got.Platform, tt.wantPlatform)
			}
		})
	}
}
