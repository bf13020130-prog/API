package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sysutil"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// SystemHandler handles system-related operations
type SystemHandler struct {
	updateSvc          *service.UpdateService
	lockSvc            *service.SystemOperationLockService
	adapterProviderSvc *service.AdapterProviderService
	routePolicySvc     *service.RoutePolicyService
	adapterRequestSvc  *service.AdapterRequestService
	adapterUsageSvc    *service.AdapterUsageService
}

// NewSystemHandler creates a new SystemHandler
func NewSystemHandler(updateSvc *service.UpdateService, lockSvc *service.SystemOperationLockService, adapterProviderSvc *service.AdapterProviderService, routePolicySvc *service.RoutePolicyService, optionalServices ...any) *SystemHandler {
	var requestSvc *service.AdapterRequestService
	var usageSvc *service.AdapterUsageService
	for _, optional := range optionalServices {
		switch svc := optional.(type) {
		case *service.AdapterRequestService:
			requestSvc = svc
		case *service.AdapterUsageService:
			usageSvc = svc
		}
	}
	return &SystemHandler{
		updateSvc:          updateSvc,
		lockSvc:            lockSvc,
		adapterProviderSvc: adapterProviderSvc,
		routePolicySvc:     routePolicySvc,
		adapterRequestSvc:  requestSvc,
		adapterUsageSvc:    usageSvc,
	}
}

// GetVersion returns the current version
// GET /api/v1/admin/system/version
func (h *SystemHandler) GetVersion(c *gin.Context) {
	info, _ := h.updateSvc.CheckUpdate(c.Request.Context(), false)
	response.Success(c, gin.H{
		"version": info.CurrentVersion,
	})
}

// CheckUpdates checks for available updates
// GET /api/v1/admin/system/check-updates
func (h *SystemHandler) CheckUpdates(c *gin.Context) {
	force := c.Query("force") == "true"
	info, err := h.updateSvc.CheckUpdate(c.Request.Context(), force)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, info)
}

// GetAdapterProviderDiagnostics returns safe read-only diagnostics for configured long-tail adapters.
// GET /api/v1/admin/system/adapter-providers/diagnostics
func (h *SystemHandler) GetAdapterProviderDiagnostics(c *gin.Context) {
	if h == nil || h.adapterProviderSvc == nil {
		response.Success(c, service.NewAdapterProviderService(nil).Diagnostics())
		return
	}

	diagnostics, err := h.adapterProviderSvc.ProviderDiagnostics(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, diagnostics)
}

type adapterProviderRequest struct {
	Name         string            `json:"name" binding:"required,max=100"`
	Slug         string            `json:"slug" binding:"required,max=64"`
	Status       string            `json:"status" binding:"omitempty,oneof=active disabled"`
	AdapterType  string            `json:"adapter_type" binding:"omitempty,max=32"`
	BaseURL      string            `json:"base_url" binding:"required,max=512"`
	AuthMode     string            `json:"auth_mode" binding:"omitempty,max=32"`
	Credentials  map[string]string `json:"credentials"`
	Capabilities []string          `json:"capabilities" binding:"required,min=1"`
	Priority     int               `json:"priority"`
	TimeoutMS    int               `json:"timeout_ms"`
	Extra        map[string]any    `json:"extra"`
}

type updateAdapterProviderRequest struct {
	Name         string             `json:"name" binding:"required,max=100"`
	Slug         string             `json:"slug" binding:"required,max=64"`
	Status       string             `json:"status" binding:"omitempty,oneof=active disabled"`
	AdapterType  string             `json:"adapter_type" binding:"omitempty,max=32"`
	BaseURL      string             `json:"base_url" binding:"required,max=512"`
	AuthMode     string             `json:"auth_mode" binding:"omitempty,max=32"`
	Credentials  *map[string]string `json:"credentials"`
	Capabilities []string           `json:"capabilities" binding:"required,min=1"`
	Priority     int                `json:"priority"`
	TimeoutMS    int                `json:"timeout_ms"`
	Extra        map[string]any     `json:"extra"`
}

// ListAdapterProviders returns DB-backed long-tail adapter providers without credential values.
// GET /api/v1/admin/system/adapter-providers
func (h *SystemHandler) ListAdapterProviders(c *gin.Context) {
	if h == nil || h.adapterProviderSvc == nil {
		response.Success(c, []service.AdapterProviderSafeView{})
		return
	}
	providers, err := h.adapterProviderSvc.ListSafe(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, providers)
}

// GetAdapterProvider returns one DB-backed adapter provider without credential values.
// GET /api/v1/admin/system/adapter-providers/:id
func (h *SystemHandler) GetAdapterProvider(c *gin.Context) {
	id, ok := parseAdapterProviderID(c)
	if !ok {
		return
	}
	provider, err := h.adapterProviderSvc.GetSafeByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, provider)
}

// CreateAdapterProvider creates a long-tail adapter provider.
// POST /api/v1/admin/system/adapter-providers
func (h *SystemHandler) CreateAdapterProvider(c *gin.Context) {
	var req adapterProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.adapterProviderSvc.Create(c.Request.Context(), &service.AdapterProvider{
		Name:         req.Name,
		Slug:         req.Slug,
		Status:       req.Status,
		AdapterType:  req.AdapterType,
		BaseURL:      req.BaseURL,
		AuthMode:     req.AuthMode,
		Credentials:  req.Credentials,
		Capabilities: req.Capabilities,
		Priority:     req.Priority,
		TimeoutMS:    req.TimeoutMS,
		Extra:        req.Extra,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created.SafeView())
}

// UpdateAdapterProvider updates a long-tail adapter provider.
// PUT /api/v1/admin/system/adapter-providers/:id
func (h *SystemHandler) UpdateAdapterProvider(c *gin.Context) {
	id, ok := parseAdapterProviderID(c)
	if !ok {
		return
	}
	var req updateAdapterProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.adapterProviderSvc.Update(c.Request.Context(), &service.AdapterProviderUpdate{
		ID:           id,
		Name:         req.Name,
		Slug:         req.Slug,
		Status:       req.Status,
		AdapterType:  req.AdapterType,
		BaseURL:      req.BaseURL,
		AuthMode:     req.AuthMode,
		Credentials:  req.Credentials,
		Capabilities: req.Capabilities,
		Priority:     req.Priority,
		TimeoutMS:    req.TimeoutMS,
		Extra:        req.Extra,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated.SafeView())
}

// DeleteAdapterProvider soft-deletes a long-tail adapter provider.
// DELETE /api/v1/admin/system/adapter-providers/:id
func (h *SystemHandler) DeleteAdapterProvider(c *gin.Context) {
	id, ok := parseAdapterProviderID(c)
	if !ok {
		return
	}
	if err := h.adapterProviderSvc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Adapter provider deleted successfully"})
}

func parseAdapterProviderID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid adapter provider ID")
		return 0, false
	}
	return id, true
}

type routePolicyRequest struct {
	Name               string         `json:"name" binding:"required,max=100"`
	Status             string         `json:"status" binding:"omitempty,oneof=active disabled"`
	MatchMethod        string         `json:"match_method" binding:"omitempty,max=16"`
	MatchPath          string         `json:"match_path" binding:"omitempty,max=255"`
	MatchModel         string         `json:"match_model" binding:"omitempty,max=100"`
	MatchCapability    string         `json:"match_capability" binding:"omitempty,max=64"`
	MatchGroupPlatform string         `json:"match_group_platform" binding:"omitempty,max=50"`
	Target             string         `json:"target" binding:"required,max=32"`
	Platform           string         `json:"platform" binding:"omitempty,max=50"`
	AdapterProviderID  *int64         `json:"adapter_provider_id"`
	Priority           int            `json:"priority"`
	Conditions         map[string]any `json:"conditions"`
	Description        string         `json:"description"`
}

// ListRoutePolicies returns DB-backed capability route policies.
// GET /api/v1/admin/system/route-policies
func (h *SystemHandler) ListRoutePolicies(c *gin.Context) {
	if h == nil || h.routePolicySvc == nil {
		response.Success(c, []service.RoutePolicySafeView{})
		return
	}
	policies, err := h.routePolicySvc.ListSafe(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policies)
}

// GetRoutePolicy returns one route policy.
// GET /api/v1/admin/system/route-policies/:id
func (h *SystemHandler) GetRoutePolicy(c *gin.Context) {
	id, ok := parseRoutePolicyID(c)
	if !ok {
		return
	}
	policy, err := h.routePolicySvc.GetSafeByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

// CreateRoutePolicy creates a route policy.
// POST /api/v1/admin/system/route-policies
func (h *SystemHandler) CreateRoutePolicy(c *gin.Context) {
	var req routePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	created, err := h.routePolicySvc.Create(c.Request.Context(), routePolicyFromRequest(0, req))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created.SafeView())
}

// UpdateRoutePolicy updates a route policy.
// PUT /api/v1/admin/system/route-policies/:id
func (h *SystemHandler) UpdateRoutePolicy(c *gin.Context) {
	id, ok := parseRoutePolicyID(c)
	if !ok {
		return
	}
	var req routePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.routePolicySvc.Update(c.Request.Context(), routePolicyFromRequest(id, req))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated.SafeView())
}

// DeleteRoutePolicy soft-deletes a route policy.
// DELETE /api/v1/admin/system/route-policies/:id
func (h *SystemHandler) DeleteRoutePolicy(c *gin.Context) {
	id, ok := parseRoutePolicyID(c)
	if !ok {
		return
	}
	if err := h.routePolicySvc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Route policy deleted successfully"})
}

func parseRoutePolicyID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid route policy ID")
		return 0, false
	}
	return id, true
}

func routePolicyFromRequest(id int64, req routePolicyRequest) *service.RoutePolicy {
	return &service.RoutePolicy{
		ID:                 id,
		Name:               req.Name,
		Status:             req.Status,
		MatchMethod:        req.MatchMethod,
		MatchPath:          req.MatchPath,
		MatchModel:         req.MatchModel,
		MatchCapability:    req.MatchCapability,
		MatchGroupPlatform: req.MatchGroupPlatform,
		Target:             req.Target,
		Platform:           req.Platform,
		AdapterProviderID:  req.AdapterProviderID,
		Priority:           req.Priority,
		Conditions:         req.Conditions,
		Description:        req.Description,
	}
}

// ListAdapterRequests returns adapter execution audit records.
// GET /api/v1/admin/system/adapter-requests
func (h *SystemHandler) ListAdapterRequests(c *gin.Context) {
	if h == nil || h.adapterRequestSvc == nil {
		response.Success(c, []service.AdapterRequestSafeView{})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	createdFrom, ok := parseOptionalRFC3339Query(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalRFC3339Query(c, "created_to")
	if !ok {
		return
	}
	records, err := h.adapterRequestSvc.List(c.Request.Context(), service.AdapterRequestListFilters{
		Provider:    strings.TrimSpace(c.Query("provider")),
		RequestID:   strings.TrimSpace(c.Query("request_id")),
		Status:      strings.TrimSpace(c.Query("status")),
		Focus:       strings.TrimSpace(c.Query("focus")),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
		Offset:      offset,
		Limit:       limit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, records)
}

// CountAdapterRequests returns adapter execution audit count for the same filters as ListAdapterRequests.
// GET /api/v1/admin/system/adapter-requests/count
func (h *SystemHandler) CountAdapterRequests(c *gin.Context) {
	if h == nil || h.adapterRequestSvc == nil {
		response.Success(c, gin.H{"total": 0})
		return
	}
	createdFrom, ok := parseOptionalRFC3339Query(c, "created_from")
	if !ok {
		return
	}
	createdTo, ok := parseOptionalRFC3339Query(c, "created_to")
	if !ok {
		return
	}
	total, err := h.adapterRequestSvc.Count(c.Request.Context(), service.AdapterRequestListFilters{
		Provider:    strings.TrimSpace(c.Query("provider")),
		RequestID:   strings.TrimSpace(c.Query("request_id")),
		Status:      strings.TrimSpace(c.Query("status")),
		Focus:       strings.TrimSpace(c.Query("focus")),
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"total": total})
}

func parseOptionalRFC3339Query(c *gin.Context, key string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, true
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		response.BadRequest(c, "Invalid "+key)
		return time.Time{}, false
	}
	return parsed, true
}

// ListAdapterUsages returns adapter usage analytics records.
// GET /api/v1/admin/system/adapter-usages
func (h *SystemHandler) ListAdapterUsages(c *gin.Context) {
	if h == nil || h.adapterUsageSvc == nil {
		response.Success(c, []service.AdapterUsageSafeView{})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	records, err := h.adapterUsageSvc.List(c.Request.Context(), service.AdapterUsageFilters{
		Provider:  strings.TrimSpace(c.Query("provider")),
		RequestID: strings.TrimSpace(c.Query("request_id")),
		Status:    strings.TrimSpace(c.Query("status")),
		Limit:     limit,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, records)
}

// GetAdapterUsageSummary returns adapter usage analytics summary.
// GET /api/v1/admin/system/adapter-usages/summary
func (h *SystemHandler) GetAdapterUsageSummary(c *gin.Context) {
	if h == nil || h.adapterUsageSvc == nil {
		response.Success(c, service.AdapterUsageSummary{})
		return
	}
	summary, err := h.adapterUsageSvc.Summary(c.Request.Context(), service.AdapterUsageFilters{
		Provider:  strings.TrimSpace(c.Query("provider")),
		RequestID: strings.TrimSpace(c.Query("request_id")),
		Status:    strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

// PerformUpdate downloads and applies the update
// POST /api/v1/admin/system/update
func (h *SystemHandler) PerformUpdate(c *gin.Context) {
	operationID := buildSystemOperationID(c, "update")
	payload := gin.H{"operation_id": operationID}
	executeAdminIdempotentJSON(c, "admin.system.update", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		var releaseReason string
		succeeded := false
		defer func() {
			release(releaseReason, succeeded)
		}()

		if err := h.updateSvc.PerformUpdate(ctx); err != nil {
			releaseReason = "SYSTEM_UPDATE_FAILED"
			return nil, err
		}
		succeeded = true

		return gin.H{
			"message":      "Update completed. Please restart the service.",
			"need_restart": true,
			"operation_id": lock.OperationID(),
		}, nil
	})
}

// Rollback restores the previous version
// POST /api/v1/admin/system/rollback
func (h *SystemHandler) Rollback(c *gin.Context) {
	operationID := buildSystemOperationID(c, "rollback")
	payload := gin.H{"operation_id": operationID}
	executeAdminIdempotentJSON(c, "admin.system.rollback", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		var releaseReason string
		succeeded := false
		defer func() {
			release(releaseReason, succeeded)
		}()

		if err := h.updateSvc.Rollback(); err != nil {
			releaseReason = "SYSTEM_ROLLBACK_FAILED"
			return nil, err
		}
		succeeded = true

		return gin.H{
			"message":      "Rollback completed. Please restart the service.",
			"need_restart": true,
			"operation_id": lock.OperationID(),
		}, nil
	})
}

// RestartService restarts the systemd service
// POST /api/v1/admin/system/restart
func (h *SystemHandler) RestartService(c *gin.Context) {
	operationID := buildSystemOperationID(c, "restart")
	payload := gin.H{"operation_id": operationID}
	executeAdminIdempotentJSON(c, "admin.system.restart", payload, service.DefaultSystemOperationIdempotencyTTL(), func(ctx context.Context) (any, error) {
		lock, release, err := h.acquireSystemLock(ctx, operationID)
		if err != nil {
			return nil, err
		}
		succeeded := false
		defer func() {
			release("", succeeded)
		}()

		// Schedule service restart in background after sending response
		// This ensures the client receives the success response before the service restarts
		go func() {
			// Wait a moment to ensure the response is sent
			time.Sleep(500 * time.Millisecond)
			sysutil.RestartServiceAsync()
		}()
		succeeded = true
		return gin.H{
			"message":      "Service restart initiated",
			"operation_id": lock.OperationID(),
		}, nil
	})
}

func (h *SystemHandler) acquireSystemLock(
	ctx context.Context,
	operationID string,
) (*service.SystemOperationLock, func(string, bool), error) {
	if h.lockSvc == nil {
		return nil, nil, service.ErrIdempotencyStoreUnavail
	}
	lock, err := h.lockSvc.Acquire(ctx, operationID)
	if err != nil {
		return nil, nil, err
	}
	release := func(reason string, succeeded bool) {
		releaseCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.lockSvc.Release(releaseCtx, lock, succeeded, reason)
	}
	return lock, release, nil
}

func buildSystemOperationID(c *gin.Context, operation string) string {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" {
		return "sysop-" + operation + "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	actorScope := "admin:0"
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok {
		actorScope = "admin:" + strconv.FormatInt(subject.UserID, 10)
	}
	seed := operation + "|" + actorScope + "|" + c.FullPath() + "|" + key
	hash := service.HashIdempotencyKey(seed)
	if len(hash) > 24 {
		hash = hash[:24]
	}
	return "sysop-" + hash
}
