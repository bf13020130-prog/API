package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/service/capabilityrouter"
	"github.com/gin-gonic/gin"
)

func TestCapabilityRouteObserverStoresDecisionAndContinues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		group := &service.Group{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
		c.Set(string(ContextKeyAPIKey), &service.APIKey{ID: 100, Group: group})
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Next()
	})
	router.Use(CapabilityRouteObserver(capabilityrouter.New(capabilityrouter.Config{
		NewAPIAdapterProviders: []string{"midjourney"},
	})))
	router.POST("/v1/messages", func(c *gin.Context) {
		decision, ok := GetCapabilityRouteDecisionFromContext(c)
		if !ok {
			t.Fatal("capability route decision missing from gin context")
		}
		if decision.Target != capabilityrouter.TargetSub2APINative {
			t.Fatalf("Target = %q, want %q", decision.Target, capabilityrouter.TargetSub2APINative)
		}
		if decision.Platform != service.PlatformOpenAI {
			t.Fatalf("Platform = %q, want %q", decision.Platform, service.PlatformOpenAI)
		}

		ctxDecision, ok := c.Request.Context().Value(ctxkey.CapabilityRouteDecision).(capabilityrouter.Decision)
		if !ok {
			t.Fatal("capability route decision missing from request context")
		}
		if ctxDecision != decision {
			t.Fatalf("context decision = %+v, gin decision = %+v", ctxDecision, decision)
		}
		c.Status(http.StatusAccepted)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}
}

func TestCapabilityRouteObserverAddsAccessLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sink := initMiddlewareTestLogger(t)

	router := gin.New()
	router.Use(Logger())
	router.Use(func(c *gin.Context) {
		group := &service.Group{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
		c.Set(string(ContextKeyAPIKey), &service.APIKey{ID: 100, Group: group})
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Next()
	})
	router.Use(CapabilityRouteObserver(capabilityrouter.New(capabilityrouter.Config{})))
	router.POST("/v1/messages", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	for _, event := range sink.list() {
		if event == nil || event.Message != "http request completed" {
			continue
		}
		if event.Fields["capability_route_target"] != string(capabilityrouter.TargetSub2APINative) {
			t.Fatalf("capability_route_target = %v", event.Fields["capability_route_target"])
		}
		if event.Fields["capability_route_platform"] != service.PlatformOpenAI {
			t.Fatalf("capability_route_platform = %v", event.Fields["capability_route_platform"])
		}
		if event.Fields["capability_route_observe_only"] != true {
			t.Fatalf("capability_route_observe_only = %v", event.Fields["capability_route_observe_only"])
		}
		return
	}
	t.Fatal("access log event not found")
}

func TestCapabilityRouteObserverWorksWithoutAPIKeyContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(CapabilityRouteObserver(capabilityrouter.New(capabilityrouter.Config{})))
	router.GET("/unknown", func(c *gin.Context) {
		decision, ok := GetCapabilityRouteDecisionFromContext(c)
		if !ok {
			t.Fatal("capability route decision missing")
		}
		if decision.Target != capabilityrouter.TargetUnsupported {
			t.Fatalf("Target = %q, want %q", decision.Target, capabilityrouter.TargetUnsupported)
		}
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestCapabilityRouteObserverNoopsNilRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := &gin.Context{}
	CapabilityRouteObserver(capabilityrouter.New(capabilityrouter.Config{}))(c)
}
