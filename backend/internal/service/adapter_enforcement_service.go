package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	coderws "github.com/coder/websocket"
)

type AdapterEnforcementConfig struct {
	Enabled            bool
	DiagnosticSampling AdapterDiagnosticSamplingConfig
}

type AdapterEnforcementInput struct {
	RequestID          string
	UserID             int64
	APIKeyID           int64
	GroupID            int64
	GroupPlatform      string
	Method             string
	Path               string
	Capability         string
	Model              string
	Headers            map[string]string
	Payload            []byte
	RequestPayloadHash string
	APIKey             *APIKey
	User               *User
	Subscription       *UserSubscription
}

type AdapterEnforcementResult struct {
	Handled  bool
	Response adapterclient.Response
	Policy   *RoutePolicy
	Provider *AdapterProvider
}

type AdapterWebSocketEnforcementResult struct {
	Handled  bool
	Tunnel   adapterclient.WSTunnel
	Policy   *RoutePolicy
	Provider *AdapterProvider
}

const adapterUsageTrailerHeader = "X-Sub2API-Adapter-Usage"

type AdapterEnforcementService struct {
	cfg                AdapterEnforcementConfig
	routePolicySvc     *RoutePolicyService
	adapterProviderSvc *AdapterProviderService
	adapterRequestSvc  *AdapterRequestService
	adapterUsageSvc    *AdapterUsageService
	client             adapterclient.Client
	billingRepo        UsageBillingRepository
}

func NewAdapterEnforcementService(
	cfg AdapterEnforcementConfig,
	routePolicySvc *RoutePolicyService,
	adapterProviderSvc *AdapterProviderService,
	adapterRequestSvc *AdapterRequestService,
	client adapterclient.Client,
	billingRepo ...UsageBillingRepository,
) *AdapterEnforcementService {
	var repo UsageBillingRepository
	if len(billingRepo) > 0 {
		repo = billingRepo[0]
	}
	return &AdapterEnforcementService{
		cfg:                cfg,
		routePolicySvc:     routePolicySvc,
		adapterProviderSvc: adapterProviderSvc,
		adapterRequestSvc:  adapterRequestSvc,
		client:             client,
		billingRepo:        repo,
	}
}

func (s *AdapterEnforcementService) SetAdapterUsageService(adapterUsageSvc *AdapterUsageService) {
	if s != nil {
		s.adapterUsageSvc = adapterUsageSvc
	}
}

func (s *AdapterEnforcementService) Enforce(ctx context.Context, input AdapterEnforcementInput) (AdapterEnforcementResult, error) {
	if s == nil || !s.cfg.Enabled || s.routePolicySvc == nil || s.adapterProviderSvc == nil || s.client == nil {
		return AdapterEnforcementResult{}, nil
	}
	input = normalizeAdapterEnforcementInput(input)
	if isCorePlatform(input.GroupPlatform) {
		return AdapterEnforcementResult{}, nil
	}

	policy, err := s.matchPolicy(ctx, input)
	if err != nil || policy == nil || policy.AdapterProviderID == nil {
		return AdapterEnforcementResult{}, err
	}
	provider, err := s.adapterProviderSvc.GetByID(ctx, *policy.AdapterProviderID)
	if err != nil {
		return AdapterEnforcementResult{}, err
	}
	if provider.Status != AdapterProviderStatusActive || provider.validate() != nil {
		return AdapterEnforcementResult{}, nil
	}

	req := adapterclient.Request{
		RequestID:   input.RequestID,
		UserID:      input.UserID,
		APIKeyID:    input.APIKeyID,
		GroupID:     input.GroupID,
		Provider:    provider.Slug,
		Capability:  firstNonEmptyAdapterString(input.Capability, policy.MatchCapability),
		Model:       input.Model,
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      input.Method,
		Path:        input.Path,
		Headers:     cloneHeaderMap(input.Headers),
		Payload:     append([]byte(nil), input.Payload...),
	}

	start := time.Now()
	resp, callErr := s.client.Do(ctx, req)
	durationMS := int(time.Since(start).Milliseconds())
	billingMeta, billingErr := s.bill(ctx, input, resp)
	diagnostics := newAdapterDiagnosticSampler(s.cfg.DiagnosticSampling, input, provider, resp, callErr, billingErr)
	if diagnostics != nil {
		billingMeta["diagnostic"] = diagnostics.initialMetadata(input, provider, resp, callErr, billingErr)
	}
	auditRecord, _ := s.audit(ctx, input, policy, provider, resp, durationMS, callErr, billingMeta, billingErr)
	usageRecord, _ := s.recordUsage(ctx, input, policy, provider, resp, durationMS, callErr, billingMeta, billingErr)
	if callErr == nil && billingErr != nil {
		callErr = billingErr
	}
	if callErr == nil && resp.Stream && resp.BodyStream != nil {
		streamResp := resp
		observer := newAdapterStreamUsageObserver(resp.ContentType)
		diagnosticObserver := diagnostics.newStreamObserver(resp.ContentType)
		if len(resp.Trailers) > 0 || observer != nil || diagnosticObserver != nil {
			resp.BodyStream = newAdapterFinalizingReadCloser(resp.BodyStream, observer, diagnosticObserver, func() {
				finalUsage, source, ok := resolveFinalStreamingUsage(streamResp, observer)
				if !ok {
					if diagnosticMeta := diagnostics.streamMetadata(diagnosticObserver, source); len(diagnosticMeta) > 0 {
						s.updateAuditWithDiagnostic(ctx, auditRecord, diagnosticMeta)
						s.updateUsageWithDiagnostic(ctx, usageRecord, diagnosticMeta)
					}
					return
				}
				s.finalizeStreamingUsage(ctx, input, policy, provider, streamResp, auditRecord, usageRecord, finalUsage, source, diagnostics.streamMetadata(diagnosticObserver, source))
			})
		}
	}

	return AdapterEnforcementResult{
		Handled:  true,
		Response: resp,
		Policy:   policy.Clone(),
		Provider: provider.Clone(),
	}, callErr
}

func (s *AdapterEnforcementService) EnforceWebSocket(ctx context.Context, input AdapterEnforcementInput) (AdapterWebSocketEnforcementResult, error) {
	if s == nil || !s.cfg.Enabled || s.routePolicySvc == nil || s.adapterProviderSvc == nil || s.client == nil {
		return AdapterWebSocketEnforcementResult{}, nil
	}
	wsClient, ok := s.client.(adapterclient.WebSocketClient)
	if !ok || wsClient == nil {
		return AdapterWebSocketEnforcementResult{}, nil
	}
	input = normalizeAdapterEnforcementInput(input)
	if isCorePlatform(input.GroupPlatform) {
		return AdapterWebSocketEnforcementResult{}, nil
	}
	policy, err := s.matchPolicy(ctx, input)
	if err != nil || policy == nil || policy.AdapterProviderID == nil {
		return AdapterWebSocketEnforcementResult{}, err
	}
	provider, err := s.adapterProviderSvc.GetByID(ctx, *policy.AdapterProviderID)
	if err != nil {
		return AdapterWebSocketEnforcementResult{}, err
	}
	if provider.Status != AdapterProviderStatusActive || provider.validate() != nil {
		return AdapterWebSocketEnforcementResult{}, nil
	}

	req := adapterclient.Request{
		RequestID:   input.RequestID,
		UserID:      input.UserID,
		APIKeyID:    input.APIKeyID,
		GroupID:     input.GroupID,
		Provider:    provider.Slug,
		Capability:  firstNonEmptyAdapterString(input.Capability, policy.MatchCapability),
		Model:       input.Model,
		RouteTarget: capabilityrouter.TargetNewAPIAdapter,
		Method:      input.Method,
		Path:        input.Path,
		Headers:     cloneHeaderMap(input.Headers),
		Payload:     append([]byte(nil), input.Payload...),
	}

	start := time.Now()
	tunnel, callErr := wsClient.OpenWebSocket(ctx, req)
	durationMS := int(time.Since(start).Milliseconds())
	resp := adapterclient.Response{Status: adapterclient.StatusPending, AdapterStatus: http.StatusSwitchingProtocols}
	if callErr != nil {
		resp.Status = adapterclient.StatusFailed
		resp.AdapterStatus = http.StatusBadGateway
		resp.ErrorMessage = callErr.Error()
	}
	billingMeta, billingErr := s.bill(ctx, input, resp)
	billingMeta["websocket"] = true
	billingMeta["transport"] = "websocket"
	diagnostics := newAdapterDiagnosticSampler(s.cfg.DiagnosticSampling, input, provider, resp, callErr, billingErr)
	if diagnostics != nil {
		billingMeta["diagnostic"] = diagnostics.initialMetadata(input, provider, resp, callErr, billingErr)
		if websocketMeta := diagnostics.websocketOpenMetadata(); len(websocketMeta) > 0 {
			billingMeta["diagnostic"] = mergeAdapterDiagnosticMetadata(billingMeta["diagnostic"], websocketMeta)
		}
	}
	auditRecord, _ := s.audit(ctx, input, policy, provider, resp, durationMS, callErr, billingMeta, billingErr)
	usageRecord, _ := s.recordUsage(ctx, input, policy, provider, resp, durationMS, callErr, billingMeta, billingErr)
	if callErr == nil && billingErr != nil {
		callErr = billingErr
	}
	if callErr != nil {
		return AdapterWebSocketEnforcementResult{Handled: true, Policy: policy.Clone(), Provider: provider.Clone()}, callErr
	}
	tunnel = s.wrapFinalizingWebSocketTunnel(ctx, input, policy, provider, resp, tunnel, auditRecord, usageRecord, diagnostics)
	return AdapterWebSocketEnforcementResult{
		Handled:  true,
		Tunnel:   tunnel,
		Policy:   policy.Clone(),
		Provider: provider.Clone(),
	}, nil
}

func (s *AdapterEnforcementService) wrapFinalizingWebSocketTunnel(ctx context.Context, input AdapterEnforcementInput, policy *RoutePolicy, provider *AdapterProvider, resp adapterclient.Response, tunnel adapterclient.WSTunnel, auditRecord *AdapterRequestRecord, usageRecord *AdapterUsageRecord, diagnostics *adapterDiagnosticSampler) adapterclient.WSTunnel {
	if s == nil || tunnel == nil {
		return tunnel
	}
	diagnosticObserver := diagnostics.newWebSocketObserver()
	return &adapterFinalizingWSTunnel{
		inner: tunnel,
		onUsage: func(usage adapterclient.Usage, source string) {
			s.finalizeWebSocketUsage(ctx, input, policy, provider, resp, auditRecord, usageRecord, usage, source, diagnostics.websocketMetadata(diagnosticObserver, source))
		},
		onDiagnostic: func() {
			diagnosticMeta := diagnostics.websocketMetadata(diagnosticObserver, "")
			if len(diagnosticMeta) == 0 {
				return
			}
			s.updateAuditWithDiagnostic(ctx, auditRecord, diagnosticMeta)
			s.updateUsageWithDiagnostic(ctx, usageRecord, diagnosticMeta)
		},
		diagnostics: diagnosticObserver,
	}
}

func resolveFinalStreamingUsage(resp adapterclient.Response, observer *adapterStreamUsageObserver) (adapterclient.Usage, string, bool) {
	if finalUsage, ok := parseAdapterUsageTrailer(resp.Trailers); ok {
		return finalUsage, "trailer", true
	}
	if observer != nil {
		if finalUsage, ok := observer.Usage(); ok {
			return finalUsage, "sse_final_chunk", true
		}
	}
	return adapterclient.Usage{}, "", false
}

type adapterStreamUsageObserver struct {
	pendingLine      string
	dataLines        []string
	dataBytes        int
	dataOverflow     bool
	usage            adapterclient.Usage
	hasUsage         bool
	currentEventSeen bool
}

const adapterSSEMaxEventBytes = 256 * 1024

func newAdapterStreamUsageObserver(contentType string) *adapterStreamUsageObserver {
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return nil
	}
	return &adapterStreamUsageObserver{}
}

func (o *adapterStreamUsageObserver) Observe(chunk []byte) {
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
	if len(text) > adapterSSEMaxEventBytes {
		text = text[len(text)-adapterSSEMaxEventBytes:]
	}
	o.pendingLine = text
}

func (o *adapterStreamUsageObserver) Flush() {
	if o == nil {
		return
	}
	if strings.TrimSpace(o.pendingLine) != "" {
		o.observeLine(o.pendingLine)
		o.pendingLine = ""
	}
	o.processEvent()
}

func (o *adapterStreamUsageObserver) Usage() (adapterclient.Usage, bool) {
	if o == nil || !o.hasUsage {
		return adapterclient.Usage{}, false
	}
	return o.usage, true
}

func (o *adapterStreamUsageObserver) observeLine(line string) {
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
	if !ok || field != "data" {
		return
	}
	if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}
	if o.dataBytes+len(value) > adapterSSEMaxEventBytes {
		o.dataOverflow = true
		return
	}
	o.dataLines = append(o.dataLines, value)
	o.dataBytes += len(value)
}

func (o *adapterStreamUsageObserver) processEvent() {
	if o == nil || !o.currentEventSeen {
		return
	}
	defer func() {
		o.dataLines = nil
		o.dataBytes = 0
		o.dataOverflow = false
		o.currentEventSeen = false
	}()
	if o.dataOverflow || len(o.dataLines) == 0 {
		return
	}
	data := strings.TrimSpace(strings.Join(o.dataLines, "\n"))
	if data == "" || data == "[DONE]" {
		return
	}
	if usage, ok := parseAdapterSSEDataUsage(data); ok {
		o.mergeUsage(usage)
		o.hasUsage = true
	}
}

func (o *adapterStreamUsageObserver) mergeUsage(usage adapterclient.Usage) {
	if o == nil {
		return
	}
	if !o.hasUsage {
		o.usage = usage
		return
	}
	previousBillableWasTokenSum := o.usage.BillableUnit > 0 && o.usage.BillableUnit == o.usage.InputUnits+o.usage.OutputUnits
	if usage.InputUnits > 0 {
		o.usage.InputUnits = usage.InputUnits
	}
	if usage.OutputUnits > 0 {
		o.usage.OutputUnits = usage.OutputUnits
	}
	if usage.BillableUnit > 0 && usage.InputUnits > 0 && usage.OutputUnits > 0 {
		o.usage.BillableUnit = usage.BillableUnit
	} else if sum := o.usage.InputUnits + o.usage.OutputUnits; sum > 0 && (o.usage.BillableUnit <= 0 || previousBillableWasTokenSum) {
		o.usage.BillableUnit = sum
	}
	if usage.CostUSD > 0 {
		o.usage.CostUSD = usage.CostUSD
	}
}

func parseAdapterSSEDataUsage(data string) (adapterclient.Usage, bool) {
	data = strings.TrimSpace(data)
	if data == "" || data == "[DONE]" || !strings.HasPrefix(data, "{") {
		return adapterclient.Usage{}, false
	}
	var envelope struct {
		Usage         json.RawMessage `json:"usage"`
		Response      json.RawMessage `json:"response"`
		Message       json.RawMessage `json:"message"`
		UsageMetadata json.RawMessage `json:"usageMetadata"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return adapterclient.Usage{}, false
	}
	if len(envelope.Usage) > 0 && strings.TrimSpace(string(envelope.Usage)) != "null" {
		return parseAdapterUsagePayloadJSON(envelope.Usage)
	}
	if usage, ok := parseAdapterNestedUsageJSON(envelope.Response, "usage"); ok {
		return usage, true
	}
	if usage, ok := parseAdapterNestedUsageJSON(envelope.Message, "usage"); ok {
		return usage, true
	}
	if usage, ok := parseAdapterGeminiUsageMetadataJSON(envelope.UsageMetadata); ok {
		return usage, true
	}
	if usage, ok := parseAdapterNestedGeminiUsageMetadataJSON(envelope.Response); ok {
		return usage, true
	}
	return parseAdapterUsagePayloadJSON([]byte(data))
}

func parseAdapterWebSocketEventUsage(payload []byte) (adapterclient.Usage, bool) {
	data := strings.TrimSpace(string(payload))
	if data == "" || !strings.HasPrefix(data, "{") {
		return adapterclient.Usage{}, false
	}
	var envelope struct {
		Usage    json.RawMessage `json:"usage"`
		Response json.RawMessage `json:"response"`
		Message  json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return adapterclient.Usage{}, false
	}
	if len(envelope.Usage) > 0 && strings.TrimSpace(string(envelope.Usage)) != "null" {
		return parseAdapterUsagePayloadJSON(envelope.Usage)
	}
	if usage, ok := parseAdapterNestedUsageJSON(envelope.Response, "usage"); ok {
		return usage, true
	}
	if usage, ok := parseAdapterNestedUsageJSON(envelope.Message, "usage"); ok {
		return usage, true
	}
	return parseAdapterUsagePayloadJSON([]byte(data))
}

func parseAdapterNestedUsageJSON(raw json.RawMessage, field string) (adapterclient.Usage, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return adapterclient.Usage{}, false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return adapterclient.Usage{}, false
	}
	nested, ok := payload[field]
	if !ok || len(nested) == 0 || strings.TrimSpace(string(nested)) == "null" {
		return adapterclient.Usage{}, false
	}
	return parseAdapterUsagePayloadJSON(nested)
}

func parseAdapterNestedGeminiUsageMetadataJSON(raw json.RawMessage) (adapterclient.Usage, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return adapterclient.Usage{}, false
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return adapterclient.Usage{}, false
	}
	return parseAdapterGeminiUsageMetadataJSON(payload["usageMetadata"])
}

func parseAdapterGeminiUsageMetadataJSON(raw json.RawMessage) (adapterclient.Usage, bool) {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return adapterclient.Usage{}, false
	}
	var payload struct {
		PromptTokenCount        int64 `json:"promptTokenCount"`
		CandidatesTokenCount    int64 `json:"candidatesTokenCount"`
		ThoughtsTokenCount      int64 `json:"thoughtsTokenCount"`
		CachedContentTokenCount int64 `json:"cachedContentTokenCount"`
		TotalTokenCount         int64 `json:"totalTokenCount"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return adapterclient.Usage{}, false
	}
	inputUnits := payload.PromptTokenCount
	if payload.CachedContentTokenCount > 0 && inputUnits >= payload.CachedContentTokenCount {
		inputUnits -= payload.CachedContentTokenCount
	}
	outputUnits := payload.CandidatesTokenCount + payload.ThoughtsTokenCount
	billableUnit := payload.TotalTokenCount
	if billableUnit <= 0 && inputUnits+outputUnits > 0 {
		billableUnit = inputUnits + outputUnits
	}
	usage := adapterclient.Usage{
		InputUnits:   inputUnits,
		OutputUnits:  outputUnits,
		BillableUnit: billableUnit,
	}
	if usage.InputUnits <= 0 && usage.OutputUnits <= 0 && usage.BillableUnit <= 0 {
		return adapterclient.Usage{}, false
	}
	return usage, true
}

func parseAdapterUsagePayloadJSON(raw []byte) (adapterclient.Usage, bool) {
	var payload struct {
		InputUnits       int64   `json:"input_units"`
		OutputUnits      int64   `json:"output_units"`
		BillableUnit     int64   `json:"billable_unit"`
		InputTokens      int64   `json:"input_tokens"`
		OutputTokens     int64   `json:"output_tokens"`
		PromptTokens     int64   `json:"prompt_tokens"`
		CompletionTokens int64   `json:"completion_tokens"`
		TotalTokens      int64   `json:"total_tokens"`
		CostUSD          float64 `json:"cost_usd"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return adapterclient.Usage{}, false
	}
	usage := adapterclient.Usage{
		InputUnits:   firstPositiveInt64(payload.InputUnits, payload.PromptTokens, payload.InputTokens),
		OutputUnits:  firstPositiveInt64(payload.OutputUnits, payload.CompletionTokens, payload.OutputTokens),
		BillableUnit: firstPositiveInt64(payload.BillableUnit, payload.TotalTokens),
		CostUSD:      payload.CostUSD,
	}
	if usage.BillableUnit <= 0 && usage.InputUnits+usage.OutputUnits > 0 {
		usage.BillableUnit = usage.InputUnits + usage.OutputUnits
	}
	if usage.InputUnits <= 0 && usage.OutputUnits <= 0 && usage.BillableUnit <= 0 && usage.CostUSD <= 0 {
		return adapterclient.Usage{}, false
	}
	return usage, true
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *AdapterEnforcementService) matchPolicy(ctx context.Context, input AdapterEnforcementInput) (*RoutePolicy, error) {
	policies, err := s.routePolicySvc.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, policy := range policies {
		if !adapterPolicyMatches(policy, input) {
			continue
		}
		return policy.Clone(), nil
	}
	return nil, nil
}

func (s *AdapterEnforcementService) bill(ctx context.Context, input AdapterEnforcementInput, resp adapterclient.Response) (map[string]any, error) {
	meta := map[string]any{
		"cost_usd":      resp.Usage.CostUSD,
		"input_units":   resp.Usage.InputUnits,
		"output_units":  resp.Usage.OutputUnits,
		"billable_unit": resp.Usage.BillableUnit,
	}
	if s == nil || s.billingRepo == nil {
		meta["billing_skipped_reason"] = "billing_repo_unavailable"
		return meta, nil
	}
	if resp.Status != adapterclient.StatusSucceeded || resp.Usage.CostUSD <= 0 {
		meta["billing_skipped_reason"] = "not_billable"
		return meta, nil
	}
	user := input.User
	if user == nil && input.UserID > 0 {
		user = &User{ID: input.UserID}
	}
	apiKey := input.APIKey
	if apiKey == nil && input.APIKeyID > 0 {
		apiKey = &APIKey{ID: input.APIKeyID, UserID: input.UserID}
	}
	if user == nil || user.ID <= 0 || apiKey == nil || apiKey.ID <= 0 {
		meta["billing_skipped_reason"] = "missing_owner"
		return meta, nil
	}

	cmd := &UsageBillingCommand{
		RequestID:          input.RequestID,
		APIKeyID:           apiKey.ID,
		UserID:             user.ID,
		Model:              input.Model,
		RequestPayloadHash: strings.TrimSpace(input.RequestPayloadHash),
		InputTokens:        adapterUsageInt(resp.Usage.InputUnits),
		OutputTokens:       adapterUsageInt(resp.Usage.OutputUnits),
		BalanceCost:        resp.Usage.CostUSD,
		APIKeyQuotaCost:    adapterAPIKeyQuotaCost(apiKey, resp.Usage.CostUSD),
		APIKeyRateLimitCost: adapterAPIKeyRateLimitCost(apiKey,
			resp.Usage.CostUSD),
	}
	if isAdapterSubscriptionBilling(apiKey, input.Subscription) {
		cmd.SubscriptionID = &input.Subscription.ID
		cmd.SubscriptionCost = resp.Usage.CostUSD
		cmd.BalanceCost = 0
	}
	cmd.Normalize()
	result, err := s.billingRepo.Apply(ctx, cmd)
	if err != nil {
		meta["billing_error"] = err.Error()
		return meta, err
	}
	applied := result != nil && result.Applied
	meta["billing_applied"] = applied
	meta["billing_fingerprint"] = cmd.RequestFingerprint
	return meta, nil
}

func (s *AdapterEnforcementService) audit(ctx context.Context, input AdapterEnforcementInput, policy *RoutePolicy, provider *AdapterProvider, resp adapterclient.Response, durationMS int, callErr error, billingMeta map[string]any, billingErr error) (*AdapterRequestRecord, error) {
	if s == nil || s.adapterRequestSvc == nil || provider == nil {
		return nil, nil
	}
	var groupID *int64
	if input.GroupID > 0 {
		id := input.GroupID
		groupID = &id
	}
	var statusCode *int
	if resp.AdapterStatus > 0 {
		status := resp.AdapterStatus
		statusCode = &status
	}
	duration := durationMS
	errorMessage := ""
	if callErr != nil {
		errorMessage = callErr.Error()
	} else if billingErr != nil {
		errorMessage = billingErr.Error()
	} else if resp.ErrorMessage != "" {
		errorMessage = resp.ErrorMessage
	}
	metadata := map[string]any{
		"policy_id": policy.ID,
		"status":    string(resp.Status),
	}
	for key, value := range billingMeta {
		mergeAdapterDiagnosticIntoMetadata(metadata, key, value)
	}
	record, err := s.adapterRequestSvc.Create(ctx, &AdapterRequestRecord{
		RequestID:         input.RequestID,
		UserID:            input.UserID,
		APIKeyID:          input.APIKeyID,
		GroupID:           groupID,
		AdapterProviderID: provider.ID,
		Provider:          provider.Slug,
		Capability:        firstNonEmptyAdapterString(input.Capability, policy.MatchCapability),
		RouteTarget:       string(capabilityrouter.TargetNewAPIAdapter),
		Method:            input.Method,
		Path:              input.Path,
		Model:             input.Model,
		StatusCode:        statusCode,
		DurationMS:        &duration,
		ErrorMessage:      errorMessage,
		Metadata:          metadata,
	})
	return record, err
}

func (s *AdapterEnforcementService) recordUsage(ctx context.Context, input AdapterEnforcementInput, policy *RoutePolicy, provider *AdapterProvider, resp adapterclient.Response, durationMS int, callErr error, billingMeta map[string]any, billingErr error) (*AdapterUsageRecord, error) {
	if s == nil || s.adapterUsageSvc == nil || provider == nil {
		return nil, nil
	}
	var groupID *int64
	if input.GroupID > 0 {
		id := input.GroupID
		groupID = &id
	}
	var policyID *int64
	if policy != nil && policy.ID > 0 {
		id := policy.ID
		policyID = &id
	}
	policyCapability := ""
	if policy != nil {
		policyCapability = policy.MatchCapability
	}
	var statusCode *int
	if resp.AdapterStatus > 0 {
		status := resp.AdapterStatus
		statusCode = &status
	}
	duration := durationMS
	errorMessage := ""
	if callErr != nil {
		errorMessage = callErr.Error()
	} else if billingErr != nil {
		errorMessage = billingErr.Error()
	} else if resp.ErrorMessage != "" {
		errorMessage = resp.ErrorMessage
	}
	metadata := map[string]any{
		"source":       "adapter_enforcement",
		"route_target": string(capabilityrouter.TargetNewAPIAdapter),
	}
	if value, ok := billingMeta["websocket"]; ok {
		metadata["websocket"] = value
	}
	if value, ok := billingMeta["transport"]; ok {
		metadata["transport"] = value
	}
	if value, ok := billingMeta["diagnostic"]; ok {
		mergeAdapterDiagnosticIntoMetadata(metadata, "diagnostic", value)
	}
	record, err := s.adapterUsageSvc.RecordAdapterResult(ctx, AdapterUsageRecordInput{
		RequestID:         input.RequestID,
		UserID:            input.UserID,
		APIKeyID:          input.APIKeyID,
		GroupID:           groupID,
		AdapterProviderID: provider.ID,
		RoutePolicyID:     policyID,
		Provider:          provider.Slug,
		Capability:        firstNonEmptyAdapterString(input.Capability, policyCapability),
		Model:             input.Model,
		Method:            input.Method,
		Path:              input.Path,
		StatusCode:        statusCode,
		DurationMS:        &duration,
		AdapterStatus:     resp.Status,
		ErrorMessage:      errorMessage,
		Usage:             resp.Usage,
		BillingMetadata:   billingMeta,
		Metadata:          metadata,
	})
	return record, err
}

func (s *AdapterEnforcementService) finalizeStreamingUsage(ctx context.Context, input AdapterEnforcementInput, policy *RoutePolicy, provider *AdapterProvider, resp adapterclient.Response, auditRecord *AdapterRequestRecord, usageRecord *AdapterUsageRecord, finalUsage adapterclient.Usage, source string, diagnosticMeta map[string]any) {
	if s == nil || provider == nil {
		return
	}
	finalResp := resp
	finalResp.Usage = finalUsage
	finalResp.Status = adapterclient.StatusSucceeded
	billingMeta, billingErr := s.bill(ctx, input, finalResp)
	billingMeta["stream_usage_finalized"] = true
	billingMeta["usage_source"] = source
	if billingErr != nil {
		billingMeta["billing_error"] = billingErr.Error()
	}
	if len(diagnosticMeta) > 0 {
		billingMeta["diagnostic"] = diagnosticMeta
	}
	durationMS := 0
	if usageRecord != nil && usageRecord.DurationMS != nil {
		durationMS = *usageRecord.DurationMS
	} else if auditRecord != nil && auditRecord.DurationMS != nil {
		durationMS = *auditRecord.DurationMS
	}
	_ = s.updateAuditWithFinalUsage(ctx, auditRecord, finalResp, billingMeta, billingErr, durationMS)
	_ = s.updateUsageWithFinalUsage(ctx, usageRecord, finalResp, billingMeta, billingErr, durationMS)
}

func (s *AdapterEnforcementService) finalizeWebSocketUsage(ctx context.Context, input AdapterEnforcementInput, policy *RoutePolicy, provider *AdapterProvider, resp adapterclient.Response, auditRecord *AdapterRequestRecord, usageRecord *AdapterUsageRecord, finalUsage adapterclient.Usage, source string, diagnosticMeta map[string]any) {
	if s == nil || provider == nil {
		return
	}
	finalResp := resp
	finalResp.Usage = finalUsage
	finalResp.Status = adapterclient.StatusSucceeded
	billingMeta, billingErr := s.bill(ctx, input, finalResp)
	billingMeta["websocket"] = true
	billingMeta["transport"] = "websocket"
	billingMeta["websocket_usage_finalized"] = true
	billingMeta["usage_source"] = source
	if billingErr != nil {
		billingMeta["billing_error"] = billingErr.Error()
	}
	if len(diagnosticMeta) > 0 {
		billingMeta["diagnostic"] = diagnosticMeta
	}
	durationMS := 0
	if usageRecord != nil && usageRecord.DurationMS != nil {
		durationMS = *usageRecord.DurationMS
	} else if auditRecord != nil && auditRecord.DurationMS != nil {
		durationMS = *auditRecord.DurationMS
	}
	_ = s.updateAuditWithFinalUsage(ctx, auditRecord, finalResp, billingMeta, billingErr, durationMS)
	_ = s.updateUsageWithFinalUsage(ctx, usageRecord, finalResp, billingMeta, billingErr, durationMS)
}

func parseAdapterUsageTrailer(trailers http.Header) (adapterclient.Usage, bool) {
	raw := strings.TrimSpace(headerValueCaseInsensitive(trailers, adapterUsageTrailerHeader))
	if raw == "" {
		return adapterclient.Usage{}, false
	}
	return parseAdapterUsagePayloadJSON([]byte(raw))
}

func headerValueCaseInsensitive(headers http.Header, key string) string {
	if value := headers.Get(key); strings.TrimSpace(value) != "" {
		return value
	}
	for headerKey, values := range headers {
		if !strings.EqualFold(headerKey, key) || len(values) == 0 {
			continue
		}
		return values[0]
	}
	return ""
}

func (s *AdapterEnforcementService) updateAuditWithFinalUsage(ctx context.Context, record *AdapterRequestRecord, resp adapterclient.Response, billingMeta map[string]any, billingErr error, durationMS int) error {
	if s == nil || s.adapterRequestSvc == nil || record == nil || record.ID <= 0 {
		return nil
	}
	updated := record.Clone()
	if resp.AdapterStatus > 0 {
		status := resp.AdapterStatus
		updated.StatusCode = &status
	}
	if durationMS > 0 {
		duration := durationMS
		updated.DurationMS = &duration
	}
	if billingErr != nil {
		updated.ErrorMessage = billingErr.Error()
	} else if resp.ErrorMessage != "" {
		updated.ErrorMessage = resp.ErrorMessage
	}
	metadata := cloneAnyMap(updated.Metadata)
	for key, value := range billingMeta {
		mergeAdapterDiagnosticIntoMetadata(metadata, key, value)
	}
	updated.Metadata = metadata
	_, err := s.adapterRequestSvc.Update(ctx, updated)
	return err
}

func (s *AdapterEnforcementService) updateUsageWithFinalUsage(ctx context.Context, record *AdapterUsageRecord, resp adapterclient.Response, billingMeta map[string]any, billingErr error, durationMS int) error {
	if s == nil || s.adapterUsageSvc == nil || record == nil || record.ID <= 0 {
		return nil
	}
	updated := record.Clone()
	if resp.AdapterStatus > 0 {
		status := resp.AdapterStatus
		updated.StatusCode = &status
	}
	if durationMS > 0 {
		duration := durationMS
		updated.DurationMS = &duration
	}
	updated.Status = string(resp.Status)
	if updated.Status == "" {
		updated.Status = string(adapterclient.StatusSucceeded)
	}
	if billingErr != nil {
		updated.ErrorMessage = billingErr.Error()
	} else if resp.ErrorMessage != "" {
		updated.ErrorMessage = resp.ErrorMessage
	}
	updated.InputUnits = adapterUsageInt(resp.Usage.InputUnits)
	updated.OutputUnits = adapterUsageInt(resp.Usage.OutputUnits)
	updated.BillableUnits = adapterUsageInt(resp.Usage.BillableUnit)
	updated.BillableUnit = adapterUsageInt(resp.Usage.BillableUnit)
	updated.CostUSD = resp.Usage.CostUSD
	updated.BillingApplied = boolValue(billingMeta["billing_applied"])
	if fingerprint, _ := billingMeta["billing_fingerprint"].(string); strings.TrimSpace(fingerprint) != "" {
		updated.BillingFingerprint = strings.TrimSpace(fingerprint)
	}
	metadata := cloneAnyMap(updated.Metadata)
	for key, value := range billingMeta {
		mergeAdapterDiagnosticIntoMetadata(metadata, key, value)
	}
	updated.Metadata = metadata
	_, err := s.adapterUsageSvc.UpdateAdapterResult(ctx, updated)
	return err
}

func (s *AdapterEnforcementService) updateAuditWithDiagnostic(ctx context.Context, record *AdapterRequestRecord, diagnosticMeta map[string]any) error {
	if s == nil || s.adapterRequestSvc == nil || record == nil || record.ID <= 0 || len(diagnosticMeta) == 0 {
		return nil
	}
	updated := record.Clone()
	metadata := cloneAnyMap(updated.Metadata)
	mergeAdapterDiagnosticIntoMetadata(metadata, "diagnostic", diagnosticMeta)
	updated.Metadata = metadata
	_, err := s.adapterRequestSvc.Update(ctx, updated)
	return err
}

func (s *AdapterEnforcementService) updateUsageWithDiagnostic(ctx context.Context, record *AdapterUsageRecord, diagnosticMeta map[string]any) error {
	if s == nil || s.adapterUsageSvc == nil || record == nil || record.ID <= 0 || len(diagnosticMeta) == 0 {
		return nil
	}
	updated := record.Clone()
	metadata := cloneAnyMap(updated.Metadata)
	mergeAdapterDiagnosticIntoMetadata(metadata, "diagnostic", diagnosticMeta)
	updated.Metadata = metadata
	_, err := s.adapterUsageSvc.UpdateAdapterResult(ctx, updated)
	return err
}

type adapterFinalizingReadCloser struct {
	inner              io.ReadCloser
	observer           *adapterStreamUsageObserver
	diagnosticObserver *adapterStreamDiagnosticObserver
	once               sync.Once
	finalize           func()
}

func newAdapterFinalizingReadCloser(inner io.ReadCloser, observer *adapterStreamUsageObserver, diagnosticObserver *adapterStreamDiagnosticObserver, finalize func()) io.ReadCloser {
	return &adapterFinalizingReadCloser{inner: inner, observer: observer, diagnosticObserver: diagnosticObserver, finalize: finalize}
}

func (r *adapterFinalizingReadCloser) Read(p []byte) (int, error) {
	n, err := r.inner.Read(p)
	if n > 0 && r.observer != nil {
		r.observer.Observe(p[:n])
	}
	if n > 0 && r.diagnosticObserver != nil {
		r.diagnosticObserver.Observe(p[:n])
	}
	return n, err
}

func (r *adapterFinalizingReadCloser) Close() error {
	err := r.inner.Close()
	r.once.Do(func() {
		if r.observer != nil {
			r.observer.Flush()
		}
		if r.diagnosticObserver != nil {
			r.diagnosticObserver.Flush()
		}
		if r.finalize != nil {
			r.finalize()
		}
	})
	return err
}

type adapterFinalizingWSTunnel struct {
	inner        adapterclient.WSTunnel
	usage        adapterclient.Usage
	hasUsage     bool
	source       string
	onUsage      func(adapterclient.Usage, string)
	onDiagnostic func()
	diagnostics  *adapterEventDiagnosticObserver
	once         sync.Once
	observeLock  sync.Mutex
}

func (t *adapterFinalizingWSTunnel) Read(ctx context.Context) (coderws.MessageType, []byte, error) {
	msgType, payload, err := t.inner.Read(ctx)
	if err == nil && msgType == coderws.MessageText {
		t.observe(payload)
	}
	return msgType, payload, err
}

func (t *adapterFinalizingWSTunnel) Write(ctx context.Context, msgType coderws.MessageType, payload []byte) error {
	return t.inner.Write(ctx, msgType, payload)
}

func (t *adapterFinalizingWSTunnel) Close() error {
	err := t.inner.Close()
	t.once.Do(func() {
		t.observeLock.Lock()
		usage := t.usage
		source := t.source
		hasUsage := t.hasUsage
		t.observeLock.Unlock()
		if hasUsage && t.onUsage != nil {
			t.onUsage(usage, source)
		} else if !hasUsage && t.onDiagnostic != nil {
			t.onDiagnostic()
		}
	})
	return err
}

func (t *adapterFinalizingWSTunnel) observe(payload []byte) {
	usage, ok := parseAdapterWebSocketEventUsage(payload)
	if t.diagnostics != nil {
		t.diagnostics.Observe(payload, ok)
	}
	if !ok {
		return
	}
	t.observeLock.Lock()
	defer t.observeLock.Unlock()
	t.mergeUsageLocked(usage)
	t.hasUsage = true
	t.source = "websocket_event"
}

func (t *adapterFinalizingWSTunnel) mergeUsageLocked(usage adapterclient.Usage) {
	if !t.hasUsage {
		t.usage = usage
		return
	}
	previousBillableWasTokenSum := t.usage.BillableUnit > 0 && t.usage.BillableUnit == t.usage.InputUnits+t.usage.OutputUnits
	if usage.InputUnits > 0 {
		t.usage.InputUnits = usage.InputUnits
	}
	if usage.OutputUnits > 0 {
		t.usage.OutputUnits = usage.OutputUnits
	}
	if usage.BillableUnit > 0 && usage.InputUnits > 0 && usage.OutputUnits > 0 {
		t.usage.BillableUnit = usage.BillableUnit
	} else if sum := t.usage.InputUnits + t.usage.OutputUnits; sum > 0 && (t.usage.BillableUnit <= 0 || previousBillableWasTokenSum) {
		t.usage.BillableUnit = sum
	}
	if usage.CostUSD > 0 {
		t.usage.CostUSD = usage.CostUSD
	}
}

func adapterUsageInt(value int64) int {
	if value <= 0 {
		return 0
	}
	if value > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(value)
}

func adapterAPIKeyQuotaCost(apiKey *APIKey, cost float64) float64 {
	if apiKey != nil && apiKey.Quota > 0 && cost > 0 {
		return cost
	}
	return 0
}

func adapterAPIKeyRateLimitCost(apiKey *APIKey, cost float64) float64 {
	if apiKey != nil && apiKey.HasRateLimits() && cost > 0 {
		return cost
	}
	return 0
}

func isAdapterSubscriptionBilling(apiKey *APIKey, subscription *UserSubscription) bool {
	return subscription != nil && apiKey != nil && apiKey.Group != nil && apiKey.Group.IsSubscriptionType()
}

func adapterPolicyMatches(policy *RoutePolicy, input AdapterEnforcementInput) bool {
	if policy == nil {
		return false
	}
	policy.normalizeDefaults()
	if policy.Status != RoutePolicyStatusActive {
		return false
	}
	if capabilityrouter.Target(policy.Target) != capabilityrouter.TargetNewAPIAdapter {
		return false
	}
	if policy.AdapterProviderID == nil || *policy.AdapterProviderID <= 0 {
		return false
	}
	if !matchOptional(policy.MatchMethod, input.Method) {
		return false
	}
	if !matchOptional(policy.MatchPath, normalizePolicyPath(input.Path)) {
		return false
	}
	if !matchOptional(policy.MatchModel, input.Model) {
		return false
	}
	if !matchOptional(policy.MatchCapability, input.Capability) {
		return false
	}
	if !matchOptional(policy.MatchGroupPlatform, normalizePolicyPlatform(input.GroupPlatform)) {
		return false
	}
	return true
}

func normalizeAdapterEnforcementInput(input AdapterEnforcementInput) AdapterEnforcementInput {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.Method = strings.ToUpper(strings.TrimSpace(input.Method))
	input.Path = normalizePolicyPath(input.Path)
	input.Model = strings.ToLower(strings.TrimSpace(input.Model))
	input.Capability = strings.ToLower(strings.TrimSpace(input.Capability))
	input.GroupPlatform = normalizePolicyPlatform(input.GroupPlatform)
	return input
}

func matchOptional(pattern, value string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	value = strings.ToLower(strings.TrimSpace(value))
	if pattern == "" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == value
}

func cloneHeaderMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func firstNonEmptyAdapterString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
