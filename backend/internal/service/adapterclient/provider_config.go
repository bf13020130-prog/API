package adapterclient

import (
	"errors"
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type ProviderStatus string

const (
	ProviderStatusActive   ProviderStatus = "active"
	ProviderStatusDisabled ProviderStatus = "disabled"
)

type ProviderConfig struct {
	Name         string
	Slug         string
	Status       ProviderStatus
	AdapterType  string
	BaseURL      string
	AuthMode     string
	Credentials  map[string]string
	Capabilities []string
	Priority     int
	TimeoutMS    int
}

type ProviderDiagnostic struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	Status       string   `json:"status"`
	AdapterType  string   `json:"adapter_type"`
	BaseURL      string   `json:"base_url"`
	AuthMode     string   `json:"auth_mode,omitempty"`
	Capabilities []string `json:"capabilities"`
	Priority     int      `json:"priority"`
	TimeoutMS    int      `json:"timeout_ms"`
	Valid        bool     `json:"valid"`
	Enabled      bool     `json:"enabled"`
	Reason       string   `json:"reason,omitempty"`
}

func (c ProviderConfig) Validate() error {
	switch {
	case strings.TrimSpace(c.Name) == "":
		return errors.New("provider name is required")
	case strings.TrimSpace(c.Slug) == "":
		return errors.New("provider slug is required")
	case isCoreProviderSlug(c.Slug):
		return errors.New("core provider slug cannot be configured as an adapter provider")
	case strings.TrimSpace(c.AdapterType) == "":
		return errors.New("adapter type is required")
	case !validHTTPBaseURL(c.BaseURL):
		return errors.New("valid http base url is required")
	case len(c.Capabilities) == 0:
		return errors.New("at least one capability is required")
	case c.TimeoutMS < 0:
		return errors.New("timeout_ms cannot be negative")
	}
	switch c.normalizedStatus() {
	case ProviderStatusActive, ProviderStatusDisabled:
	default:
		return errors.New("provider status must be active or disabled")
	}

	for _, capability := range c.Capabilities {
		if strings.TrimSpace(capability) == "" {
			return errors.New("capability cannot be blank")
		}
	}
	for key := range c.Credentials {
		if strings.TrimSpace(key) == "" {
			return errors.New("credential key cannot be blank")
		}
	}

	return nil
}

func BuildProviderDiagnostics(configs []ProviderConfig) []ProviderDiagnostic {
	diagnostics := make([]ProviderDiagnostic, 0, len(configs))
	for _, config := range configs {
		diagnostic := ProviderDiagnostic{
			Name:         config.Name,
			Slug:         strings.TrimSpace(config.Slug),
			Status:       string(config.normalizedStatus()),
			AdapterType:  config.AdapterType,
			BaseURL:      config.BaseURL,
			AuthMode:     config.AuthMode,
			Capabilities: append([]string(nil), config.Capabilities...),
			Priority:     config.Priority,
			TimeoutMS:    config.TimeoutMS,
		}

		if err := config.Validate(); err != nil {
			diagnostic.Reason = err.Error()
		} else {
			diagnostic.Valid = true
			if config.normalizedStatus() == ProviderStatusActive {
				diagnostic.Enabled = true
				diagnostic.Reason = "enabled"
			} else {
				diagnostic.Reason = "provider_disabled"
			}
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

func ActiveProviderSlugs(configs []ProviderConfig) []string {
	slugs := make([]string, 0, len(configs))
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		if config.normalizedStatus() != ProviderStatusActive {
			continue
		}
		if err := config.Validate(); err != nil {
			continue
		}
		slug := normalizeProviderSlug(config.Slug)
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		slugs = append(slugs, slug)
	}
	return slugs
}

func (c ProviderConfig) normalizedStatus() ProviderStatus {
	status := ProviderStatus(strings.ToLower(strings.TrimSpace(string(c.Status))))
	if status == "" {
		return ProviderStatusActive
	}
	return status
}

func validHTTPBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func isCoreProviderSlug(slug string) bool {
	switch normalizeProviderSlug(slug) {
	case domain.PlatformOpenAI, domain.PlatformAnthropic, domain.PlatformGemini, domain.PlatformAntigravity:
		return true
	case "claude", "codex", "chatgpt", "anthropic-claude", "openai-compatible":
		return true
	default:
		return false
	}
}

func normalizeProviderSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}
