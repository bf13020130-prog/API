package adapterclient

type ProviderDiagnostics struct {
	ObserveOnly        bool                 `json:"observe_only"`
	EnforcementEnabled bool                 `json:"enforcement_enabled"`
	ActiveSlugs        []string             `json:"active_slugs"`
	Providers          []ProviderDiagnostic `json:"providers"`
}

type ProviderRegistry interface {
	Providers() []ProviderConfig
	Diagnostics() ProviderDiagnostics
}

type StaticProviderRegistry struct {
	providers []ProviderConfig
}

func NewStaticProviderRegistry(providers []ProviderConfig) *StaticProviderRegistry {
	return &StaticProviderRegistry{
		providers: cloneProviderConfigs(providers),
	}
}

func (r *StaticProviderRegistry) Providers() []ProviderConfig {
	if r == nil {
		return nil
	}
	return cloneProviderConfigs(r.providers)
}

func (r *StaticProviderRegistry) Diagnostics() ProviderDiagnostics {
	providers := r.Providers()
	return ProviderDiagnostics{
		ObserveOnly:        true,
		EnforcementEnabled: false,
		ActiveSlugs:        ActiveProviderSlugs(providers),
		Providers:          BuildProviderDiagnostics(providers),
	}
}

func cloneProviderConfigs(providers []ProviderConfig) []ProviderConfig {
	if len(providers) == 0 {
		return nil
	}
	out := make([]ProviderConfig, len(providers))
	for i := range providers {
		out[i] = providers[i]
		if providers[i].Credentials != nil {
			out[i].Credentials = make(map[string]string, len(providers[i].Credentials))
			for key, value := range providers[i].Credentials {
				out[i].Credentials[key] = value
			}
		}
		out[i].Capabilities = append([]string(nil), providers[i].Capabilities...)
	}
	return out
}
