package service

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
)

type AdapterDiagnosticSamplingConfig struct {
	Enabled         bool
	SampleAll       bool
	Providers       []string
	RequestIDs      []string
	MaxPayloadBytes int
	MaxStringBytes  int
	MaxEvents       int
}

type adapterDiagnosticSampler struct {
	cfg AdapterDiagnosticSamplingConfig
	by  []string
}

const (
	defaultAdapterDiagnosticMaxPayloadBytes = 4096
	defaultAdapterDiagnosticMaxStringBytes  = 512
	defaultAdapterDiagnosticMaxEvents       = 3
)

var adapterDiagnosticBearerPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)
var adapterDiagnosticSecretPattern = regexp.MustCompile(`(?i)\b(?:sk|sess|pk|ak)-[A-Za-z0-9._~+/=-]+`)

func newAdapterDiagnosticSampler(cfg AdapterDiagnosticSamplingConfig, input AdapterEnforcementInput, provider *AdapterProvider, resp adapterclient.Response, callErr error, billingErr error) *adapterDiagnosticSampler {
	cfg = normalizeAdapterDiagnosticSamplingConfig(cfg)
	by := make([]string, 0, 2)
	providerSlug := ""
	if provider != nil {
		providerSlug = strings.TrimSpace(provider.Slug)
	}
	if providerSlug == "" {
		providerSlug = strings.TrimSpace(input.GroupPlatform)
	}
	if isAdapterDiagnosticFailure(resp, callErr, billingErr) {
		by = append(by, "failure")
	}
	if cfg.Enabled {
		if cfg.SampleAll {
			by = append(by, "sample_all")
		}
		if adapterDiagnosticListContainsFold(cfg.Providers, providerSlug) {
			by = append(by, "provider:"+strings.ToLower(providerSlug))
		}
		if adapterDiagnosticListContains(cfg.RequestIDs, input.RequestID) {
			by = append(by, "request_id:"+strings.TrimSpace(input.RequestID))
		}
	}
	if len(by) == 0 {
		return nil
	}
	return &adapterDiagnosticSampler{cfg: cfg, by: by}
}

func isAdapterDiagnosticFailure(resp adapterclient.Response, callErr error, billingErr error) bool {
	return callErr != nil ||
		billingErr != nil ||
		resp.Status == adapterclient.StatusFailed ||
		resp.AdapterStatus >= 400 ||
		strings.TrimSpace(resp.ErrorMessage) != ""
}

func normalizeAdapterDiagnosticSamplingConfig(cfg AdapterDiagnosticSamplingConfig) AdapterDiagnosticSamplingConfig {
	if cfg.MaxPayloadBytes <= 0 {
		cfg.MaxPayloadBytes = defaultAdapterDiagnosticMaxPayloadBytes
	}
	if cfg.MaxStringBytes <= 0 {
		cfg.MaxStringBytes = defaultAdapterDiagnosticMaxStringBytes
	}
	if cfg.MaxEvents <= 0 {
		cfg.MaxEvents = defaultAdapterDiagnosticMaxEvents
	}
	return cfg
}

func (s *adapterDiagnosticSampler) initialMetadata(input AdapterEnforcementInput, provider *AdapterProvider, resp adapterclient.Response, callErr error, billingErr error) map[string]any {
	if s == nil {
		return nil
	}
	out := map[string]any{
		"sampled_by": append([]string(nil), s.by...),
		"request":    s.requestMetadata(input, provider),
	}
	if response := s.responseMetadata(resp); len(response) > 0 {
		out["response"] = response
	}
	if errMeta := s.errorMetadata(resp, callErr, billingErr); len(errMeta) > 0 {
		out["error"] = errMeta
	}
	return out
}

func (s *adapterDiagnosticSampler) requestMetadata(input AdapterEnforcementInput, provider *AdapterProvider) map[string]any {
	providerSlug := strings.TrimSpace(input.GroupPlatform)
	if provider != nil && strings.TrimSpace(provider.Slug) != "" {
		providerSlug = strings.TrimSpace(provider.Slug)
	}
	out := map[string]any{
		"request_id": input.RequestID,
		"provider":   providerSlug,
		"method":     input.Method,
		"path":       input.Path,
		"capability": input.Capability,
		"model":      input.Model,
		"headers":    redactAdapterDiagnosticHeaders(input.Headers, s.cfg),
	}
	if payload := buildAdapterDiagnosticPayload(input.Payload, s.cfg); len(payload) > 0 {
		out["payload"] = payload
	}
	return out
}

func (s *adapterDiagnosticSampler) responseMetadata(resp adapterclient.Response) map[string]any {
	out := map[string]any{}
	if resp.Status != "" {
		out["status"] = string(resp.Status)
	}
	if resp.AdapterStatus > 0 {
		out["adapter_status"] = float64(resp.AdapterStatus)
	}
	if strings.TrimSpace(resp.ErrorCode) != "" {
		out["error_code"] = truncateAdapterDiagnosticString(redactAdapterDiagnosticString(resp.ErrorCode), s.cfg.MaxStringBytes)
	}
	if len(resp.Body) > 0 {
		out["body"] = buildAdapterDiagnosticPayload(resp.Body, s.cfg)
	}
	return out
}

func (s *adapterDiagnosticSampler) errorMetadata(resp adapterclient.Response, callErr error, billingErr error) map[string]any {
	if callErr != nil {
		return map[string]any{
			"root_cause": "adapter_call_error",
			"message":    truncateAdapterDiagnosticString(redactAdapterDiagnosticString(callErr.Error()), s.cfg.MaxStringBytes),
		}
	}
	if billingErr != nil {
		return map[string]any{
			"root_cause": "billing_error",
			"message":    truncateAdapterDiagnosticString(redactAdapterDiagnosticString(billingErr.Error()), s.cfg.MaxStringBytes),
		}
	}
	if resp.Status == adapterclient.StatusFailed || resp.AdapterStatus >= 400 || strings.TrimSpace(resp.ErrorMessage) != "" {
		out := map[string]any{
			"root_cause": "adapter_response_error",
		}
		if resp.AdapterStatus > 0 {
			out["adapter_status"] = float64(resp.AdapterStatus)
		}
		if strings.TrimSpace(resp.ErrorMessage) != "" {
			out["message"] = truncateAdapterDiagnosticString(redactAdapterDiagnosticString(resp.ErrorMessage), s.cfg.MaxStringBytes)
		}
		if strings.TrimSpace(resp.ErrorCode) != "" {
			out["code"] = truncateAdapterDiagnosticString(redactAdapterDiagnosticString(resp.ErrorCode), s.cfg.MaxStringBytes)
		}
		return out
	}
	return nil
}

func (s *adapterDiagnosticSampler) newStreamObserver(contentType string) *adapterStreamDiagnosticObserver {
	if s == nil || !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return nil
	}
	return &adapterStreamDiagnosticObserver{cfg: s.cfg}
}

func (s *adapterDiagnosticSampler) streamMetadata(observer *adapterStreamDiagnosticObserver, usageSource string) map[string]any {
	if s == nil || observer == nil {
		return nil
	}
	events := observer.Events()
	if len(events) == 0 && strings.TrimSpace(usageSource) == "" {
		return nil
	}
	stream := map[string]any{
		"transport": "sse",
	}
	if strings.TrimSpace(usageSource) != "" {
		stream["usage_source"] = usageSource
	}
	if len(events) > 0 {
		stream["events"] = events
	}
	return map[string]any{"stream": stream}
}

func (s *adapterDiagnosticSampler) newWebSocketObserver() *adapterEventDiagnosticObserver {
	if s == nil {
		return nil
	}
	return &adapterEventDiagnosticObserver{cfg: s.cfg}
}

func (s *adapterDiagnosticSampler) websocketOpenMetadata() map[string]any {
	if s == nil {
		return nil
	}
	return map[string]any{
		"websocket": map[string]any{
			"open": true,
		},
	}
}

func (s *adapterDiagnosticSampler) websocketMetadata(observer *adapterEventDiagnosticObserver, usageSource string) map[string]any {
	if s == nil || observer == nil {
		return nil
	}
	events := observer.Events()
	if len(events) == 0 && strings.TrimSpace(usageSource) == "" {
		return nil
	}
	websocket := map[string]any{
		"open": true,
	}
	if strings.TrimSpace(usageSource) != "" {
		websocket["usage_source"] = usageSource
	}
	if len(events) > 0 {
		websocket["events"] = events
	}
	return map[string]any{"websocket": websocket}
}

type adapterStreamDiagnosticObserver struct {
	cfg              AdapterDiagnosticSamplingConfig
	pendingLine      string
	dataLines        []string
	dataBytes        int
	dataOverflow     bool
	eventType        string
	currentEventSeen bool
	events           []any
}

func (o *adapterStreamDiagnosticObserver) Observe(chunk []byte) {
	if o == nil || len(chunk) == 0 {
		return
	}
	text := o.pendingLine + string(chunk)
	for {
		idx := strings.IndexByte(text, '\n')
		if idx < 0 {
			break
		}
		o.observeLine(text[:idx])
		text = text[idx+1:]
	}
	if len(text) > o.cfg.MaxPayloadBytes {
		text = text[len(text)-o.cfg.MaxPayloadBytes:]
	}
	o.pendingLine = text
}

func (o *adapterStreamDiagnosticObserver) Flush() {
	if o == nil {
		return
	}
	if strings.TrimSpace(o.pendingLine) != "" {
		o.observeLine(o.pendingLine)
		o.pendingLine = ""
	}
	o.processEvent()
}

func (o *adapterStreamDiagnosticObserver) Events() []any {
	if o == nil || len(o.events) == 0 {
		return nil
	}
	return append([]any(nil), o.events...)
}

func (o *adapterStreamDiagnosticObserver) observeLine(line string) {
	if o == nil {
		return
	}
	line = strings.TrimRight(line, "\r")
	if line == "" {
		o.processEvent()
		return
	}
	o.currentEventSeen = true
	if strings.HasPrefix(line, ":") {
		return
	}
	field, value, ok := strings.Cut(line, ":")
	if !ok {
		return
	}
	if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}
	switch field {
	case "event":
		o.eventType = strings.TrimSpace(value)
	case "data":
		if o.dataBytes+len(value) > o.cfg.MaxPayloadBytes {
			o.dataOverflow = true
			return
		}
		o.dataLines = append(o.dataLines, value)
		o.dataBytes += len(value)
	}
}

func (o *adapterStreamDiagnosticObserver) processEvent() {
	if o == nil || !o.currentEventSeen {
		return
	}
	defer func() {
		o.dataLines = nil
		o.dataBytes = 0
		o.dataOverflow = false
		o.eventType = ""
		o.currentEventSeen = false
	}()
	if o.dataOverflow || len(o.dataLines) == 0 {
		return
	}
	data := strings.TrimSpace(strings.Join(o.dataLines, "\n"))
	if data == "" || data == "[DONE]" {
		return
	}
	usage, _ := parseAdapterSSEDataUsage(data)
	event := buildAdapterDiagnosticEvent([]byte(data), o.eventType, usage != (adapterclient.Usage{}), o.cfg)
	if len(event) == 0 {
		return
	}
	o.appendEvent(event)
}

func (o *adapterStreamDiagnosticObserver) appendEvent(event map[string]any) {
	o.events = append(o.events, event)
	if maxEvents := o.cfg.MaxEvents; maxEvents > 0 && len(o.events) > maxEvents {
		o.events = append([]any(nil), o.events[len(o.events)-maxEvents:]...)
	}
}

type adapterEventDiagnosticObserver struct {
	cfg    AdapterDiagnosticSamplingConfig
	events []any
}

func (o *adapterEventDiagnosticObserver) Observe(payload []byte, usageDetected bool) {
	if o == nil || len(payload) == 0 {
		return
	}
	event := buildAdapterDiagnosticEvent(payload, "", usageDetected, o.cfg)
	if len(event) == 0 {
		return
	}
	o.events = append(o.events, event)
	if maxEvents := o.cfg.MaxEvents; maxEvents > 0 && len(o.events) > maxEvents {
		o.events = append([]any(nil), o.events[len(o.events)-maxEvents:]...)
	}
}

func (o *adapterEventDiagnosticObserver) Events() []any {
	if o == nil || len(o.events) == 0 {
		return nil
	}
	return append([]any(nil), o.events...)
}

func buildAdapterDiagnosticEvent(payload []byte, explicitEventType string, usageDetected bool, cfg AdapterDiagnosticSamplingConfig) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	var parsed any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return map[string]any{
			"event_type":     strings.TrimSpace(explicitEventType),
			"usage_detected": usageDetected,
			"payload_bytes":  len(payload),
			"parse_error":    "invalid_json",
		}
	}
	keyPaths := collectAdapterDiagnosticKeyPaths(parsed, 80)
	eventType := strings.TrimSpace(explicitEventType)
	if eventType == "" {
		eventType = firstAdapterDiagnosticTypeString(parsed)
	}
	event := map[string]any{
		"event_type":     eventType,
		"usage_detected": usageDetected,
		"payload_bytes":  len(payload),
		"json_key_paths": keyPaths,
		"sample":         redactAdapterDiagnosticValue(parsed, "", cfg, 0),
	}
	return event
}

func buildAdapterDiagnosticPayload(payload []byte, cfg AdapterDiagnosticSamplingConfig) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	out := map[string]any{
		"bytes": len(payload),
	}
	truncated := len(payload) > cfg.MaxPayloadBytes
	if truncated {
		out["truncated"] = true
	}
	if json.Valid(payload) {
		var parsed any
		if err := json.Unmarshal(payload, &parsed); err != nil {
			out["parse_error"] = "invalid_json"
			return out
		}
		out["json_key_paths"] = collectAdapterDiagnosticKeyPaths(parsed, 80)
		out["sample"] = redactAdapterDiagnosticValue(parsed, "", cfg, 0)
		return out
	}
	if truncated {
		out["sample"] = "[omitted_truncated_non_json]"
		return out
	}
	if !json.Valid(payload) {
		out["sample"] = truncateAdapterDiagnosticString(redactAdapterDiagnosticString(string(payload)), cfg.MaxStringBytes)
		return out
	}
	var parsed any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		out["parse_error"] = "invalid_json"
		return out
	}
	out["json_key_paths"] = collectAdapterDiagnosticKeyPaths(parsed, 80)
	out["sample"] = redactAdapterDiagnosticValue(parsed, "", cfg, 0)
	return out
}

func collectAdapterDiagnosticKeyPaths(value any, maxPaths int) []string {
	paths := make([]string, 0)
	var walk func(any, string)
	walk = func(current any, path string) {
		if len(paths) >= maxPaths {
			return
		}
		switch typed := current.(type) {
		case map[string]any:
			keys := make([]string, 0, len(typed))
			for key := range typed {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				nextPath := key
				if path != "" {
					nextPath = path + "." + key
				}
				walk(typed[key], nextPath)
			}
		case []any:
			for i, item := range typed {
				if len(paths) >= maxPaths {
					return
				}
				nextPath := path + "[" + adapterDiagnosticSmallInt(i) + "]"
				walk(item, nextPath)
			}
		default:
			if path != "" {
				paths = append(paths, path)
			}
		}
	}
	walk(value, "")
	return paths
}

func firstAdapterDiagnosticTypeString(value any) string {
	payload, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if raw, ok := payload["type"].(string); ok {
		return truncateAdapterDiagnosticString(redactAdapterDiagnosticString(raw), defaultAdapterDiagnosticMaxStringBytes)
	}
	if response, ok := payload["response"].(map[string]any); ok {
		if raw, ok := response["type"].(string); ok {
			return truncateAdapterDiagnosticString(redactAdapterDiagnosticString(raw), defaultAdapterDiagnosticMaxStringBytes)
		}
	}
	return ""
}

func redactAdapterDiagnosticHeaders(headers map[string]string, cfg AdapterDiagnosticSamplingConfig) map[string]any {
	if len(headers) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(headers))
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if isSensitiveAdapterDiagnosticKey(key) {
			out[key] = "[redacted]"
			continue
		}
		out[key] = truncateAdapterDiagnosticString(redactAdapterDiagnosticString(headers[key]), cfg.MaxStringBytes)
	}
	return out
}

func redactAdapterDiagnosticValue(value any, key string, cfg AdapterDiagnosticSamplingConfig, depth int) any {
	if depth > 8 {
		return "[truncated]"
	}
	if key != "" && isSensitiveAdapterDiagnosticKey(key) {
		return "[redacted]"
	}
	if key != "" && isContentAdapterDiagnosticKey(key) {
		return "[redacted]"
	}
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			out[childKey] = redactAdapterDiagnosticValue(typed[childKey], childKey, cfg, depth+1)
		}
		return out
	case []any:
		limit := len(typed)
		if limit > 3 {
			limit = 3
		}
		out := make([]any, 0, limit+1)
		for i := 0; i < limit; i++ {
			out = append(out, redactAdapterDiagnosticValue(typed[i], key, cfg, depth+1))
		}
		if len(typed) > limit {
			out = append(out, map[string]any{"truncated_items": len(typed) - limit})
		}
		return out
	case string:
		return truncateAdapterDiagnosticString(redactAdapterDiagnosticString(typed), cfg.MaxStringBytes)
	default:
		return typed
	}
}

func isSensitiveAdapterDiagnosticKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "_", "-")
	switch normalized {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "api-key", "x-api-key", "openai-api-key", "access-token", "refresh-token", "password", "secret", "credential", "credentials", "key":
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "credential") ||
		strings.HasSuffix(normalized, "-key")
}

func isContentAdapterDiagnosticKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "prompt", "content", "text", "completion", "answer", "message", "messages", "contents", "parts", "input", "instructions":
		return true
	default:
		return false
	}
}

func redactAdapterDiagnosticString(value string) string {
	value = adapterDiagnosticBearerPattern.ReplaceAllString(value, "Bearer [redacted]")
	value = adapterDiagnosticSecretPattern.ReplaceAllString(value, "[redacted]")
	return value
}

func truncateAdapterDiagnosticString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	if maxBytes <= len("...[truncated]") {
		return value[:maxBytes]
	}
	limit := maxBytes - len("...[truncated]")
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit] + "...[truncated]"
}

func mergeAdapterDiagnosticMetadata(existing any, next any) any {
	existingMap, existingOK := existing.(map[string]any)
	nextMap, nextOK := next.(map[string]any)
	if !existingOK || len(existingMap) == 0 {
		if nextOK {
			return cloneDiagnosticMap(nextMap)
		}
		return next
	}
	if !nextOK || len(nextMap) == 0 {
		return cloneDiagnosticMap(existingMap)
	}
	out := cloneDiagnosticMap(existingMap)
	for key, value := range nextMap {
		if current, ok := out[key]; ok {
			out[key] = mergeAdapterDiagnosticMetadata(current, value)
		} else if nested, ok := value.(map[string]any); ok {
			out[key] = cloneDiagnosticMap(nested)
		} else {
			out[key] = value
		}
	}
	return out
}

func cloneDiagnosticMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			out[key] = cloneDiagnosticMap(nested)
			continue
		}
		out[key] = value
	}
	return out
}

func mergeAdapterDiagnosticIntoMetadata(metadata map[string]any, key string, value any) {
	if key == "diagnostic" {
		metadata[key] = mergeAdapterDiagnosticMetadata(metadata[key], value)
		return
	}
	metadata[key] = value
}

func adapterDiagnosticListContainsFold(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "*" || strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func adapterDiagnosticListContains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	if want == "" {
		return false
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "*" || value == want {
			return true
		}
	}
	return false
}

func adapterDiagnosticSmallInt(value int) string {
	return strconv.Itoa(value)
}
