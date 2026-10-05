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

// PackagePlan 套餐配置：绑定一个普通（余额）分组，按「周期 × 额度档位」区分。
//
// 删除策略：硬删除。已购套餐在 user_packages 中快照了名称、档位与额度，
// 删除配置不影响已购；日常下架用 for_sale。
type PackagePlan struct {
	ent.Schema
}

func (PackagePlan) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "package_plans"},
	}
}

func (PackagePlan) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (PackagePlan) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("group_id"),
		field.String("name").
			NotEmpty().
			MaxLen(50),
		// cycle: week | month，决定有效期与前端样式。
		field.String("cycle").
			MaxLen(10),
		// tier: 1 | 2，2 档额度固定为同分组同周期 1 档的两倍。
		field.Int8("tier"),
		field.Float("price").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,2)"}),
		field.Float("quota_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Int("validity_days"),
		field.Bool("for_sale").
			Default(true),
	}
}

func (PackagePlan) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "cycle", "tier").Unique(),
	}
}
