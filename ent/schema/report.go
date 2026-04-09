package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// Report holds the schema definition for inspection reports.
type Report struct {
	ent.Schema
}

// Annotations of the Report.
func (Report) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "reports"},
	}
}

// Mixin of the Report.
func (Report) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

// Fields of the Report.
func (Report) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			Unique(),
		field.Enum("trigger").
			Values("manual", "webhook", "scheduled"),
		field.String("trigger_id"),
		field.Enum("status").
			Values("pending", "running", "completed", "failed").
			Default("pending"),
		field.Enum("severity").
			Values("info", "warning", "critical").
			Default("info"),
		field.String("summary").
			Default(""),
		field.Text("detail").
			Default(""),
		field.Strings("datasources"),
		field.Int("iterations").
			Default(0),
	}
}

// Indexes of the Report.
func (Report) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("trigger"),
		index.Fields("status"),
		index.Fields("severity"),
	}
}
