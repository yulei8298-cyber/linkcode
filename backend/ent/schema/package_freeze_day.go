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

// PackageFreezeDay 可冻结日期：官方节假日（source=auto，定时同步）与后台手动添加（source=manual）。
//
// kind=off 为放假日，可冻结；kind=work 为调休补班日，仅用于日历标注（周末本身固定可冻结）。
type PackageFreezeDay struct {
	ent.Schema
}

func (PackageFreezeDay) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "package_freeze_days"},
	}
}

func (PackageFreezeDay) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (PackageFreezeDay) Fields() []ent.Field {
	return []ent.Field{
		field.Time("day").
			SchemaType(map[string]string{dialect.Postgres: "date"}),
		field.String("name").
			MaxLen(50).
			Default(""),
		field.String("kind").
			MaxLen(8),
		field.String("source").
			MaxLen(8),
	}
}

func (PackageFreezeDay) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("day", "source").Unique(),
	}
}
