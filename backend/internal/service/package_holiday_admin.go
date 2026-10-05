package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	packageHolidayNameMaxRunes = 50
	// packageHolidayMaxSpanDays 单次手动添加的最长日期段，防止误填跨年范围。
	packageHolidayMaxSpanDays = 60
)

// PackageHolidayRange 后台展示的一段连续可冻结日期（同名、同来源、同类型）。
type PackageHolidayRange struct {
	Name   string `json:"name"`
	Start  string `json:"start"`
	End    string `json:"end"`
	Days   int    `json:"days"`
	Kind   string `json:"kind"`
	Source string `json:"source"`
}

// ListHolidayRanges 列出某年的节假日（官方同步 + 手动添加），连续日期合并为一段。
func (s *PackageService) ListHolidayRanges(ctx context.Context, year int) ([]PackageHolidayRange, error) {
	from := time.Date(year, time.January, 1, 0, 0, 0, 0, timezone.Location())
	to := time.Date(year, time.December, 31, 0, 0, 0, 0, timezone.Location())
	days, err := s.repo.ListFreezeDays(ctx, from, to)
	if err != nil {
		return nil, err
	}
	return groupPackageHolidayRanges(days), nil
}

func groupPackageHolidayRanges(days []PackageFreezeDay) []PackageHolidayRange {
	out := []PackageHolidayRange{}
	open := map[string]int{} // source|kind|name → out 中最后一段的下标
	for _, d := range days {
		key := d.Source + "|" + d.Kind + "|" + d.Name
		day := packageDayKey(d.Day)
		if i, ok := open[key]; ok && packageDayKey(parsePackageDay(out[i].End).AddDate(0, 0, 1)) == day {
			out[i].End = day
			out[i].Days++
			continue
		}
		out = append(out, PackageHolidayRange{Name: d.Name, Start: day, End: day, Days: 1, Kind: d.Kind, Source: d.Source})
		open[key] = len(out) - 1
	}
	return out
}

// AddManualHoliday 后台手动添加一段可冻结日期（如平台活动日），同日已有手动记录时覆盖名称。
func (s *PackageService) AddManualHoliday(ctx context.Context, name, start, end string) error {
	name, from, to, err := validatePackageHolidayInput(name, start, end)
	if err != nil {
		return err
	}
	return s.repo.AddManualFreezeDays(ctx, name, from, to)
}

// DeleteManualHoliday 删除一段手动添加的日期；官方同步的记录不能删除（下次同步会恢复）。
func (s *PackageService) DeleteManualHoliday(ctx context.Context, name, start, end string) error {
	name, from, to, err := validatePackageHolidayInput(name, start, end)
	if err != nil {
		return err
	}
	n, err := s.repo.DeleteManualFreezeDays(ctx, name, from, to)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPackageHolidayNotFound
	}
	return nil
}

func validatePackageHolidayInput(name, start, end string) (string, time.Time, time.Time, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > packageHolidayNameMaxRunes {
		return "", time.Time{}, time.Time{}, ErrPackageHolidayInvalid
	}
	from, errFrom := timezone.ParseInLocation("2006-01-02", strings.TrimSpace(start))
	to, errTo := timezone.ParseInLocation("2006-01-02", strings.TrimSpace(end))
	if errFrom != nil || errTo != nil || to.Before(from) || to.Sub(from) > packageHolidayMaxSpanDays*24*time.Hour {
		return "", time.Time{}, time.Time{}, ErrPackageHolidayInvalid
	}
	return name, from, to, nil
}

func parsePackageDay(s string) time.Time {
	t, _ := timezone.ParseInLocation("2006-01-02", s)
	return t
}
