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

// IntelCheckRound holds the schema definition for the IntelCheckRound entity.
// 智力检测轮次：一次调度对所有受检分组跑一遍题目。明细见 intel_check_results，
// 超过保留期由每日维护任务物理删除（日志类表不做软删除）。
type IntelCheckRound struct {
	ent.Schema
}

func (IntelCheckRound) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "intel_check_rounds"},
	}
}

func (IntelCheckRound) Fields() []ent.Field {
	return []ent.Field{
		// seq: 对外展示的自增轮次号（如 #1284），与主键解耦，清理旧数据后不回退。
		field.Int64("seq").
			Unique(),
		field.Time("started_at").
			Default(time.Now),
		field.Time("finished_at").
			Optional().
			Nillable(),
		// 本轮实际抽到的题目；题目被删除时置空，历史轮次仍保留。
		field.Int64("logic_question_id").
			Optional().
			Nillable(),
		field.Int64("drawing_question_id").
			Optional().
			Nillable(),
		// trigger_source: cron | manual。
		// 列名不用 trigger：TRIGGER 是 Postgres 保留字，手写 SQL 需额外加引号。
		field.String("trigger_source").
			Default("cron").
			MaxLen(16).
			Comment("Round trigger: cron or manual"),
	}
}

func (IntelCheckRound) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("results", IntelCheckResult.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (IntelCheckRound) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("started_at"),
	}
}
