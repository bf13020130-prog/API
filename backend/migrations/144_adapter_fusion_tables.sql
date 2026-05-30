-- sub2api + new-api fusion persistence boundary.
-- Core provider families stay native in sub2api. These tables are only for
-- explicit long-tail adapter providers and their audit trail.

CREATE TABLE IF NOT EXISTS adapter_providers (
    id             BIGSERIAL PRIMARY KEY,
    name           VARCHAR(100) NOT NULL CHECK (btrim(name) <> ''),
    slug           VARCHAR(64) NOT NULL CHECK (btrim(slug) <> ''),
    status         VARCHAR(20) NOT NULL DEFAULT 'disabled' CHECK (status IN ('active', 'disabled')),
    adapter_type   VARCHAR(32) NOT NULL DEFAULT 'new-api' CHECK (adapter_type IN ('new-api')),
    base_url       VARCHAR(512) NOT NULL CHECK (base_url ~* '^https?://[^[:space:]]+$'),
    auth_mode      VARCHAR(32),
    credentials    JSONB NOT NULL DEFAULT '{}'::jsonb,
    capabilities   JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(capabilities) = 'array'),
    priority       INTEGER NOT NULL DEFAULT 50,
    timeout_ms     INTEGER NOT NULL DEFAULT 30000 CHECK (timeout_ms >= 0),
    extra          JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(extra) = 'object'),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT adapter_providers_slug_long_tail_check CHECK (
        lower(btrim(slug)) NOT IN (
            'openai',
            'anthropic',
            'claude',
            'gemini',
            'codex',
            'chatgpt',
            'antigravity',
            'anthropic-claude',
            'openai-compatible'
        )
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS adapterproviders_slug_uq
    ON adapter_providers (lower(btrim(slug)))
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS adapterproviders_status
    ON adapter_providers (status);

CREATE INDEX IF NOT EXISTS adapterproviders_adapter_type
    ON adapter_providers (adapter_type);

CREATE INDEX IF NOT EXISTS adapterproviders_deleted_at
    ON adapter_providers (deleted_at);

CREATE INDEX IF NOT EXISTS adapterproviders_priority
    ON adapter_providers (priority);

CREATE TABLE IF NOT EXISTS route_policies (
    id                    BIGSERIAL PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL CHECK (btrim(name) <> ''),
    status                VARCHAR(20) NOT NULL DEFAULT 'disabled' CHECK (status IN ('active', 'disabled')),
    match_method          VARCHAR(16),
    match_path            VARCHAR(255),
    match_model           VARCHAR(100),
    match_capability      VARCHAR(64),
    match_group_platform  VARCHAR(50),
    target                VARCHAR(32) NOT NULL CHECK (target IN ('sub2api_native', 'sub2api_upstream', 'new_api_adapter', 'unsupported')),
    platform              VARCHAR(50),
    adapter_provider_id   BIGINT REFERENCES adapter_providers(id) ON DELETE RESTRICT,
    priority              INTEGER NOT NULL DEFAULT 50,
    conditions            JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(conditions) = 'object'),
    description           TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ,
    CONSTRAINT route_policies_adapter_target_check CHECK (
        target <> 'new_api_adapter' OR adapter_provider_id IS NOT NULL
    )
);

CREATE INDEX IF NOT EXISTS routepolicies_status
    ON route_policies (status);

CREATE INDEX IF NOT EXISTS routepolicies_target
    ON route_policies (target);

CREATE INDEX IF NOT EXISTS routepolicies_priority
    ON route_policies (priority);

CREATE INDEX IF NOT EXISTS routepolicies_adapter_provider_id
    ON route_policies (adapter_provider_id);

CREATE INDEX IF NOT EXISTS routepolicies_group_platform_status
    ON route_policies (match_group_platform, status);

CREATE INDEX IF NOT EXISTS routepolicies_path_method
    ON route_policies (match_path, match_method);

CREATE INDEX IF NOT EXISTS routepolicies_deleted_at
    ON route_policies (deleted_at);

CREATE TABLE IF NOT EXISTS adapter_requests (
    id                    BIGSERIAL PRIMARY KEY,
    request_id            VARCHAR(64) NOT NULL CHECK (btrim(request_id) <> ''),
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id            BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    group_id              BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    adapter_provider_id   BIGINT NOT NULL REFERENCES adapter_providers(id) ON DELETE RESTRICT,
    provider              VARCHAR(64) NOT NULL CHECK (btrim(provider) <> ''),
    capability            VARCHAR(64) NOT NULL CHECK (btrim(capability) <> ''),
    route_target          VARCHAR(32) NOT NULL DEFAULT 'new_api_adapter' CHECK (route_target = 'new_api_adapter'),
    method                VARCHAR(16) NOT NULL CHECK (btrim(method) <> ''),
    path                  VARCHAR(255) NOT NULL CHECK (btrim(path) <> ''),
    model                 VARCHAR(100),
    status_code           INTEGER,
    duration_ms           INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    error_message         TEXT,
    metadata              JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS adapterrequests_request_id
    ON adapter_requests (request_id);

CREATE INDEX IF NOT EXISTS adapterrequests_user_created_at
    ON adapter_requests (user_id, created_at);

CREATE INDEX IF NOT EXISTS adapterrequests_api_key_created_at
    ON adapter_requests (api_key_id, created_at);

CREATE INDEX IF NOT EXISTS adapterrequests_group_created_at
    ON adapter_requests (group_id, created_at);

CREATE INDEX IF NOT EXISTS adapterrequests_adapter_provider_created_at
    ON adapter_requests (adapter_provider_id, created_at);

CREATE INDEX IF NOT EXISTS adapterrequests_provider_created_at
    ON adapter_requests (provider, created_at);

CREATE INDEX IF NOT EXISTS adapterrequests_route_target
    ON adapter_requests (route_target);
