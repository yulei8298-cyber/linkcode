package service

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	PackageCalendarWeekend = "weekend" // 周末
	PackageCalendarHoliday = "holiday" // 法定或自定义节假日
	PackageCalendarMakeup  = "makeup"  // 调休补班的周末（按规则仍可冻结）
	PackageCalendarNone    = "none"    // 工作日，不可冻结
)

// PackageCalendarDay 冻结日历的一天。
type PackageCalendarDay struct {
	Date      string `json:"date"`
	Weekday   int    `json:"weekday"`
	Freezable bool   `json:"freezable"`
	Kind      string `json:"kind"`
	Label     string `json:"label"`
}

// packageDayIndex 以 "YYYY-MM-DD" 为键索引可冻结日期记录。
// 同一天既有官方记录又有手动记录时，放假（off）优先于补班（work）。
type packageDayIndex map[string]PackageFreezeDay

func newPackageDayIndex(days []PackageFreezeDay) packageDayIndex {
	idx := make(packageDayIndex, len(days))
	for _, d := range days {
		key := packageDayKey(d.Day)
		if existing, ok := idx[key]; ok && existing.Kind == PackageDayKindOff {
			continue
		}
		idx[key] = d
	}
	return idx
}

// classify 判断某天能否冻结，以及日历上的类型与名称。
func (idx packageDayIndex) classify(day time.Time, enabled bool) PackageCalendarDay {
	local := day.In(timezone.Location())
	out := PackageCalendarDay{Date: packageDayKey(local), Weekday: int(local.Weekday()), Kind: PackageCalendarNone}
	weekend := local.Weekday() == time.Saturday || local.Weekday() == time.Sunday
	rec, hasRec := idx[out.Date]
	switch {
	case hasRec && rec.Kind == PackageDayKindOff:
		out.Kind, out.Label = PackageCalendarHoliday, rec.Name
	case hasRec && rec.Kind == PackageDayKindWork && weekend:
		out.Kind, out.Label = PackageCalendarMakeup, "调休"
	case weekend:
		out.Kind, out.Label = PackageCalendarWeekend, "周末"
	}
	out.Freezable = enabled && out.Kind != PackageCalendarNone
	return out
}

// buildPackageCalendar 生成 [from, to] 每一天的冻结日历（按服务时区的自然日）。
func buildPackageCalendar(from, to time.Time, days []PackageFreezeDay, enabled bool) []PackageCalendarDay {
	idx := newPackageDayIndex(days)
	start := timezone.StartOfDay(from)
	end := timezone.StartOfDay(to)
	var out []PackageCalendarDay
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		out = append(out, idx.classify(d, enabled))
	}
	return out
}

func packageDayKey(t time.Time) string {
	return t.In(timezone.Location()).Format("2006-01-02")
}
