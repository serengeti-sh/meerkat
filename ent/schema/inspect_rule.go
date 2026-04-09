package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// InspectRule holds the schema definition for scheduled inspection rules.
type InspectRule struct {
	ent.Schema
}

// Annotations of the InspectRule.
func (InspectRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "inspect_rules"},
	}
}

// Mixin of the InspectRule.
func (InspectRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the InspectRule.
func (InspectRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Unique(),
		field.String("name").
			Unique().
			MaxLen(128),
		field.String("interval").
			MaxLen(32),
		field.String("metric_query").
			Default(""),
		field.String("log_query").
			Default(""),
		field.Bool("enabled").
			Default(true),
	}
}

// Indexes of the InspectRule.
func (InspectRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("enabled"),
	}
}
