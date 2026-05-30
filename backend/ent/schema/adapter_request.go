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

// AdapterRequest records adapter-bound calls for audit and reconciliation.
type AdapterRequest struct {
	ent.Schema
}

func (AdapterRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "adapter_requests"},
	}
}

func (AdapterRequest) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(64).NotEmpty(),
		field.Int64("user_id"),
		field.Int64("api_key_id"),
		field.Int64("group_id").Optional().Nillable(),
		field.Int64("adapter_provider_id"),
		field.String("provider").MaxLen(64).NotEmpty(),
		field.String("capability").MaxLen(64).NotEmpty(),
		field.String("route_target").MaxLen(32).Default("new_api_adapter"),
		field.String("method").MaxLen(16).NotEmpty(),
		field.String("path").MaxLen(255).NotEmpty(),
		field.String("model").MaxLen(100).Optional().Nillable(),
		field.Int("status_code").Optional().Nillable(),
		field.Int("duration_ms").Optional().Nillable(),
		field.String("error_message").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
		field.JSON("metadata", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
	}
}

func (AdapterRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("adapter_requests").
			Field("user_id").
			Required().
			Unique(),
		edge.From("api_key", APIKey.Type).
			Ref("adapter_requests").
			Field("api_key_id").
			Required().
			Unique(),
		edge.From("group", Group.Type).
			Ref("adapter_requests").
			Field("group_id").
			Unique(),
		edge.From("adapter_provider", AdapterProvider.Type).
			Ref("adapter_requests").
			Field("adapter_provider_id").
			Required().
			Unique(),
	}
}

func (AdapterRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("request_id"),
		index.Fields("user_id", "created_at"),
		index.Fields("api_key_id", "created_at"),
		index.Fields("group_id", "created_at"),
		index.Fields("adapter_provider_id", "created_at"),
		index.Fields("provider", "created_at"),
		index.Fields("route_target"),
	}
}
