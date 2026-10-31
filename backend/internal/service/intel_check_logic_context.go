package service

import (
	"regexp"
	"sort"
	"strings"
)

// 每个候选携带原文位置；候选选择不读取标准答案。
type intelCheckAnswerCandidate struct {
	answer     string
	start      int
	contextEnd int
	signal     int // 盒装3、显式作答2、加粗1、末行0
}

type intelCheckAnswerRank struct {
	primary, declared, declaration, signal, position int
}

type intelCheckAnswerEvent struct {
	at, kind int
}

const (
	intelCheckScopeHeading = iota
	intelCheckScopeSentence
	intelCheckScopeNote
	intelCheckScopeCondition
	intelCheckScopeAlternative
	intelCheckScopePrimary
	intelCheckScopeConclusion
)

var (
	intelCheckHeadingRe     = regexp.MustCompile(`(?m)^[ 	]{0,3}#{1,6}[ 	]+`)
	intelCheckNoteRe        = regexp.MustCompile(`(?im)(?:^|[。！？；;])[ 	#>*_]*(?:注释|注意|备注|补充说明|补充|注|notes?|remarks?)[*_]*[ 	]*(?:[：:]|$)`)
	intelCheckConditionRe   = regexp.MustCompile(`(?i)如果|假如|假设|若|倘若|\bif\b|\bassuming\b|\bsuppos(?:e|ing)\b`)
	intelCheckAlternativeRe = regexp.MustCompile(`(?i)(?:另(?:一(?:种|个)?|外)|不同|换(?:成|为|用)?(?:一(?:种|个)|另一(?:种|个))?|改(?:为|成|用)(?:另一(?:种|个)|一(?:种|个))?)(?:的)?(?:条件|假设|情形|场景|情况)|如果不允许|若不允许|\b(?:different|alternative|another)\s+(?:assumption|condition|scenario)|\bif\s+instead\b|\botherwise\b`)
	intelCheckPrimaryRe     = regexp.MustCompile(`(?i)本题|按题意|题目(?:给定|规定|要求)|\b(?:this|the given|the original)\s+(?:problem|question)\b`)
	intelCheckConclusionRe  = regexp.MustCompile(`(?i)更正|修正|纠正|计算有误|前面(?:算|计算)错|最终(?:答案|结论)|本题(?:的)?答案|故答案|(?:因此|综上)[^。！？；;
]*答案|\bcorrection\b|\bcorrected\b|\bfinal\s+answer\b`)
	intelCheckScaffoldRe = regexp.MustCompile(`(?i)^(?:如下(?:所示)?|见下(?:文|式)?|as follows|the following|(?:以上|上述|这|此).*最终(?:答案|结论)|(?:最终(?:答案|结论)|答案|final answer)(?:如下(?:所示)?|是|为|as follows)?|#{1,6}.*)$`)
	// 英文句号仅在后跟空白时视为句界，避免切断小数。
	intelCheckSentenceEndRe = regexp.MustCompile(`[。！？；;]|[.!?](?:\s|$)`)
)

func selectIntelCheckAnswer(text string, candidates []intelCheckAnswerCandidate) string {
	events := intelCheckAnswerEvents(text)
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].contextEnd < candidates[j].contextEnd })
	var best intelCheckAnswerRank
	answer := ""
	note, other, clauseAlternative := false, false, false
	declaration, nextEvent := -1, 0
	for _, candidate := range candidates {
		// 引导语和说明标签只改变状态，不能自己成为答案。
		scaffold := intelCheckScaffoldRe.MatchString(strings.TrimSpace(candidate.answer))
		if candidate.signal == 0 && (intelCheckAnswerPhraseRe.MatchString(candidate.answer) || intelCheckConclusionRe.MatchString(candidate.answer)) {
			scaffold = true
		}
		for nextEvent < len(events) && events[nextEvent].at <= candidate.contextEnd {
			event := events[nextEvent]
			switch event.kind {
			case intelCheckScopeHeading:
				note, other, clauseAlternative = false, false, false
				declaration = -1
			case intelCheckScopeSentence:
				clauseAlternative = false
			case intelCheckScopeNote:
				note, other, clauseAlternative = true, false, false
				declaration = -1
			case intelCheckScopeCondition:
				if note {
					other = true
				}
			case intelCheckScopeAlternative:
				other, clauseAlternative = true, true
			case intelCheckScopePrimary:
				// 同句的明确另类假设不能靠“本题”二字消除；普通按题意作答可以恢复。
				if !clauseAlternative {
					other, note = false, false
				}
			case intelCheckScopeConclusion:
				declaration = event.at
			}
			nextEvent++
		}
		if scaffold {
			continue
		}
		rank := intelCheckAnswerRank{primary: 1, signal: candidate.signal, position: candidate.start}
		if other {
			rank.primary = 0
		}
		if declaration >= 0 {
			rank.declared = 1
			rank.declaration = declaration
		}
		if answer == "" || rank.betterThan(best) {
			answer, best = candidate.answer, rank
		}
	}
	return answer
}

func (r intelCheckAnswerRank) betterThan(other intelCheckAnswerRank) bool {
	a, b := [5]int{r.primary, r.declared, r.declaration, r.signal, r.position}, [5]int{other.primary, other.declared, other.declaration, other.signal, other.position}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i]
		}
	}
	return false
}

// 标记和候选各排序一次后按原文顺序推进。状态可跨空行、长段落保留，
// 标题在本行起点生效，不会用后面的标题污染前面的候选。
func intelCheckAnswerEvents(text string) []intelCheckAnswerEvent {
	var events []intelCheckAnswerEvent
	patterns := []*regexp.Regexp{intelCheckHeadingRe, intelCheckSentenceEndRe, intelCheckNoteRe, intelCheckConditionRe, intelCheckAlternativeRe, intelCheckPrimaryRe, intelCheckConclusionRe}
	for kind, pattern := range patterns {
		for _, match := range pattern.FindAllStringIndex(text, -1) {
			at := match[1]
			if kind == intelCheckScopeHeading {
				at = match[0]
			}
			events = append(events, intelCheckAnswerEvent{at: at, kind: kind})
		}
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].at == events[j].at {
			return events[i].kind < events[j].kind
		}
		return events[i].at < events[j].at
	})
	return events
}
