package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AdapterProvider stores long-tail providers served through the new-api adapter boundary.
type AdapterProvider struct {
	ent.Schema
}

func (AdapterProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "adapter_providers"},
	}
}

func (AdapterProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (AdapterProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("slug").MaxLen(64).NotEmpty(),
		field.String("status").MaxLen(20).Default("disabled"),
		field.String("adapter_type").MaxLen(32).Default("new-api"),
		field.String("base_url").MaxLen(512).NotEmpty(),
		field.String("auth_mode").MaxLen(32).Optional().Nillable(),
		field.JSON("credentials", map[string]string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.JSON("capabilities", []string{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.Int("priority").Default(50),
		field.Int("timeout_ms").Default(30000),
		field.JSON("extra", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
	}
}

func (AdapterProvider) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("route_policies", RoutePolicy.Type),
		edge.To("adapter_requests", AdapterRequest.Type),
		edge.To("adapter_usage_records", AdapterUsageRecord.Type),
	}
}

func (AdapterProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("slug").Unique(),
		index.Fields("status"),
		index.Fields("adapter_type"),
		index.Fields("deleted_at"),
		index.Fields("priority"),
	}
}
