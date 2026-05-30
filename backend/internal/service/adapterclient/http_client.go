package adapterclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	coderws "github.com/coder/websocket"
)

var ErrProviderNotConfigured = errors.New("adapter provider not configured")

type HTTPClient struct {
	registry   ProviderRegistry
	httpClient *http.Client
}

func NewHTTPClient(registry ProviderRegistry, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &HTTPClient{
		registry:   registry,
		httpClient: httpClient,
	}
}

func (c *HTTPClient) Do(ctx context.Context, req Request) (Response, error) {
	if err := req.Validate(); err != nil {
		return Response{}, err
	}
	provider, ok := c.findProvider(req.Provider)
	if !ok {
		return Response{}, fmt.Errorf("%w: %s", ErrProviderNotConfigured, req.Provider)
	}

	endpoint, err := adapterExecuteURL(provider.BaseURL)
	if err != nil {
		return Response{}, err
	}
	body, err := json.Marshal(newEnvelope(req))
	if err != nil {
		return Response{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Sub2API-Request-ID", req.RequestID)
	httpReq.Header.Set("X-Sub2API-Route-Target", string(req.RouteTarget))
	applyProviderAuth(httpReq, provider)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return Response{}, err
	}
	if isEventStream(resp.Header.Get("Content-Type")) && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return Response{
			Status:        StatusSucceeded,
			AdapterStatus: resp.StatusCode,
			ContentType:   normalizeContentType(resp.Header.Get("Content-Type")),
			Stream:        true,
			BodyStream:    resp.Body,
			Trailers:      resp.Trailer,
		}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	return decodeAdapterResponse(resp.StatusCode, respBody)
}

func (c *HTTPClient) OpenWebSocket(ctx context.Context, req Request) (WSTunnel, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	provider, ok := c.findProvider(req.Provider)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotConfigured, req.Provider)
	}
	endpoint, err := adapterWebSocketURL(provider.BaseURL)
	if err != nil {
		return nil, err
	}
	headers := http.Header{}
	headers.Set("X-Sub2API-Request-ID", req.RequestID)
	headers.Set("X-Sub2API-Route-Target", string(req.RouteTarget))
	headers.Set("X-Sub2API-Provider", strings.TrimSpace(req.Provider))
	headers.Set("X-Sub2API-Capability", strings.TrimSpace(req.Capability))
	headers.Set("X-Sub2API-User-ID", fmt.Sprintf("%d", req.UserID))
	headers.Set("X-Sub2API-API-Key-ID", fmt.Sprintf("%d", req.APIKeyID))
	headers.Set("X-Sub2API-Group-ID", fmt.Sprintf("%d", req.GroupID))
	headers.Set("X-Sub2API-Method", strings.ToUpper(strings.TrimSpace(req.Method)))
	headers.Set("X-Sub2API-Path", strings.TrimSpace(req.Path))
	if strings.TrimSpace(req.Model) != "" {
		headers.Set("X-Sub2API-Model", strings.TrimSpace(req.Model))
	}
	applyProviderAuthToHeader(headers, provider)

	conn, resp, err := coderws.Dial(ctx, endpoint, &coderws.DialOptions{
		HTTPClient: c.httpClient,
		HTTPHeader: headers,
	})
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		if status > 0 {
			return nil, fmt.Errorf("adapter websocket handshake failed: status %d: %w", status, err)
		}
		return nil, err
	}
	return &coderWebSocketTunnel{conn: conn}, nil
}

func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

func normalizeContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(contentType, ";")[0])
}

func (c *HTTPClient) findProvider(slug string) (ProviderConfig, bool) {
	if c == nil || c.registry == nil {
		return ProviderConfig{}, false
	}
	want := normalizeProviderSlug(slug)
	for _, provider := range c.registry.Providers() {
		if provider.normalizedStatus() != ProviderStatusActive {
			continue
		}
		if err := provider.Validate(); err != nil {
			continue
		}
		if normalizeProviderSlug(provider.Slug) == want {
			return provider, true
		}
	}
	return ProviderConfig{}, false
}

type envelope struct {
	RequestID       string            `json:"request_id"`
	UserID          int64             `json:"user_id"`
	APIKeyID        int64             `json:"api_key_id"`
	GroupID         int64             `json:"group_id"`
	Provider        string            `json:"provider"`
	Capability      string            `json:"capability"`
	Model           string            `json:"model,omitempty"`
	RouteTarget     string            `json:"route_target"`
	BillingCategory string            `json:"billing_category,omitempty"`
	Method          string            `json:"method,omitempty"`
	Path            string            `json:"path,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Payload         json.RawMessage   `json:"payload,omitempty"`
}

func newEnvelope(req Request) envelope {
	env := envelope{
		RequestID:       req.RequestID,
		UserID:          req.UserID,
		APIKeyID:        req.APIKeyID,
		GroupID:         req.GroupID,
		Provider:        strings.TrimSpace(req.Provider),
		Capability:      strings.TrimSpace(req.Capability),
		Model:           strings.TrimSpace(req.Model),
		RouteTarget:     string(req.RouteTarget),
		BillingCategory: strings.TrimSpace(req.BillingCategory),
		Method:          strings.ToUpper(strings.TrimSpace(req.Method)),
		Path:            strings.TrimSpace(req.Path),
		Headers:         cloneStringMap(req.Headers),
	}
	if len(req.Payload) > 0 && json.Valid(req.Payload) {
		env.Payload = append(json.RawMessage(nil), req.Payload...)
	}
	return env
}

func adapterExecuteURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("adapter base url must be http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("adapter base url host is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/internal/adapter/execute"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func adapterWebSocketURL(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("adapter base url must be http, https, ws, or wss")
	}
	if parsed.Host == "" {
		return "", errors.New("adapter base url host is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/internal/adapter/ws"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func applyProviderAuth(req *http.Request, provider ProviderConfig) {
	if req == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(provider.AuthMode)) {
	case "bearer":
		token := strings.TrimSpace(firstCredential(provider.Credentials, "token", "api_key", "key"))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	case "header":
		for key, value := range provider.Credentials {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			req.Header.Set(key, value)
		}
	}
}

func applyProviderAuthToHeader(headers http.Header, provider ProviderConfig) {
	if headers == nil {
		return
	}
	switch strings.ToLower(strings.TrimSpace(provider.AuthMode)) {
	case "bearer":
		token := strings.TrimSpace(firstCredential(provider.Credentials, "token", "api_key", "key"))
		if token != "" {
			headers.Set("Authorization", "Bearer "+token)
		}
	case "header":
		for key, value := range provider.Credentials {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			headers.Set(key, value)
		}
	}
}

type coderWebSocketTunnel struct {
	conn *coderws.Conn
}

func (t *coderWebSocketTunnel) Read(ctx context.Context) (coderws.MessageType, []byte, error) {
	if t == nil || t.conn == nil {
		return coderws.MessageText, nil, errors.New("adapter websocket tunnel is closed")
	}
	return t.conn.Read(ctx)
}

func (t *coderWebSocketTunnel) Write(ctx context.Context, msgType coderws.MessageType, payload []byte) error {
	if t == nil || t.conn == nil {
		return errors.New("adapter websocket tunnel is closed")
	}
	return t.conn.Write(ctx, msgType, payload)
}

func (t *coderWebSocketTunnel) Close() error {
	if t == nil || t.conn == nil {
		return nil
	}
	_ = t.conn.Close(coderws.StatusNormalClosure, "")
	return t.conn.CloseNow()
}

func firstCredential(credentials map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := credentials[key]; strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type adapterHTTPResponse struct {
	Status        Status          `json:"status"`
	AdapterStatus int             `json:"adapter_status"`
	UpstreamID    string          `json:"upstream_id"`
	Usage         Usage           `json:"usage"`
	Body          json.RawMessage `json:"body"`
	ErrorCode     string          `json:"error_code"`
	ErrorMessage  string          `json:"error_message"`
}

func decodeAdapterResponse(statusCode int, body []byte) (Response, error) {
	var decoded adapterHTTPResponse
	if len(body) > 0 {
		_ = json.Unmarshal(body, &decoded)
	}
	resp := Response{
		Status:        decoded.Status,
		AdapterStatus: decoded.AdapterStatus,
		UpstreamID:    decoded.UpstreamID,
		Usage:         decoded.Usage,
		Body:          append([]byte(nil), decoded.Body...),
		ErrorCode:     decoded.ErrorCode,
		ErrorMessage:  decoded.ErrorMessage,
	}
	if resp.AdapterStatus == 0 {
		resp.AdapterStatus = statusCode
	}
	if statusCode < 200 || statusCode >= 300 {
		resp.Status = StatusFailed
		if resp.ErrorCode == "" {
			resp.ErrorCode = "adapter_http_error"
		}
		if resp.ErrorMessage == "" {
			resp.ErrorMessage = http.StatusText(statusCode)
		}
		return resp, nil
	}
	if resp.Status == "" {
		resp.Status = StatusSucceeded
	}
	if len(resp.Body) == 0 && len(body) > 0 && !json.Valid(body) {
		resp.Body = append([]byte(nil), body...)
	}
	return resp, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
