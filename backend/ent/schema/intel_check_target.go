package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IntelCheckTarget holds the schema definition for the IntelCheckTarget entity.
// 智力检测受检分组：一个对外承诺「不降智」的分组，配置自己的上游地址、
// 凭据、模型与推理等级。base_url / api_key 仅管理端可见，公开接口永不返回。
type IntelCheckTarget struct {
	ent.Schema
}

func (IntelCheckTarget) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "intel_check_targets"},
	}
}

func (IntelCheckTarget) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (IntelCheckTarget) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			NotEmpty().
			MaxLen(100),
		field.String("description").
			Optional().
			Default("").
			MaxLen(500),
		field.String("base_url").
			NotEmpty().
			MaxLen(500).
			Comment("Upstream base origin, e.g. https://api.example.com; never exposed publicly"),
		field.String("api_key_encrypted").
			NotEmpty().
			Sensitive().
			Comment("AES-256-GCM encrypted API key"),
		// api_mode: responses | chat_completions，与 channel_monitors.api_mode 同义。
		field.String("api_mode").
			Default("responses").
			MaxLen(32).
			Comment("Upstream request protocol: responses or chat_completions"),
		field.String("model").
			NotEmpty().
			MaxLen(200),
		// reasoning_effort: 仅 responses 协议下随请求下发。
		field.String("reasoning_effort").
			Default("medium").
			MaxLen(32).
			Comment("Reasoning effort passed to upstream: low, medium, high or xhigh"),
		// rate_label: 纯展示用的倍率文案（如 ×0.3），不参与任何计费逻辑。
		field.String("rate_label").
			Optional().
			Default("").
			MaxLen(20).
			Comment("Display-only rate badge, e.g. x0.3"),
		field.Bool("enabled").
			Default(true),
		field.Int("sort_order").
			Default(0),
		field.Int64("created_by"),
	}
}

func (IntelCheckTarget) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("results", IntelCheckResult.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (IntelCheckTarget) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("enabled", "sort_order"),
	}
}
