package capabilityrouter

import (
	"net/url"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
)

type Target string

const (
	TargetSub2APINative   Target = "sub2api_native"
	TargetSub2APIUpstream Target = "sub2api_upstream"
	TargetNewAPIAdapter   Target = "new_api_adapter"
	TargetUnsupported     Target = "unsupported"
)

type Config struct {
	NewAPIAdapterProviders []string
}

type Router struct {
	newAPIAdapterProviders map[string]struct{}
}

type Input struct {
	Method             string
	Path               string
	Model              string
	Capability         string
	GroupPlatform      string
	AccountPlatform    string
	AccountType        string
	AdapterProvider    string
	AllowNewAPIAdapter bool
}

type Decision struct {
	Target   Target
	Platform string
	Reason   string
}

func New(config Config) Router {
	providers := make(map[string]struct{}, len(config.NewAPIAdapterProviders))
	for _, provider := range config.NewAPIAdapterProviders {
		provider = normalizeToken(provider)
		if provider == "" || isCoreProviderSlug(provider) {
			continue
		}
		providers[provider] = struct{}{}
	}
	return Router{newAPIAdapterProviders: providers}
}

func (r Router) Decide(input Input) Decision {
	normalized := normalizeInput(input)

	if normalized.AccountType == domain.AccountTypeUpstream {
		platform := firstNonEmpty(normalized.AccountPlatform, normalized.GroupPlatform, inferPlatformFromModel(normalized.Model))
		if platform == "" {
			platform = domain.PlatformOpenAI
		}
		return Decision{
			Target:   TargetSub2APIUpstream,
			Platform: platform,
			Reason:   "account_type_upstream_owned_by_sub2api",
		}
	}

	if platform := inferNativePlatform(normalized); platform != "" {
		return Decision{
			Target:   TargetSub2APINative,
			Platform: platform,
			Reason:   "core_provider_or_native_route",
		}
	}

	if normalized.AdapterProvider != "" {
		if !normalized.AllowNewAPIAdapter {
			return Decision{
				Target: TargetUnsupported,
				Reason: "new_api_adapter_not_allowed",
			}
		}
		if _, ok := r.newAPIAdapterProviders[normalized.AdapterProvider]; ok {
			return Decision{
				Target: TargetNewAPIAdapter,
				Reason: "explicit_long_tail_adapter",
			}
		}
		return Decision{
			Target: TargetUnsupported,
			Reason: "new_api_adapter_provider_not_configured",
		}
	}

	return Decision{
		Target: TargetUnsupported,
		Reason: "no_matching_capability_route",
	}
}

func normalizeInput(input Input) Input {
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Path = normalizePath(input.Path)
	input.Model = normalizeToken(input.Model)
	input.Capability = normalizeToken(input.Capability)
	input.GroupPlatform = normalizePlatform(input.GroupPlatform)
	input.AccountPlatform = normalizePlatform(input.AccountPlatform)
	input.AccountType = normalizeToken(input.AccountType)
	input.AdapterProvider = normalizeToken(input.AdapterProvider)
	return input
}

func inferNativePlatform(input Input) string {
	if platform := forcedNativeRoutePlatform(input.Path); platform != "" {
		return platform
	}
	if input.GroupPlatform != "" {
		return input.GroupPlatform
	}
	if input.AccountPlatform != "" {
		return input.AccountPlatform
	}
	if platform := inferPlatformFromModel(input.Model); platform != "" {
		return platform
	}
	return defaultNativeRoutePlatform(input.Path)
}

func forcedNativeRoutePlatform(path string) string {
	switch {
	case path == "/backend-api/codex/responses" || strings.HasPrefix(path, "/backend-api/codex/responses/"):
		return domain.PlatformOpenAI
	case path == "/v1/embeddings" || path == "/embeddings":
		return domain.PlatformOpenAI
	case path == "/v1/images/generations" || path == "/v1/images/edits" || path == "/images/generations" || path == "/images/edits":
		return domain.PlatformOpenAI
	case path == "/v1beta/models" || strings.HasPrefix(path, "/v1beta/models/"):
		return domain.PlatformGemini
	case path == "/antigravity/models" || strings.HasPrefix(path, "/antigravity/"):
		return domain.PlatformAntigravity
	default:
		return ""
	}
}

func defaultNativeRoutePlatform(path string) string {
	switch {
	case path == "/responses" || strings.HasPrefix(path, "/responses/"):
		return domain.PlatformOpenAI
	case path == "/v1/responses" || strings.HasPrefix(path, "/v1/responses/"):
		return domain.PlatformOpenAI
	case path == "/v1/chat/completions" || path == "/chat/completions":
		return domain.PlatformOpenAI
	case path == "/v1/messages" || path == "/v1/messages/count_tokens":
		return domain.PlatformAnthropic
	case path == "/v1/models":
		return domain.PlatformAnthropic
	default:
		return ""
	}
}

func inferPlatformFromModel(model string) string {
	switch {
	case model == "":
		return ""
	case strings.Contains(model, "antigravity"):
		return domain.PlatformAntigravity
	case strings.HasPrefix(model, "models/gemini-") || strings.HasPrefix(model, "gemini-"):
		return domain.PlatformGemini
	case strings.HasPrefix(model, "claude-") || strings.HasPrefix(model, "anthropic."):
		return domain.PlatformAnthropic
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "chatgpt-") || strings.Contains(model, "codex"):
		return domain.PlatformOpenAI
	case model == "o1" || model == "o3" || model == "o4" || model == "o5":
		return domain.PlatformOpenAI
	case strings.HasPrefix(model, "o1-") || strings.HasPrefix(model, "o3-") || strings.HasPrefix(model, "o4-") || strings.HasPrefix(model, "o5-"):
		return domain.PlatformOpenAI
	default:
		return ""
	}
}

func isCoreProviderSlug(provider string) bool {
	switch provider {
	case domain.PlatformOpenAI, domain.PlatformAnthropic, domain.PlatformGemini, domain.PlatformAntigravity:
		return true
	case "claude", "codex", "chatgpt", "anthropic-claude", "openai-compatible":
		return true
	default:
		return false
	}
}

func normalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if parsed, err := url.Parse(path); err == nil && parsed.Path != "" {
		path = parsed.Path
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return strings.ToLower(path)
}

func normalizePlatform(platform string) string {
	platform = normalizeToken(platform)
	switch platform {
	case "claude":
		return domain.PlatformAnthropic
	default:
		return platform
	}
}

func normalizeToken(token string) string {
	return strings.ToLower(strings.TrimSpace(token))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
