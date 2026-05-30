package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
)

func TestNewGatewayCapabilityRouterDefaultsToNoAdapterProviders(t *testing.T) {
	router := newGatewayCapabilityRouter(nil)

	got := router.Decide(capabilityrouter.Input{
		AdapterProvider:    "midjourney",
		Capability:         "image_task",
		AllowNewAPIAdapter: true,
	})
	if got.Target != capabilityrouter.TargetUnsupported {
		t.Fatalf("Target = %q, want %q", got.Target, capabilityrouter.TargetUnsupported)
	}
}

func TestNewGatewayCapabilityRouterEnablesConfiguredLongTailProvidersOnly(t *testing.T) {
	router := newGatewayCapabilityRouter(adapterclient.NewStaticProviderRegistry([]adapterclient.ProviderConfig{
		{
			Name:         "Midjourney",
			Slug:         "midjourney",
			Status:       "active",
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.internal/midjourney",
			Capabilities: []string{"image_task"},
		},
		{
			Name:         "OpenAI",
			Slug:         "openai",
			Status:       "active",
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.internal/openai",
			Capabilities: []string{"chat"},
		},
		{
			Name:         "Suno",
			Slug:         "suno",
			Status:       "disabled",
			AdapterType:  "new-api",
			BaseURL:      "https://adapter.internal/suno",
			Capabilities: []string{"audio_task"},
		},
	}))

	midjourney := router.Decide(capabilityrouter.Input{
		AdapterProvider:    "midjourney",
		Capability:         "image_task",
		AllowNewAPIAdapter: true,
	})
	if midjourney.Target != capabilityrouter.TargetNewAPIAdapter {
		t.Fatalf("midjourney Target = %q, want %q", midjourney.Target, capabilityrouter.TargetNewAPIAdapter)
	}

	for _, provider := range []string{"openai", "suno"} {
		t.Run(provider, func(t *testing.T) {
			got := router.Decide(capabilityrouter.Input{
				AdapterProvider:    provider,
				Capability:         "task",
				AllowNewAPIAdapter: true,
			})
			if got.Target != capabilityrouter.TargetUnsupported {
				t.Fatalf("Target = %q, want %q", got.Target, capabilityrouter.TargetUnsupported)
			}
		})
	}
}
