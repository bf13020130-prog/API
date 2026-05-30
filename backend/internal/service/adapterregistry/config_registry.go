package adapterregistry

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
)

func NewConfigProviderRegistry(cfg *config.Config) adapterclient.ProviderRegistry {
	return adapterclient.NewStaticProviderRegistry(ProviderConfigsFromConfig(cfg))
}

func ProviderConfigsFromConfig(cfg *config.Config) []adapterclient.ProviderConfig {
	if cfg == nil || len(cfg.Gateway.AdapterProviders) == 0 {
		return nil
	}
	providers := make([]adapterclient.ProviderConfig, 0, len(cfg.Gateway.AdapterProviders))
	for _, provider := range cfg.Gateway.AdapterProviders {
		providers = append(providers, adapterclient.ProviderConfig{
			Name:         provider.Name,
			Slug:         provider.Slug,
			Status:       adapterclient.ProviderStatus(provider.Status),
			AdapterType:  provider.AdapterType,
			BaseURL:      provider.BaseURL,
			AuthMode:     provider.AuthMode,
			Credentials:  provider.Credentials,
			Capabilities: provider.Capabilities,
			Priority:     provider.Priority,
			TimeoutMS:    provider.TimeoutMS,
		})
	}
	return providers
}
