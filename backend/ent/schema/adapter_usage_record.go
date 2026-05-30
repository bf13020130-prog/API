package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdapterUsageRecord stores adapter usage facts for analytics without
// polluting native account-backed usage_logs.
type AdapterUsageRecord struct {
	ent.Schema
}

func (AdapterUsageRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "adapter_usage_records"},
	}
}

func (AdapterUsageRecord) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(64).NotEmpty(),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("group_id").Optional().Nillable(),
		field.Int64("adapter_provider_id"),
		field.Int64("route_policy_id").Optional().Nillable(),
		field.String("provider").MaxLen(64).NotEmpty(),
		field.String("capability").MaxLen(64).NotEmpty(),
		field.String("model").MaxLen(100).Optional().Nillable(),
		field.String("method").MaxLen(16).Optional(),
		field.String("path").MaxLen(255).Optional(),
		field.String("status").MaxLen(32).NotEmpty(),
		field.Int("status_code").Optional().Nillable(),
		field.Int("duration_ms").Optional().Nillable(),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.Int("input_units").Default(0),
		field.Int("output_units").Default(0),
		field.Int("billable_units").Default(0),
		field.Float("cost_usd").
			Default(0).
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,10)"}),
		field.Int("billable_unit").Default(0),
		field.Bool("billing_applied").Default(false),
		field.String("billing_fingerprint").MaxLen(160).Optional().Nillable(),
		field.JSON("metadata", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AdapterUsageRecord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("adapter_usage_records").
			Field("user_id").
			Required().
			Unique(),
		edge.From("api_key", APIKey.Type).
			Ref("adapter_usage_records").
			Field("api_key_id").
			Required().
			Unique(),
		edge.From("group", Group.Type).
			Ref("adapter_usage_records").
			Field("group_id").
			Unique(),
		edge.From("adapter_provider", AdapterProvider.Type).
			Ref("adapter_usage_records").
			Field("adapter_provider_id").
			Required().
			Unique(),
		edge.From("route_policy", RoutePolicy.Type).
			Ref("adapter_usage_records").
			Field("route_policy_id").
			Unique(),
	}
}

func (AdapterUsageRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id"),
		index.Fields("user_id", "created_at"),
		index.Fields("api_key_id", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("adapter_provider_id", "created_at"),
		index.Fields("provider", "created_at"),
		index.Fields("status", "created_at"),
		index.Fields("model", "created_at"),
	}
}
