package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
)

func newGatewayCapabilityRouter(registry adapterclient.ProviderRegistry) capabilityrouter.Router {
	var activeSlugs []string
	if registry != nil {
		activeSlugs = registry.Diagnostics().ActiveSlugs
	}
	return capabilityrouter.New(capabilityrouter.Config{
		NewAPIAdapterProviders: activeSlugs,
	})
}
