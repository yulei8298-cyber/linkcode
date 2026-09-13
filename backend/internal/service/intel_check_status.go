package service

// DeriveIntelCheckState 依据 degraded_rule 推导分组的智力状态。
//
// statuses 为同一分组同一题型的历史结果状态，**按时间倒序**（statuses[0] 最新）。
// request_error 与 running 表示「这次没测出结论」，不参与连续计数，也不会打断连续段：
// 上游抖动不应被当成降智证据，同样也不应让一段失败被意外「洗白」。
//
// previous 为该分组上一次推导出的状态，用于实现带滞回的状态机，
// 避免在阈值边界上来回抖动；首次推导传空字符串即可。
func DeriveIntelCheckState(statuses []string, rule IntelCheckDegradedRule, previous string) string {
	rule = normalizeIntelCheckDegradedRule(rule)

	decisive := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if status == IntelCheckStatusPass || status == IntelCheckStatusFail {
			decisive = append(decisive, status)
		}
	}
	if len(decisive) == 0 {
		return IntelCheckStateUnknown
	}

	leading := decisive[0]
	streak := 0
	for _, status := range decisive {
		if status != leading {
			break
		}
		streak++
	}

	if previous == IntelCheckStateDegraded {
		// 已判降智，需要连续 RecoverStreak 次通过才恢复。
		if leading == IntelCheckStatusPass && streak >= rule.RecoverStreak {
			return IntelCheckStateNormal
		}
		return IntelCheckStateDegraded
	}
	// 正常（或首次推导）：连续 FailStreak 次失败才判降智。
	if leading == IntelCheckStatusFail && streak >= rule.FailStreak {
		return IntelCheckStateDegraded
	}
	return IntelCheckStateNormal
}

// ReplayIntelCheckState 从最旧一条结果起逐条推进状态机，得出当前状态。
//
// 公开页必须走回放而不能拿全量历史调一次 DeriveIntelCheckState：滞回逻辑依赖
// previous 入参，而库里没有任何列保存「上一次判定的状态」。若 previous 恒传空，
// 「连续 M 次通过才恢复」就失效了——某组连续 3 次失败已判降智、其后只通过 1 次时，
// 单次调用看到 leading=pass、streak=1，不满足降智条件便直接返回「正常」，
// 而 recover_streak=2 要求它仍是「疑似降智」。状态本身由历史回放得出，
// 因此不需要为它加一列，也就不存在存储状态与历史对不上的问题。
//
// statusesAsc 按时间升序（statusesAsc[0] 最旧），与时间线色块从左到右的顺序一致。
func ReplayIntelCheckState(statusesAsc []string, rule IntelCheckDegradedRule) string {
	// DeriveIntelCheckState 要求倒序入参，这里反转一次后用后缀切片取窗口：
	// desc[i:] 恰好是「截至升序第 len-1-i 条」的全部历史（倒序），
	// 于是 i 从大到小走一遍就是时间从旧到新推进一遍，无需反复分配切片。
	desc := make([]string, 0, len(statusesAsc))
	for i := len(statusesAsc) - 1; i >= 0; i-- {
		desc = append(desc, statusesAsc[i])
	}

	state := ""
	for i := len(desc) - 1; i >= 0; i-- {
		state = DeriveIntelCheckState(desc[i:], rule, state)
	}
	if state == "" {
		return IntelCheckStateUnknown
	}
	return state
}

// IntelCheckPassRate 统计通过率，分母只含 pass 与 fail。
// 无有效样本时返回 (0, false)，调用方据此显示「暂无数据」而非 0%。
func IntelCheckPassRate(statuses []string) (float64, bool) {
	var passed, failed int64
	for _, status := range statuses {
		switch status {
		case IntelCheckStatusPass:
			passed++
		case IntelCheckStatusFail:
			failed++
		}
	}
	return intelCheckPassRate(passed, failed)
}

// intelCheckPassRate 比值本体。
//
// 单独抽出是因为通过率有两个数据来源：时间线拿到的是状态列表，而 24 小时统计
// 走的是 SQL 聚合出来的计数。两边各写一遍比值与「无样本」判断，就会出现
// 一处把无样本当 0% 另一处当「暂无数据」的分裂。
func intelCheckPassRate(passed, failed int64) (float64, bool) {
	total := passed + failed
	if total == 0 {
		return 0, false
	}
	return float64(passed) / float64(total), true
}

// normalizeIntelCheckDegradedRule 兜底非法阈值，保证状态机不会因配置为 0 而恒判降智。
func normalizeIntelCheckDegradedRule(rule IntelCheckDegradedRule) IntelCheckDegradedRule {
	rule.FailStreak = clampIntelCheckInt(rule.FailStreak, 1, intelCheckMaxStreak, intelCheckDefaultFailStreak)
	rule.RecoverStreak = clampIntelCheckInt(rule.RecoverStreak, 1, intelCheckMaxStreak, intelCheckDefaultRecoverStreak)
	return rule
}
