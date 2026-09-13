package schema

import (
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IntelCheckQuestion holds the schema definition for the IntelCheckQuestion entity.
// 智力检测题库：逻辑题带标准答案直接比对；绘图题保存参考稿原文与其结构指标，
// 判定时先做相对参考稿的结构门禁，再由评审模型读源码打分。
type IntelCheckQuestion struct {
	ent.Schema
}

func (IntelCheckQuestion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "intel_check_questions"},
	}
}

func (IntelCheckQuestion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.TimeMixin{},
	}
}

func (IntelCheckQuestion) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("kind").
			Values("logic", "drawing"),
		field.String("title").
			NotEmpty().
			MaxLen(100),
		// prompt: 提示词即题目，原文下发给上游。
		field.Text("prompt").
			NotEmpty(),

		// ---- 逻辑题判定字段 ----

		field.String("expected_answer").
			Optional().
			Default("").
			MaxLen(500),
		// match_mode: exact | numeric | contains | regex
		field.String("match_mode").
			Default("exact").
			MaxLen(32).
			Comment("Answer match mode: exact, numeric, contains or regex"),

		// ---- 绘图题判定字段 ----

		// reference_html: 满血模型产出的参考稿原文，既用于算参考指标，
		// 也在第二层评审时作为对照样本一并发给评审模型。
		field.Text("reference_html").
			Optional().
			Default(""),
		// reference_metrics: 上传参考稿时服务端自动计算的结构指标快照。
		field.JSON("reference_metrics", map[string]any{}).
			Optional(),
		// drawing_rules: 第一层门禁配置（min_ratio / required_keywords / max_bytes）。
		field.JSON("drawing_rules", map[string]any{}).
			Optional(),
		// review_rubric: 第二层源码评审的检查清单，为空时使用内置默认清单。
		field.Text("review_rubric").
			Optional().
			Default(""),

		field.Bool("enabled").
			Default(true),
	}
}

func (IntelCheckQuestion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("kind", "enabled"),
	}
}
