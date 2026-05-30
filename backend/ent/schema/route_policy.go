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

// RoutePolicy stores explicit capability routing rules owned by sub2api.
type RoutePolicy struct {
	ent.Schema
}

func (RoutePolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "route_policies"},
	}
}

func (RoutePolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (RoutePolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty(),
		field.String("status").MaxLen(20).Default("disabled"),
		field.String("match_method").MaxLen(16).Optional().Nillable(),
		field.String("match_path").MaxLen(255).Optional().Nillable(),
		field.String("match_model").MaxLen(100).Optional().Nillable(),
		field.String("match_capability").MaxLen(64).Optional().Nillable(),
		field.String("match_group_platform").MaxLen(50).Optional().Nillable(),
		field.String("target").MaxLen(32).NotEmpty(),
		field.String("platform").MaxLen(50).Optional().Nillable(),
		field.Int64("adapter_provider_id").Optional().Nillable(),
		field.Int("priority").Default(50),
		field.JSON("conditions", map[string]any{}).
			Optional().
			SchemaType(map[string]string{dialect.Postgres: "jsonb"}),
		field.String("description").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "text"}),
	}
}

func (RoutePolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("adapter_provider", AdapterProvider.Type).
			Ref("route_policies").
			Field("adapter_provider_id").
			Unique(),
		edge.To("adapter_usage_records", AdapterUsageRecord.Type),
	}
}

func (RoutePolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status"),
		index.Fields("target"),
		index.Fields("priority"),
		index.Fields("adapter_provider_id"),
		index.Fields("match_group_platform", "status"),
		index.Fields("match_path", "match_method"),
		index.Fields("deleted_at"),
	}
}
