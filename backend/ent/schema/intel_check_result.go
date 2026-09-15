package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// IntelCheckResult holds the schema definition for the IntelCheckResult entity.
// 智力检测明细：每轮 × 每分组 × 每题一行。发起请求时先落 running 行（前端显示
// 「检测中」浅蓝块），拿到结果后原地更新，因此同一 (round, target, kind) 只有一行。
type IntelCheckResult struct {
	ent.Schema
}

func (IntelCheckResult) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "intel_check_results"},
	}
}

func (IntelCheckResult) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("round_id"),
		field.Int64("target_id"),
		field.Enum("kind").
			Values("logic", "drawing"),
		// status: running 为中间态；request_error 表示上游/网络故障，
		// unverified 表示绘图产物缺少可测量动作证据；
		// 不计入降智连续计数（见 DeriveIntelCheckState）。
		field.Enum("status").
			Values("pass", "fail", "request_error", "running", "unverified"),
		field.Int("latency_ms").
			Optional().
			Nillable(),
		// prompt_snapshot: 本轮实际发送的题面，题目后续被改动也能还原当次上下文。
		field.Text("prompt_snapshot").
			Optional().
			Default(""),
		field.Text("raw_reply").
			Optional().
			Default(""),
		field.String("extracted_answer").
			Optional().
			Default("").
			MaxLen(500),
		// html_output: 原样保留的绘图产物，仅绘图题有值。
		field.Text("html_output").
			Optional().
			Default(""),
		// judge_detail: 逐项判定结果（结构门禁 + 动作验收依据）。
		field.JSON("judge_detail", map[string]any{}).
			Optional(),
		field.String("error_message").
			Optional().
			Default("").
			MaxLen(500),
		field.Int("input_tokens").
			Optional().
			Nillable(),
		field.Int("output_tokens").
			Optional().
			Nillable(),
		field.Time("checked_at").
			Default(time.Now),
	}
}

func (IntelCheckResult) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("round", IntelCheckRound.Type).
			Ref("results").
			Field("round_id").
			Unique().
			Required(),
		edge.From("target", IntelCheckTarget.Type).
			Ref("results").
			Field("target_id").
			Unique().
			Required(),
	}
}

func (IntelCheckResult) Indexes() []ent.Index {
	return []ent.Index{
		// 时间线查询主索引：Postgres 可反向扫描，无需单独建 DESC 索引。
		index.Fields("target_id", "kind", "checked_at"),
		index.Fields("round_id"),
		// 同一轮次 × 分组 × 题型只允许一行：running 行由后续结果原地更新。
		// 缺了这个约束，一次重试就会让时间线凭空多出一个色块。
		index.Fields("round_id", "target_id", "kind").Unique(),
	}
}
