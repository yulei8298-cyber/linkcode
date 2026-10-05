package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// UserPackageFreeze 套餐冻结记录：一次冻结一行，解冻时补全结束时间与原因。
type UserPackageFreeze struct {
	ent.Schema
}

func (UserPackageFreeze) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_package_freezes"},
	}
}

func (UserPackageFreeze) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserPackageFreeze) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("package_id"),
		field.Int64("user_id"),
		field.Time("frozen_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("unfrozen_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// unfreeze_reason: manual | cap | admin | disabled，进行中为空。
		field.String("unfreeze_reason").
			MaxLen(16).
			Default(""),
		field.Int64("duration_seconds").
			Default(0),
	}
}

func (UserPackageFreeze) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("package_id", "frozen_at"),
	}
}
