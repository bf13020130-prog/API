package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/adapterclient"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

func AdapterEnforcement(enforcer *service.AdapterEnforcementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || c.Request == nil || enforcer == nil {
			if c != nil {
				c.Next()
			}
			return
		}

		if isAdapterWebSocketUpgrade(c.Request) {
			result, err := enforcer.EnforceWebSocket(c.Request.Context(), buildAdapterEnforcementInput(c, nil))
			if !result.Handled {
				c.Next()
				return
			}
			if err != nil {
				writeAdapterError(c, http.StatusBadGateway, err.Error())
				return
			}
			writeAdapterWebSocket(c, result.Tunnel)
			return
		}

		body, err := readAndRestoreBody(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "failed to read request body"}})
			c.Abort()
			return
		}

		result, err := enforcer.Enforce(c.Request.Context(), buildAdapterEnforcementInput(c, body))
		if !result.Handled {
			c.Next()
			return
		}
		if err != nil {
			writeAdapterError(c, http.StatusBadGateway, err.Error())
			return
		}
		if result.Response.Stream {
			writeAdapterStream(c, result.Response.AdapterStatus, result.Response.ContentType, result.Response.BodyStream)
			return
		}
		writeAdapterResponse(c, result.Response.AdapterStatus, result.Response.Body)
	}
}

func isAdapterWebSocketUpgrade(r *http.Request) bool {
	if r == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	for _, part := range strings.Split(r.Header.Get("Connection"), ",") {
		if strings.EqualFold(strings.TrimSpace(part), "upgrade") {
			return true
		}
	}
	return false
}

func readAndRestoreBody(c *gin.Context) ([]byte, error) {
	if c.Request.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func buildAdapterEnforcementInput(c *gin.Context, body []byte) service.AdapterEnforcementInput {
	apiKey, _ := GetAPIKeyFromContext(c)
	groupID := int64(0)
	userID := int64(0)
	apiKeyID := int64(0)
	groupPlatform := ""
	if apiKey != nil {
		apiKeyID = apiKey.ID
		userID = apiKey.UserID
		if userID <= 0 && apiKey.User != nil {
			userID = apiKey.User.ID
		}
		if apiKey.GroupID != nil {
			groupID = *apiKey.GroupID
		}
		if apiKey.Group != nil {
			if groupID <= 0 {
				groupID = apiKey.Group.ID
			}
			groupPlatform = apiKey.Group.Platform
		}
	}
	if group, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group); ok && group != nil {
		if groupID <= 0 {
			groupID = group.ID
		}
		if groupPlatform == "" {
			groupPlatform = group.Platform
		}
	}

	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	if requestID == "" {
		requestID, _ = c.Request.Context().Value(ctxkey.ClientRequestID).(string)
	}

	model, _ := c.Request.Context().Value(ctxkey.Model).(string)
	if model == "" {
		model = extractAdapterModel(body)
	}

	headers := make(map[string]string, len(c.Request.Header))
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	return service.AdapterEnforcementInput{
		RequestID:          requestID,
		UserID:             userID,
		APIKeyID:           apiKeyID,
		GroupID:            groupID,
		GroupPlatform:      groupPlatform,
		Method:             c.Request.Method,
		Path:               c.Request.URL.Path,
		Capability:         inferAdapterCapability(c.Request.Method, c.Request.URL.Path),
		Model:              model,
		Headers:            headers,
		Payload:            body,
		RequestPayloadHash: service.HashUsageRequestPayload(body),
		APIKey:             apiKey,
		User:               adapterAPIKeyUser(apiKey),
	}
}

func adapterAPIKeyUser(apiKey *service.APIKey) *service.User {
	if apiKey == nil {
		return nil
	}
	if apiKey.User != nil {
		return apiKey.User
	}
	if apiKey.UserID > 0 {
		return &service.User{ID: apiKey.UserID}
	}
	return nil
}

func inferAdapterCapability(method, path string) string {
	if strings.ToUpper(strings.TrimSpace(method)) == http.MethodPost &&
		(strings.EqualFold(path, "/v1/images/generations") || strings.EqualFold(path, "/images/generations") || strings.EqualFold(path, "/v1/images/edits") || strings.EqualFold(path, "/images/edits")) {
		return "image_generation"
	}
	if strings.Contains(strings.ToLower(path), "embeddings") {
		return "embeddings"
	}
	if strings.Contains(strings.ToLower(path), "chat/completions") || strings.Contains(strings.ToLower(path), "messages") || strings.Contains(strings.ToLower(path), "responses") {
		return "chat"
	}
	return ""
}

func extractAdapterModel(body []byte) string {
	if len(body) == 0 || !json.Valid(body) {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	model, _ := payload["model"].(string)
	return strings.TrimSpace(model)
}

func writeAdapterResponse(c *gin.Context, status int, body []byte) {
	if status <= 0 {
		status = http.StatusOK
	}
	if len(body) == 0 {
		c.Status(status)
		c.Abort()
		return
	}
	contentType := "application/json; charset=utf-8"
	if !json.Valid(body) {
		contentType = "text/plain; charset=utf-8"
	}
	c.Data(status, contentType, body)
	c.Abort()
}

func writeAdapterStream(c *gin.Context, status int, contentType string, body io.ReadCloser) {
	if status <= 0 {
		status = http.StatusOK
	}
	if strings.TrimSpace(contentType) == "" {
		contentType = "text/event-stream"
	}
	if body == nil {
		c.Status(status)
		c.Abort()
		return
	}
	defer body.Close()

	header := c.Writer.Header()
	header.Set("Content-Type", contentType)
	if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		header.Set("Cache-Control", "no-cache")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")
	}
	c.Writer.WriteHeader(status)

	buf := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			if _, writeErr := c.Writer.Write(buf[:n]); writeErr != nil {
				break
			}
			c.Writer.Flush()
		}
		if readErr != nil {
			break
		}
	}
	c.Abort()
}

func writeAdapterWebSocket(c *gin.Context, tunnel adapterclient.WSTunnel) {
	if tunnel == nil {
		writeAdapterError(c, http.StatusBadGateway, "adapter websocket tunnel unavailable")
		return
	}
	defer tunnel.Close()

	clientConn, err := coderws.Accept(c.Writer, c.Request, &coderws.AcceptOptions{
		CompressionMode: coderws.CompressionContextTakeover,
	})
	if err != nil {
		return
	}
	defer clientConn.CloseNow()
	clientConn.SetReadLimit(16 * 1024 * 1024)

	relayCtx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	errCh := make(chan error, 2)
	var closeOnce sync.Once
	closeBoth := func() {
		closeOnce.Do(func() {
			cancel()
			_ = tunnel.Close()
			_ = clientConn.Close(coderws.StatusNormalClosure, "")
			_ = clientConn.CloseNow()
		})
	}
	go relayAdapterWebSocketFrames(relayCtx, clientConn, tunnel, errCh)
	go relayAdapterWebSocketTunnel(relayCtx, tunnel, clientConn, errCh)
	<-errCh
	closeBoth()
	c.Abort()
}

func relayAdapterWebSocketFrames(ctx context.Context, clientConn *coderws.Conn, tunnel adapterclient.WSTunnel, errCh chan<- error) {
	for {
		msgType, payload, err := clientConn.Read(ctx)
		if err != nil {
			errCh <- err
			return
		}
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err = tunnel.Write(writeCtx, msgType, payload)
		cancel()
		if err != nil {
			errCh <- err
			return
		}
	}
}

func relayAdapterWebSocketTunnel(ctx context.Context, tunnel adapterclient.WSTunnel, clientConn *coderws.Conn, errCh chan<- error) {
	for {
		msgType, payload, err := tunnel.Read(ctx)
		if err != nil {
			errCh <- err
			return
		}
		writeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		err = clientConn.Write(writeCtx, msgType, payload)
		cancel()
		if err != nil {
			errCh <- err
			return
		}
	}
}

func writeAdapterError(c *gin.Context, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = "adapter request failed"
	}
	c.JSON(status, gin.H{"error": gin.H{"message": message}})
	c.Abort()
}
