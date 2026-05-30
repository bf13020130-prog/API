# Fusion Phase Status

## Completed

### Phase 1: Capability Matrix And Route Policy

Files:

- `docs/fusion/capability-matrix.md`
- `docs/fusion/route-policy.md`
- `docs/fusion/execution-roadmap.md`

Result:

- Core providers are fixed as `sub2api` native.
- `new-api` is fixed as a long-tail adapter boundary only.
- Unknown providers resolve to `unsupported`.

### Phase 2: Minimum Capability Router

Files:

- `backend/internal/service/capabilityrouter/router.go`
- `backend/internal/service/capabilityrouter/router_test.go`

Result:

- Native/core requests resolve to `sub2api_native`.
- OpenAI-compatible upstream accounts resolve to `sub2api_upstream`.
- Long-tail providers require explicit adapter enablement.
- Core provider adapter slugs are ignored even if misconfigured.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/capabilityrouter -count=1
```

### Phase 3: Data Ownership Plan

Files:

- `docs/fusion/data-ownership-plan.md`

Result:

- Durable users, API keys, groups, accounts, quota, usage, and billing stay in
  `sub2api`.
- Proposed future tables are `adapter_providers`, `route_policies`,
  `adapter_requests`, and optional `model_catalog_entries`.
- Backfill from `new-api` rejects core provider channels and imports only
  long-tail providers.

### Phase 4: Adapter Boundary Skeleton

Files:

- `backend/internal/service/adapterclient/client.go`
- `backend/internal/service/adapterclient/client_test.go`
- `backend/internal/service/adapterclient/provider_config.go`
- `backend/internal/service/adapterclient/provider_config_test.go`
- `docs/fusion/observe-only-integration-plan.md`

Result:

- Adapter calls require `sub2api` ownership context:
  `request_id`, `user_id`, `api_key_id`, `group_id`, `provider`, `capability`,
  and `route_target=new_api_adapter`.
- A fake adapter client is available for future service tests.
- Adapter provider config validation rejects core provider slugs and invalid
  endpoints before a real adapter can be wired.
- Observe-only gateway integration plan is documented.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/capabilityrouter ./internal/service/adapterclient -count=1
```

### Phase 5: Observe-Only Gateway Integration

Files:

- `backend/internal/pkg/ctxkey/ctxkey.go`
- `backend/internal/server/middleware/middleware.go`
- `backend/internal/server/middleware/capability_route_observer.go`
- `backend/internal/server/middleware/capability_route_observer_test.go`
- `backend/internal/server/middleware/logger.go`
- `backend/internal/server/routes/gateway.go`

Result:

- Gateway routes now compute a capability routing decision after API key auth
  and group assignment checks.
- Decisions are stored in both `gin.Context` and `request.Context`.
- Access logs include `capability_route_target`, `capability_route_platform`,
  `capability_route_reason`, and `capability_route_observe_only`.
- Handler selection is unchanged; this phase is observe-only and enforcement is
  still disabled.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/capabilityrouter ./internal/service/adapterclient -count=1
```

Additional middleware test command to run once dependencies are available:

```powershell
$env:GOTELEMETRY='off'
go test -tags unit ./internal/server/middleware -run TestCapabilityRouteObserver -count=1
```

Current blocker:

- Middleware/routes/schema package tests need Go dependencies that are not in the
  local module cache. Dependency download currently fails through
  `127.0.0.1:9`.

### Phase 6: Disabled Adapter Provider Config Boundary

Files:

- `backend/internal/config/config.go`
- `backend/internal/service/adapterclient/provider_config.go`
- `backend/internal/service/adapterclient/provider_config_test.go`
- `backend/internal/server/routes/gateway_capability_router.go`
- `backend/internal/server/routes/gateway_capability_router_test.go`

Result:

- `gateway.adapter_providers` can describe long-tail adapter providers.
- Only valid `active` long-tail providers are exposed to `CapabilityRouter`.
- Core provider slugs remain rejected and cannot make OpenAI/Claude/Gemini/
  Codex/Antigravity flow through `new-api`.
- Default config keeps adapter providers empty, so runtime behavior stays
  observe-only.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/capabilityrouter ./internal/service/adapterclient -count=1
```

Package-level route test is present but currently blocked by dependency
download:

```powershell
go test ./internal/server/routes -run TestNewGatewayCapabilityRouter -count=1
```

### Phase 7: Adapter Persistence Schema Draft

Files:

- `backend/ent/schema/adapter_provider.go`
- `backend/ent/schema/route_policy.go`
- `backend/ent/schema/adapter_request.go`
- `backend/ent/schema/user.go`
- `backend/ent/schema/api_key.go`
- `backend/ent/schema/group.go`

Result:

- Added Ent schema drafts for long-tail adapter providers, explicit route
  policies, and adapter request audit records.
- Existing user/API key/group schemas now expose `adapter_requests` reverse
  edges.
- No generated Ent code or SQL migration has been produced yet because the
  local dependency cache is incomplete.

Validation to run once dependencies are available:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./ent/schema -run TestNonExistent -count=1
```

### Phase 8: Read-Only Adapter Provider Diagnostics

Files:

- `backend/internal/service/adapterclient/provider_config.go`
- `backend/internal/service/adapterclient/provider_config_test.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_providers_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/admin.go`

Result:

- Added a safe admin diagnostic endpoint:
  `GET /api/v1/admin/system/adapter-providers/diagnostics`.
- The endpoint reports configured adapter providers, normalized status,
  validity, enablement, validation reason, active slugs, and the current
  observe-only/enforcement-disabled state.
- Provider credentials are intentionally omitted from diagnostics.
- The pure diagnostics builder is covered in `adapterclient` so the core
  behavior is testable even while handler package dependencies are unavailable.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/capabilityrouter ./internal/service/adapterclient -count=1
```

Handler and route level commands are present but currently blocked by missing
local Go module cache and proxy `127.0.0.1:9`:

```powershell
go test ./internal/handler/admin -run TestSystemHandlerAdapterProviderDiagnostics -count=1
go test ./internal/server/routes -run TestNewGatewayCapabilityRouter -count=1
```

### Phase 9: Frontend Adapter Provider Diagnostics

Files:

- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/router/index.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added a read-only admin page at `/admin/channels/adapters`.
- The page calls
  `GET /api/v1/admin/system/adapter-providers/diagnostics` through the
  existing admin system API module.
- The channel management sidebar now exposes an "adapter diagnostics" entry.
- The UI shows observe-only/enforcement state, active adapter slugs, provider
  validity, enablement, priority, timeout, capabilities, and validation reason.
- No adapter credentials are rendered, and no create/update/delete controls are
  exposed in this phase.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm install --frozen-lockfile --prefer-offline
pnpm run typecheck
pnpm run build
```

Notes:

- The first dependency install attempt timed out before top-level links were
  complete. A follow-up frozen install with `--prefer-offline` completed by
  reusing the local store and downloading the missing tarballs.
- `pnpm run build` completed successfully. Vite emitted existing chunking and
  dynamic/static import warnings, but no build errors.

### Phase 10: Adapter Persistence Migration And Registry Boundary

Files:

- `backend/migrations/144_adapter_fusion_tables.sql`
- `backend/internal/service/adapterclient/provider_registry.go`
- `backend/internal/service/adapterclient/provider_registry_test.go`
- `backend/internal/service/adapterregistry/config_registry.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/gateway_capability_router.go`
- `backend/cmd/server/wire_gen.go`

Result:

- Added a forward-only SQL migration for:
  - `adapter_providers`
  - `route_policies`
  - `adapter_requests`
- The migration keeps core provider families out of `adapter_providers` with a
  database-level slug check.
- `adapter_type` is intentionally constrained to `new-api` for this phase.
- Adapter route policies require an `adapter_provider_id` when the target is
  `new_api_adapter`.
- Adapter request records stay owned by sub2api user/API key/group/provider
  identifiers and are audit-only.
- Extracted `adapterclient.ProviderRegistry` so diagnostics and router setup no
  longer hand-roll provider filtering in multiple places.
- Current runtime still uses `gateway.adapter_providers` via
  `adapterregistry.NewConfigProviderRegistry`; this is the seam to replace with
  a DB-backed registry after Ent generation is available.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/adapterclient ./internal/service/capabilityrouter -count=1
```

Blocked validation:

```powershell
go test ./ent/schema -run TestNonExistent -count=1
go test ./migrations -run TestNonExistent -count=1
go test ./internal/handler/admin -run TestSystemHandlerAdapterProviderDiagnostics -count=1
go test ./internal/server/routes -run TestNewGatewayCapabilityRouter -count=1
```

These commands are still blocked by missing Go module cache / network access to
`proxy.golang.org` from this machine. Inside the sandbox the proxy is
`127.0.0.1:9`; outside the sandbox, `proxy.golang.org` also timed out over IPv6
when fetching `entgo.io/ent@v0.14.5`.

### Phase 11: HTTP Adapter Client Skeleton

Files:

- `backend/internal/service/adapterclient/client.go`
- `backend/internal/service/adapterclient/client_test.go`
- `backend/internal/service/adapterclient/http_client.go`
- `backend/internal/service/adapterclient/http_client_test.go`

Result:

- Added a standard-library HTTP adapter client behind the existing
  `adapterclient.Client` interface.
- The client only selects valid, active, configured long-tail providers from a
  `ProviderRegistry`.
- Adapter calls post to `/internal/adapter/execute` on the configured
  provider `base_url`.
- Requests carry the sub2api ownership envelope:
  `request_id`, `user_id`, `api_key_id`, `group_id`, `provider`,
  `capability`, `model`, `route_target`, billing category, original method/path,
  headers, and JSON payload.
- Bearer/header credential modes are supported for the internal provider call.
- Non-2xx adapter responses are mapped to `adapterclient.Response` with
  `StatusFailed`; they are not allowed to mutate native provider scheduling
  state here.
- This client is not wired into live gateway enforcement yet, so runtime
  behavior remains observe-only.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
go test ./internal/service/adapterclient ./internal/service/capabilityrouter -count=1
```

### Phase 12: DB-Backed Adapter Provider CRUD

Files:

- `backend/internal/service/adapter_provider_service.go`
- `backend/internal/repository/adapter_provider_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Ent code and Wire output have been generated.
- Adapter providers are now DB-backed instead of config-only.
- Admin API supports safe list/get/create/update/delete for adapter providers.
- Credential values are never returned to the frontend; only
  `has_credentials` and `credential_keys` are exposed.
- Updating a provider preserves existing credentials when `credentials` is
  omitted.
- The gateway capability observer reads active adapter slugs through the
  DB-backed provider registry.
- Frontend `/admin/channels/adapters` now exposes provider CRUD controls.
- Core provider slugs are still rejected before they can become adapter
  providers.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go generate ./ent
go generate ./cmd/server
go test ./internal/service -run TestAdapterProviderService -count=1
go test ./internal/handler/admin -run 'TestSystemHandler.*AdapterProvider' -count=1
go test ./internal/server/routes -run 'TestNewGatewayCapabilityRouter|TestGatewayRoutesOpenAI' -count=1
go test ./internal/repository -run TestNonExistent -count=1
go test ./internal/server -run TestNonExistent -count=1
go test ./cmd/server -run TestNonExistent -count=1
```

Frontend:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run typecheck
pnpm run build
```

### Phase 13: DB-Backed Route Policy Admin Boundary

Files:

- `backend/internal/service/route_policy_service.go`
- `backend/internal/service/route_policy_service_test.go`
- `backend/internal/repository/route_policy_repo.go`
- `backend/internal/repository/wire.go`
- `backend/internal/service/wire.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_route_policies_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/admin.go`
- `backend/cmd/server/wire_gen.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added route policy repository/service/admin CRUD.
- New API endpoints:
  - `GET /api/v1/admin/system/route-policies`
  - `GET /api/v1/admin/system/route-policies/:id`
  - `POST /api/v1/admin/system/route-policies`
  - `PUT /api/v1/admin/system/route-policies/:id`
  - `DELETE /api/v1/admin/system/route-policies/:id`
- Route policies default to `disabled`.
- `target=new_api_adapter` requires a valid adapter provider ID.
- Adapter-targeted policies reject core group platforms such as OpenAI,
  Anthropic/Claude, Gemini, and Antigravity.
- Non-adapter targets clear `adapter_provider_id` in service validation.
- Frontend adapter console now includes route policy list/create/update/delete.
- This phase still does not enable live adapter execution; gateway routing
  remains observe-only.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service -run TestRoutePolicyService -count=1
go test ./internal/handler/admin -run TestSystemHandler.*RoutePolicy -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

Frontend:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run typecheck
pnpm run build
```

### Phase 14: Disabled-By-Default Live Adapter Enforcement

Files:

- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/server/middleware/adapter_enforcement.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`
- `backend/internal/server/routes/gateway.go`
- `backend/internal/server/router.go`
- `backend/internal/server/http.go`
- `backend/internal/config/config.go`
- `backend/internal/repository/wire.go`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire_gen.go`

Result:

- Added a live adapter enforcement service behind
  `gateway.adapter_enforcement.enabled`, which defaults to `false`.
- Active `route_policies` with `target=new_api_adapter` can now short-circuit
  a gateway request to the HTTP adapter client when enforcement is explicitly
  enabled.
- Core group platforms remain protected: OpenAI, Anthropic/Claude, Gemini, and
  Antigravity never route through the adapter enforcement path.
- Adapter execution carries the sub2api ownership envelope:
  `request_id`, `user_id`, `api_key_id`, `group_id`, provider slug, capability,
  model, method, path, headers, and original payload.
- Adapter attempts are recorded through `adapter_requests`, including provider,
  target, status code, duration, error message, and policy metadata.
- Gateway middleware safely reads and restores request bodies when enforcement
  does not handle the request, preserving native handler behavior.
- Non-stream JSON adapter responses can be returned directly to clients; native
  providers remain the default path unless enforcement and policy both opt in.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go generate ./cmd/server
go test ./internal/service -run TestAdapterEnforcement -count=1
go test ./internal/server/middleware -run TestAdapterEnforcement -count=1
go test ./internal/server/routes -run 'TestGatewayRoutes|TestNewGatewayCapabilityRouter' -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

### Phase 15: Adapter Usage Billing Reconciliation Hook

Files:

- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/server/middleware/adapter_enforcement.go`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire_gen.go`

Result:

- Successful adapter responses with `usage.cost_usd > 0` now feed the existing
  `UsageBillingRepository` idempotent billing path.
- Adapter billing uses `request_id + api_key_id + request_fingerprint` for the
  same once-only semantics as native provider billing.
- Standard balance users are billed through `BalanceCost`; subscription groups
  are billed through `SubscriptionCost`.
- API key quota and API key rate windows are updated when the API key has those
  limits configured.
- Failed adapter responses and zero-cost adapter responses do not trigger
  billing.
- Request payload hashes are passed from gateway middleware into adapter billing
  to reduce accidental silent deduplication when a client reuses request IDs.
- `adapter_requests.metadata` records billing cost, unit counts, skipped reason,
  applied state, and fingerprint for audit/reconciliation.
- `usage_logs` are intentionally not written for adapter calls yet because that
  table requires a native `account_id`; this avoids polluting native account
  statistics with synthetic adapter accounts.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go generate ./cmd/server
go test ./internal/service -run TestAdapterEnforcement -count=1
go test ./internal/server/middleware -run TestAdapterEnforcement -count=1
```

### Phase 16: Adapter Request Observability

Files:

- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_requests_test.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/handler/wire.go`
- `backend/cmd/server/wire_gen.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added a safe admin list endpoint for adapter execution audit records:
  `GET /api/v1/admin/system/adapter-requests`.
- The endpoint returns `adapter_requests` without credentials or raw request
  bodies and supports focused filters for `provider`, `request_id`, `status`,
  and `limit`.
- Status filters map `success/succeeded` to records without `error_message`
  and `failed/error` to records with `error_message`.
- List limits default to 100 and cap at 500.
- Frontend `/admin/channels/adapters` now includes an adapter request audit
  table with provider/status/request filters, status badge, cost, billing
  state, duration, timestamp, and error summary.
- Billing reconciliation metadata from Phase 15 is visible in the audit table
  so adapter billing can be inspected without writing synthetic native
  `usage_logs`.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go generate ./cmd/server
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

Frontend:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run typecheck
pnpm run build
```

### Phase 17: Adapter Usage Analytics Model

Files:

- `backend/ent/schema/adapter_usage_record.go`
- `backend/ent/schema/adapter_provider.go`
- `backend/ent/schema/route_policy.go`
- `backend/ent/schema/user.go`
- `backend/ent/schema/api_key.go`
- `backend/ent/schema/group.go`
- `backend/migrations/145_adapter_usage_records.sql`
- `backend/internal/service/adapter_usage_service.go`
- `backend/internal/service/adapter_usage_service_test.go`
- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/repository/adapter_usage_repo.go`
- `backend/internal/repository/wire.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_usages_test.go`
- `backend/internal/handler/wire.go`
- `backend/internal/server/routes/admin.go`
- `backend/internal/service/wire.go`
- `backend/cmd/server/wire_gen.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added dedicated `adapter_usage_records` analytics facts instead of extending
  native `usage_logs` with synthetic account IDs.
- Adapter analytics are keyed by sub2api ownership identifiers:
  `request_id`, `user_id`, `api_key_id`, optional `group_id`,
  `adapter_provider_id`, optional `route_policy_id`, provider, capability, and
  model.
- Adapter usage captures response status, status code, duration, error message,
  input/output/billable units, `cost_usd`, billing applied state, and billing
  fingerprint.
- Live adapter enforcement now writes both `adapter_requests` audit records and
  `adapter_usage_records` analytics records after an adapter attempt.
- Admin API exposes:
  - `GET /api/v1/admin/system/adapter-usages`
  - `GET /api/v1/admin/system/adapter-usages/summary`
- Frontend system API includes adapter usage list/summary types and calls.
- `/admin/channels/adapters` now shows adapter usage summary cards for total
  adapter requests, success rate, cost, and billable units.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go generate ./ent
go generate ./cmd/server
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

Frontend:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run typecheck
pnpm run build
```

### Phase 18: Adapter SSE Streaming Passthrough

Files:

- `backend/internal/service/adapterclient/client.go`
- `backend/internal/service/adapterclient/http_client.go`
- `backend/internal/service/adapterclient/http_client_test.go`
- `backend/internal/server/middleware/adapter_enforcement.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`

Result:

- HTTP adapter client now detects successful `text/event-stream` adapter
  responses and returns the upstream body as an `io.ReadCloser` instead of
  buffering it into memory.
- Non-stream adapter responses continue through the existing JSON envelope
  decoding path.
- Non-2xx adapter responses continue through the existing error mapping path
  rather than being treated as streams.
- Live adapter enforcement middleware now writes streamed adapter responses
  back to the client with the adapter status code and content type.
- SSE responses set `Cache-Control: no-cache`, `Connection: keep-alive`, and
  `X-Accel-Buffering: no` before copying chunks to the response writer and
  flushing as data arrives.
- Streaming adapter calls still record audit and usage analytics immediately
  with the usage fields available at adapter response start. Full final usage
  trailer reconciliation is intentionally left for a later phase.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service/adapterclient -run TestHTTPClientReturnsSSEStreamWithoutBuffering -count=1
go test ./internal/server/middleware -run TestAdapterEnforcementMiddlewareStreamsAdapterResponse -count=1
go test ./internal/service/adapterclient ./internal/server/middleware -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

### Phase 19: Adapter Streaming Usage Trailer Reconciliation

Files:

- `backend/internal/service/adapterclient/client.go`
- `backend/internal/service/adapterclient/http_client.go`
- `backend/internal/service/adapterclient/http_client_test.go`
- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/service/adapter_request_service.go`
- `backend/internal/service/adapter_usage_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/repository/adapter_usage_repo.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`

Result:

- Streaming adapter responses now preserve HTTP trailers through
  `adapterclient.Response.Trailers`.
- The supported final usage trailer is `X-Sub2API-Adapter-Usage`, with JSON
  fields matching adapter usage: `input_units`, `output_units`,
  `billable_unit`, and `cost_usd`.
- Live adapter enforcement wraps streamed response bodies with a close finalizer.
  When the SSE body is closed after copy, the finalizer parses the usage trailer.
- Final streamed usage now runs through the existing idempotent
  `UsageBillingRepository` billing path instead of staying as a zero-cost
  started stream.
- The same `adapter_requests` and `adapter_usage_records` rows are updated with
  final billing metadata and final usage values, so a single stream remains one
  request in analytics.
- Audit/usage metadata marks finalized stream usage with
  `stream_usage_finalized=true` and `usage_source=trailer`.
- Adapter request/usage repositories now expose narrow update methods for
  status, duration, error, billing metadata, and usage fields; ownership fields
  remain create-time data.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service/adapterclient -run TestHTTPClientPreservesSSEUsageTrailer -count=1
go test ./internal/service -run TestAdapterEnforcementFinalizesStreamingUsageFromTrailer -count=1
go test ./internal/server/middleware -run TestAdapterEnforcementMiddlewareFinalizesStreamingUsageTrailer -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

### Phase 20: NYCATAI-Style User Dashboard Shell

Files:

- `frontend/src/views/user/DashboardView.vue`
- `frontend/src/views/user/__tests__/DashboardView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Refreshed `/dashboard` into a user-facing console closer to the observed
  NYCATAI structure: account credits, system announcements, quick entry cards,
  recent consumption, charts, and existing detailed stats.
- The dashboard now uses real existing data sources: current user balance,
  usage dashboard stats, recent usage records, platform quotas, and the existing
  announcement store.
- No hardcoded third-party announcement content was copied; announcement cards
  render site-owned announcements and fall back to a neutral empty state.
- The existing detailed sub2api stats, platform quota cards, trend charts, usage
  table, and quick actions remain available below the new console shell.
- Added a focused component test that shallow-renders the dashboard and verifies
  the account, announcement, and shortcut sections.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/user/__tests__/DashboardView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 21: OpenAI-Compatible SSE Usage Final Chunk

Files:

- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`

Result:

- Streaming adapter enforcement now observes `text/event-stream` data frames
  while still passing the original bytes through unchanged to the client.
- Final usage is resolved with HTTP trailer priority first, then
  OpenAI-compatible SSE final chunks that expose a JSON `usage` object.
- The SSE mapper accepts both sub2api unit fields
  `input_units/output_units/billable_unit/cost_usd` and OpenAI-style
  `prompt_tokens/completion_tokens/total_tokens/cost_usd`.
- Streams with token usage but no `cost_usd` update adapter usage analytics
  without applying billing.
- Finalized SSE usage reuses the same Phase 19 close finalizer, so the same
  `adapter_requests` and `adapter_usage_records` rows are updated rather than
  creating duplicate request facts.
- Audit/usage metadata marks finalized SSE usage with
  `stream_usage_finalized=true` and `usage_source=sse_final_chunk`.
- The observer only runs for `text/event-stream` responses and keeps a bounded
  per-event buffer to avoid unbounded stream buffering.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service -run 'TestAdapterEnforcement(FinalizesStreamingUsage|RecordsFreeOpenAISSE)' -count=1
go test ./internal/server/middleware -run 'TestAdapterEnforcementMiddlewareFinalizesStreamingUsage(Trailer|FromOpenAISSEFinalChunk)$' -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

### Phase 22: Adapter WebSocket Passthrough Tunnel

Files:

- `backend/internal/service/adapterclient/client.go`
- `backend/internal/service/adapterclient/http_client.go`
- `backend/internal/service/adapterclient/http_client_test.go`
- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/server/middleware/adapter_enforcement.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`

Result:

- `adapterclient.HTTPClient` can now open a WebSocket tunnel to long-tail
  adapters at `/internal/adapter/ws`.
- Adapter WebSocket handshakes carry the sub2api ownership and routing context
  in `X-Sub2API-*` headers, including request ID, user/API key/group IDs,
  provider, capability, method, path, model, and route target.
- Provider auth is applied to WebSocket handshakes with the same bearer/header
  modes used by HTTP adapter calls.
- Live adapter enforcement now has a dedicated `EnforceWebSocket` path. It
  preserves the same core-platform protection: OpenAI, Anthropic/Claude,
  Gemini, Codex, Antigravity, and other core slugs do not route through
  adapters.
- Middleware detects `Upgrade: websocket` requests before reading a body. When
  a long-tail route policy handles the request, sub2api accepts the client
  WebSocket and relays frames bidirectionally to the adapter tunnel.
- Adapter WebSocket attempts create the same adapter audit and analytics facts,
  marked with `websocket=true` and `transport=websocket`.
- This phase implements a generic tunnel only; provider-specific WebSocket
  final usage extraction is intentionally left for a later concrete protocol
  sample.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service/adapterclient -run 'TestHTTPClient(OpensAdapterWebSocketTunnel|ReturnsSSEStream|PreservesSSE)' -count=1
go test ./internal/service -run 'TestAdapterEnforcement(OpensWebSocketTunnel|WebSocketNeverRoutesCore|FinalizesStreamingUsage)' -count=1
go test ./internal/server/middleware -run 'TestAdapterEnforcementMiddleware(ProxiesAdapterWebSocket|StreamsAdapterResponse|FinalizesStreamingUsage)' -count=1
go test ./internal/service/adapterclient ./internal/service/capabilityrouter ./internal/service ./internal/handler/admin ./internal/server/middleware ./internal/server/routes ./internal/repository ./cmd/server -count=1
```

### Phase 23: NYCATAI-Style Adapter Operator Cockpit

Files:

- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added a compact adapter operator cockpit to the top of
  `/admin/channels/adapters`, keeping the existing provider CRUD, route policy
  editor, and adapter request audit table below it.
- The cockpit derives all values from existing live admin data sources rather
  than hardcoded counters: provider diagnostics, route policies, adapter request
  audit records, and adapter usage summary.
- It surfaces the daily operator signals that matter for the sub2api + long-tail
  adapter split: healthy routable providers, active policies, recent failures,
  finalized streaming usage, WebSocket tunnel traffic, and total adapter cost.
- The status badge flips between healthy and attention-needed based on invalid
  provider diagnostics or recent failed adapter requests.
- The design remains a dense admin console surface instead of a marketing shell:
  one operator summary band above the existing operational tables, with no new
  route or duplicated CRUD flow.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 24: Adapter Operator Drill-Down Filters

Files:

- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Turned the request-oriented cockpit metrics into lightweight drill-down
  controls. Operators can click recent failures, finalized stream usage, or
  WebSocket traffic to focus the adapter request audit table immediately.
- The drill-down is intentionally local to the already-loaded recent audit
  window, so no new backend query contract or route was introduced.
- Clicking the same cockpit metric toggles the request audit back to the full
  recent request set.
- Added accessible titles and pressed state for the interactive metrics while
  keeping non-request metrics visually consistent but disabled.
- Expanded the adapter provider view test to prove the failure metric focuses
  the request audit down to the failed request row.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 25: Server-Side Adapter Audit Focus Filters

Files:

- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_requests_test.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`

Result:

- Extended `GET /api/v1/admin/system/adapter-requests` with a narrow
  `focus` query parameter for operator drill-downs:
  `failed`, `stream`, and `websocket`.
- `focus=stream` filters audit records where
  `metadata.stream_usage_finalized=true`.
- `focus=websocket` filters audit records where either
  `metadata.websocket=true` or `metadata.transport="websocket"`.
- `focus=failed` reuses the same failure predicate as request status filtering.
- Tightened `status=failed` semantics so failed audit queries include records
  with `status_code >= 400` even when no error message was stored.
- The frontend cockpit now sends the selected `focus` value to
  `listAdapterRequests`, so drill-downs can query beyond the current local
  audit window while preserving the immediate local table focus behavior.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests' -count=1
go test ./internal/repository ./internal/service ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests|TestAdapterEnforcement' -count=1

cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 26: Adapter Audit Time Window Filters

Files:

- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_requests_test.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Extended adapter request audit listing with `created_from` and `created_to`
  RFC3339 query parameters.
- Invalid time-window values return `400` instead of silently querying the wrong
  range.
- Repository filtering now applies `created_at >= created_from` and
  `created_at <= created_to` alongside provider, request ID, status, focus, and
  limit filters.
- The admin adapter request audit defaults to a near-term 24-hour window, with
  quick switches for last 7 days, last 30 days, and all time.
- Cockpit drill-downs keep using the selected time window when they request
  server-side focused audit records.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests' -count=1

cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 27: Adapter Audit Offset Pagination

Files:

- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_requests_test.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Extended `GET /api/v1/admin/system/adapter-requests` with an `offset` query
  parameter while preserving the existing descending `created_at, id` ordering.
- `offset` is normalized in the service layer, so negative values fall back to
  the first page and the existing `limit` clamp still protects large queries.
- The admin adapter request audit now requests the first page with
  `limit=100&offset=0`.
- A compact "load more" control appends the next audit page with the same
  provider, status, request ID, focus, and time-window filters, which makes
  incident drill-downs usable beyond the first 100 records.
- The UI shows the number of loaded audit records and hides the load-more
  control once a short page is returned.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests' -count=1
go test ./internal/repository ./internal/service ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests|TestAdapterEnforcement' -count=1

cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
pnpm run typecheck
pnpm run build
```

### Phase 28: Provider-Specific Adapter Stream Usage Mapping

Files:

- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`

Result:

- Extended adapter SSE final-usage extraction beyond OpenAI-compatible
  `usage` chunks.
- Gemini/Vertex-style adapter stream events now map `usageMetadata` into
  adapter usage records:
  - `promptTokenCount - cachedContentTokenCount` becomes input units.
  - `candidatesTokenCount + thoughtsTokenCount` becomes output units.
  - `totalTokenCount` is honored when present; otherwise the billable unit
    falls back to input plus output units.
- Gemini `usageMetadata` nested under a `response` wrapper is supported, which
  matches the shape used by Gemini CLI/adapter-style streams.
- Claude/Anthropic-style adapter stream events now merge usage across
  `message_start` and `message_delta` events, so input-side usage from
  `message.usage` is not lost when the terminal delta only carries
  `output_tokens`.
- Cost-bearing Claude-style final usage continues through the existing
  sub2api-owned billing path; Gemini token-only usage is recorded for audit and
  analytics but remains `not_billable` unless the adapter supplies cost.
- Core providers are still protected by route ownership rules. These mappings
  only interpret long-tail adapter response bodies after an adapter route has
  already been selected.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service -run 'TestAdapterEnforcementFinalizesStreamingUsageFrom(GeminiUsageMetadata|ClaudeMessageEvents)' -count=1
go test ./internal/service -run 'TestAdapterEnforcement(FinalizesStreamingUsage|RecordsFreeOpenAISSE|BillsSuccessfulAdapterUsage|SkipsBilling)' -count=1
go test ./internal/server/middleware -run 'TestAdapterEnforcementMiddlewareFinalizesStreamingUsage' -count=1
go test ./internal/service/adapterclient -run 'TestHTTPClient(ReturnsSSEStream|PreservesSSE)' -count=1
```

### Phase 29: Adapter WebSocket Final Usage Events

Files:

- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/server/middleware/adapter_enforcement_test.go`

Result:

- Adapter WebSocket tunnels now observe adapter-to-client text frames without
  changing the generic relay contract.
- OpenAI-style WebSocket terminal events such as
  `{"type":"response.completed","response":{"usage":...}}` are mapped into the
  same sub2api-owned adapter audit and usage records used by HTTP/SSE adapter
  calls.
- The pending WebSocket audit/usage rows are finalized on tunnel close with:
  `websocket_usage_finalized=true`, `usage_source=websocket_event`,
  `transport=websocket`, updated token/cost fields, and billing result
  metadata.
- Cost-bearing WebSocket final usage runs through the existing
  `UsageBillingRepository` path, so API key quota/rate-limit/subscription
  billing ownership stays in sub2api.
- The middleware remains a frame relay. Business parsing lives in
  `AdapterEnforcementService` via a finalizing `WSTunnel` wrapper, so future
  provider-specific event mappings can be added without teaching the HTTP
  middleware about vendor protocols.
- Core provider protections are unchanged: core platforms still do not route to
  adapters before any WebSocket usage parsing can happen.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service -run 'TestAdapterEnforcementFinalizesWebSocketUsageFromResponseCompletedEvent' -count=1
go test ./internal/server/middleware -run 'TestAdapterEnforcementMiddlewareFinalizesWebSocketUsageEvent' -count=1
go test ./internal/service -run 'TestAdapterEnforcement(.*WebSocket|FinalizesStreamingUsage|RecordsFreeOpenAISSE)' -count=1
go test ./internal/server/middleware -run 'TestAdapterEnforcementMiddleware(.*WebSocket|FinalizesStreamingUsage|StreamsAdapterResponse)' -count=1
```

### Phase 30: Adapter Audit Usage Signal Visibility

Files:

- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- The adapter request audit table now exposes a compact signal column for
  reconciliation markers already written by the backend.
- Requests finalized from SSE usage chunks show a stream settlement marker and
  the recorded `usage_source`, such as `sse_final_chunk`.
- WebSocket requests finalized from terminal usage events show a WebSocket
  settlement marker and `usage_source=websocket_event`.
- WebSocket tunnels without final usage still show a separate WebSocket tunnel
  marker, so operators can distinguish transport traffic from reconciled
  billable usage.
- The operator cockpit counts remain unchanged; this phase makes the same facts
  visible at row level for incident review.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
```

### Phase 31: Adapter Audit Filtered Count

Files:

- `backend/internal/service/adapter_request_service.go`
- `backend/internal/repository/adapter_request_repo.go`
- `backend/internal/handler/admin/system_handler.go`
- `backend/internal/handler/admin/system_handler_adapter_requests_test.go`
- `backend/internal/server/routes/admin.go`
- `frontend/src/api/admin/system.ts`
- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added `GET /api/v1/admin/system/adapter-requests/count` for filtered adapter
  audit totals without changing the existing list response shape.
- Count uses the same provider, request ID, status, focus, and created-time
  filters as `GET /adapter-requests`, but ignores `limit` and `offset`.
- Repository filtering is shared between list and count paths, reducing the
  chance that operator totals drift from visible request rows.
- The adapter operator audit footer now shows loaded rows and filtered total,
  for example `已加载 100 / 共 101 条`.
- "Load more" still appends the next offset page without re-counting, while
  refreshes, focus changes, and time-window changes fetch a fresh total.

Validation:

```powershell
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/handler/admin -run 'TestSystemHandlerListAdapterRequests|TestSystemHandlerCountAdapterRequests' -count=1

cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
```

### Phase 32: Adapter Audit Direct Page Navigation

Files:

- `frontend/src/views/admin/AdapterProvidersView.vue`
- `frontend/src/views/admin/__tests__/AdapterProvidersView.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- The adapter request audit footer now shows current page and total pages when
  a filtered total spans multiple pages.
- Operators can move backward, forward, or type a target page number and jump
  directly to that offset page.
- Direct page navigation replaces the current audit rows, while the existing
  "load more" path still appends the next page for continuous incident review.
- Refreshes, metric focus changes, and time-window/filter changes reset the
  audit table back to page 1 with a fresh filtered total.
- Page navigation reuses the Phase 27 `offset` API and Phase 31 filtered total,
  so no new backend contract was required.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/views/admin/__tests__/AdapterProvidersView.spec.ts
```

### Phase 33: Standalone Adapter Operations Route

Files:

- `frontend/src/router/index.ts`
- `frontend/src/router/__tests__/adapter-operations-route.spec.ts`
- `frontend/src/components/layout/AppSidebar.vue`
- `frontend/src/components/layout/__tests__/AppSidebar.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

Result:

- Added `/admin/adapters` as a standalone admin-protected entry point for the
  adapter operator cockpit.
- The new route reuses the mature `AdapterProvidersView` so provider health,
  policy state, request audit, stream/WebSocket settlement markers, filtered
  totals, and page navigation stay on one operational surface.
- The channel-scoped `/admin/channels/adapters` route remains available for
  backward compatibility and for users who still discover the page through
  channel management.
- The admin sidebar now exposes a top-level "Adapter Operations" entry while
  keeping the existing channel-scoped "Adapter Diagnostics" child link.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/router/__tests__/adapter-operations-route.spec.ts
pnpm run test:run -- src/components/layout/__tests__/AppSidebar.spec.ts
```

### Phase 34: Adapter Operations Simple Mode Guard

Files:

- `frontend/src/router/index.ts`
- `frontend/src/router/__tests__/guards.spec.ts`

Result:

- Simple mode now redirects direct `/admin/adapters` visits back to
  `/admin/dashboard`, matching the sidebar behavior that hides the standalone
  adapter operations entry in simple mode.
- Simple mode also redirects `/admin/channels/*`, including the compatibility
  `/admin/channels/adapters` route, because the channel management group is
  hidden from the simple-mode admin sidebar.
- Full admin mode still has both the standalone `/admin/adapters` operator
  route and the channel-scoped compatibility route.
- This keeps adapter and channel operations from becoming deep-link escape
  hatches for simple-mode admin layouts.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\frontend
pnpm run test:run -- src/router/__tests__/guards.spec.ts
pnpm run test:run -- src/router/__tests__/adapter-operations-route.spec.ts
pnpm run test:run -- src/components/layout/__tests__/AppSidebar.spec.ts
```

### Phase 35: Controlled Adapter Diagnostic Sampling

Files:

- `backend/internal/service/adapter_diagnostics.go`
- `backend/internal/service/adapter_enforcement_service.go`
- `backend/internal/service/adapter_enforcement_service_test.go`
- `backend/internal/service/wire.go`
- `backend/internal/service/wire_test.go`
- `backend/internal/config/config.go`
- `deploy/config.example.yaml`

Result:

- Added adapter diagnostics under
  `gateway.adapter_enforcement.diagnostic_sampling`.
- Failed adapter calls/responses now always record a bounded redacted
  diagnostic bundle in audit metadata so operators can trace root cause.
- The `diagnostic_sampling.enabled` switch only controls extra proactive
  sampling for successful/ongoing traffic. That extra sampling activates when
  explicitly matched by provider, request ID, or `sample_all`; an enabled block
  with no selectors records no extra successful traffic.
- Audit metadata now can include redacted `diagnostic` data for the adapter
  request summary, safe header values, bounded JSON shape, and failure root
  cause (`adapter_call_error`, `adapter_response_error`, or `billing_error`).
- SSE and WebSocket finalizers can attach bounded terminal event samples with
  event type, JSON key paths, usage detection state, and redacted samples, so
  real provider usage formats can be mapped later without logging full prompts,
  completions, tokens, cookies, or API keys.
- Configuration wiring passes the diagnostic sampling settings into
  `AdapterEnforcementService` so runtime config can enable targeted captures.

Validation:

```powershell
cd E:\work\code\API\_study\sub2api\backend
$env:GOCACHE='E:\work\code\API\_study\sub2api\.gocache'
$env:GOPATH='E:\work\code\API\_study\sub2api\.gopath'
$env:GOTELEMETRY='off'
$env:HTTP_PROXY=''
$env:HTTPS_PROXY=''
$env:ALL_PROXY=''
$env:GOPROXY='https://goproxy.cn,direct'
go test ./internal/service -run 'TestAdapterEnforcement|TestProvideAdapterEnforcementServiceWiresDiagnosticSamplingConfig' -count=1
```

## Not Done Yet

- Streaming final usage supports the sub2api-defined HTTP trailer protocol,
  OpenAI-compatible SSE final `usage` chunks, Gemini/Vertex `usageMetadata`,
  and Claude/Anthropic `message_start` + `message_delta` usage. Other
  provider-specific nonstandard stream summary formats are not mapped yet.
- Adapter WebSocket final usage supports OpenAI-style
  `response.completed.response.usage` events. Gemini/Claude-style WebSocket
  terminal usage events still need real provider samples before mapping.
- Adapter operator drill-down now supports server-side metric focus, time
  windows, offset-based "load more" pagination, filtered total counts, and
  direct page navigation, and is available from `/admin/adapters` as well as
  the compatibility `/admin/channels/adapters` route. The standalone route is
  hidden and guarded in simple mode; channel-management deep links are guarded
  there as well.
- Failed adapter traffic now has redacted root-cause diagnostics by default.
  Extra successful/ongoing traffic sampling is available but still needs real
  provider traffic to collect concrete Gemini/Claude/WebSocket terminal usage
  samples before adding provider-specific mappings beyond the current generic
  JSON key-path capture.
- No deployment or git commit has been performed in this workspace.

## Recommended Next Steps

1. Deploy and inspect failed adapter audit metadata first; temporarily enable
   extra diagnostic sampling for one provider or request ID only when successful
   or incomplete stream/WebSocket terminal samples are needed.
2. Map the next concrete provider-specific stream/WebSocket usage summary
   format after capturing a real sample.
3. Decide whether `/admin/channels/adapters` should eventually redirect to
   `/admin/adapters` after operators have migrated their bookmarks.
4. Commit, push, and deploy from a real git checkout when this staged workspace
   is promoted into the deployable project.
