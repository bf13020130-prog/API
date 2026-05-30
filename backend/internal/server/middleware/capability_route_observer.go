package middleware

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CapabilityRouteObserver stores the capability routing decision for logs and
// future enforcement. It is observe-only: it never changes handler selection.
func CapabilityRouteObserver(router capabilityrouter.Router) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c == nil || c.Request == nil {
			if c != nil {
				c.Next()
			}
			return
		}

		decision := router.Decide(buildCapabilityRouteInput(c))
		c.Set(string(ContextKeyCapabilityRouteDecision), decision)

		ctx := context.WithValue(c.Request.Context(), ctxkey.CapabilityRouteDecision, decision)
		requestLogger := logger.FromContext(ctx).With(
			zap.String("capability_route_target", string(decision.Target)),
			zap.String("capability_route_platform", decision.Platform),
			zap.String("capability_route_reason", decision.Reason),
			zap.Bool("capability_route_observe_only", true),
		)
		ctx = logger.IntoContext(ctx, requestLogger)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

func GetCapabilityRouteDecisionFromContext(c *gin.Context) (capabilityrouter.Decision, bool) {
	if c == nil {
		return capabilityrouter.Decision{}, false
	}
	value, exists := c.Get(string(ContextKeyCapabilityRouteDecision))
	if !exists {
		return capabilityrouter.Decision{}, false
	}
	decision, ok := value.(capabilityrouter.Decision)
	return decision, ok
}

func buildCapabilityRouteInput(c *gin.Context) capabilityrouter.Input {
	input := capabilityrouter.Input{
		Method: c.Request.Method,
		Path:   c.Request.URL.Path,
	}

	if model, ok := c.Request.Context().Value(ctxkey.Model).(string); ok {
		input.Model = model
	}
	if platform, ok := GetForcePlatformFromContext(c); ok {
		input.GroupPlatform = platform
		return input
	}
	if group, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group); ok && group != nil {
		input.GroupPlatform = group.Platform
		return input
	}
	if apiKey, ok := GetAPIKeyFromContext(c); ok && apiKey != nil && apiKey.Group != nil {
		input.GroupPlatform = apiKey.Group.Platform
	}

	return input
}
