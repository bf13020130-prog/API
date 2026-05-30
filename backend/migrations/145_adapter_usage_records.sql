-- Adapter usage analytics facts.
-- This table intentionally stays separate from native account-backed
-- usage_logs because adapter calls do not have a native upstream account_id.

CREATE TABLE IF NOT EXISTS adapter_usage_records (
    id                    BIGSERIAL PRIMARY KEY,
    request_id            VARCHAR(64) NOT NULL CHECK (btrim(request_id) <> ''),
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id            BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    group_id              BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    adapter_provider_id   BIGINT NOT NULL REFERENCES adapter_providers(id) ON DELETE RESTRICT,
    route_policy_id       BIGINT REFERENCES route_policies(id) ON DELETE SET NULL,
    provider              VARCHAR(64) NOT NULL CHECK (btrim(provider) <> ''),
    capability            VARCHAR(64) NOT NULL CHECK (btrim(capability) <> ''),
    model                 VARCHAR(100),
    method                VARCHAR(16),
    path                  VARCHAR(255),
    status                VARCHAR(32) NOT NULL CHECK (btrim(status) <> ''),
    status_code           INTEGER,
    duration_ms           INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
    error_message         TEXT,
    input_units           INTEGER NOT NULL DEFAULT 0 CHECK (input_units >= 0),
    output_units          INTEGER NOT NULL DEFAULT 0 CHECK (output_units >= 0),
    billable_units        INTEGER NOT NULL DEFAULT 0 CHECK (billable_units >= 0),
    cost_usd              DECIMAL(20,10) NOT NULL DEFAULT 0 CHECK (cost_usd >= 0),
    billable_unit         INTEGER NOT NULL DEFAULT 0 CHECK (billable_unit >= 0),
    billing_applied       BOOLEAN NOT NULL DEFAULT FALSE,
    billing_fingerprint   VARCHAR(160),
    metadata              JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(metadata) = 'object'),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS adapterusagerecords_request_id
    ON adapter_usage_records (request_id);

CREATE INDEX IF NOT EXISTS adapterusagerecords_user_created_at
    ON adapter_usage_records (user_id, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_api_key_created_at
    ON adapter_usage_records (api_key_id, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_group_created_at
    ON adapter_usage_records (group_id, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_adapter_provider_created_at
    ON adapter_usage_records (adapter_provider_id, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_provider_created_at
    ON adapter_usage_records (provider, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_status_created_at
    ON adapter_usage_records (status, created_at);

CREATE INDEX IF NOT EXISTS adapterusagerecords_model_created_at
    ON adapter_usage_records (model, created_at);
