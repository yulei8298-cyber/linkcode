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

// UserPackage 用户已购套餐：一次购买一行，同分组可叠加。
//
// 名称、档位、额度均为购买时快照。冻结期间 expires_at 不变，解冻时按冻结时长顺延；
// frozen_seconds_total 只累计已结束的冻结段。
type UserPackage struct {
	ent.Schema
}

func (UserPackage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_packages"},
	}
}

func (UserPackage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (UserPackage) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("user_id"),
		field.Int64("group_id"),
		field.Int64("plan_id"),
		field.Int64("order_id").
			Optional().
			Nillable(),
		field.String("name").
			NotEmpty().
			MaxLen(50),
		field.String("cycle").
			MaxLen(10),
		field.Int8("tier"),
		field.Float("quota_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}),
		field.Float("used_usd").
			SchemaType(map[string]string{dialect.Postgres: "decimal(20,8)"}).
			Default(0),
		field.Time("starts_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Time("expires_at").
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		// status: active | frozen | exhausted | expired | voided
		field.String("status").
			MaxLen(16).
			Default("active"),
		field.Time("frozen_at").
			Optional().
			Nillable().
			SchemaType(map[string]string{dialect.Postgres: "timestamptz"}),
		field.Int64("frozen_seconds_total").
			Default(0),
	}
}

func (UserPackage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("order_id").Unique().Annotations(entsql.IndexWhere("order_id IS NOT NULL")),
		index.Fields("user_id", "group_id", "status", "expires_at"),
		index.Fields("status", "expires_at"),
		index.Fields("frozen_at").Annotations(entsql.IndexWhere("status = 'frozen'")),
	}
}
