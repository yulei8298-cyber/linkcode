package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// SettingKeyPackageHolidaySyncState 节假日同步状态（供后台展示），整块 JSON 存储。
const SettingKeyPackageHolidaySyncState = "package_holiday_sync_state"

const (
	packageHolidayFetchTimeout = 10 * time.Second
	packageHolidayMaxBody      = 1 << 20
)

// PackageHolidaySyncState 最近一次节假日同步的结果。
type PackageHolidaySyncState struct {
	LastSyncedAt *time.Time  `json:"last_synced_at,omitempty"`
	Years        map[int]int `json:"years"` // 年份 → 同步到的天数；0 表示官方尚未公布
	LastError    string      `json:"last_error,omitempty"`
}

// holidayCNFile holiday-cn 的年度文件格式。
type holidayCNFile struct {
	Year int `json:"year"`
	Days []struct {
		Name     string `json:"name"`
		Date     string `json:"date"`
		IsOffDay bool   `json:"isOffDay"`
	} `json:"days"`
}

// parseHolidayCN 解析 holiday-cn 年度文件，只保留属于该年份的日期。
func parseHolidayCN(year int, body []byte) ([]PackageFreezeDay, error) {
	var file holidayCNFile
	if err := json.Unmarshal(body, &file); err != nil {
		return nil, fmt.Errorf("解析节假日数据失败: %w", err)
	}
	if file.Year != 0 && file.Year != year {
		return nil, fmt.Errorf("节假日数据年份不符：期望 %d，实际 %d", year, file.Year)
	}
	days := make([]PackageFreezeDay, 0, len(file.Days))
	for _, d := range file.Days {
		day, err := timezone.ParseInLocation("2006-01-02", d.Date)
		if err != nil || day.Year() != year {
			continue
		}
		kind := PackageDayKindWork
		if d.IsOffDay {
			kind = PackageDayKindOff
		}
		days = append(days, PackageFreezeDay{Day: day, Name: strings.TrimSpace(d.Name), Kind: kind, Source: PackageDaySourceAuto})
	}
	return days, nil
}

// SyncHolidays 拉取今年和明年的官方节假日并整体替换自动同步记录。
// 官方尚未公布（文件不存在或 days 为空）的年份保持原数据不动；网络失败不清空数据。
func (s *PackageService) SyncHolidays(ctx context.Context) (*PackageHolidaySyncState, error) {
	settings, err := s.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now()
	state := &PackageHolidaySyncState{Years: map[int]int{}}
	var errs []string
	for _, year := range []int{now.Year(), now.Year() + 1} {
		days, err := s.fetchHolidayYear(ctx, settings.HolidaySourceURL, year)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%d 年：%v", year, err))
			continue
		}
		state.Years[year] = len(days)
		if len(days) == 0 {
			continue
		}
		if err := s.repo.ReplaceAutoFreezeDays(ctx, year, days); err != nil {
			errs = append(errs, fmt.Sprintf("%d 年：保存失败 %v", year, err))
		}
	}
	state.LastSyncedAt = &now
	state.LastError = strings.Join(errs, "；")
	if raw, mErr := json.Marshal(state); mErr == nil && s.settingRepo != nil {
		_ = s.settingRepo.Set(ctx, SettingKeyPackageHolidaySyncState, string(raw))
	}
	if len(errs) > 0 && len(state.Years) == 0 {
		return state, errors.New(state.LastError)
	}
	return state, nil
}

// GetHolidaySyncState 读取最近一次同步结果；从未同步过时返回空状态。
func (s *PackageService) GetHolidaySyncState(ctx context.Context) PackageHolidaySyncState {
	state := PackageHolidaySyncState{Years: map[int]int{}}
	if s.settingRepo == nil {
		return state
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyPackageHolidaySyncState)
	if err == nil && strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &state)
	}
	return state
}

func (s *PackageService) fetchHolidayYear(ctx context.Context, sourceURL string, year int) ([]PackageFreezeDay, error) {
	url := strings.ReplaceAll(sourceURL, "{year}", strconv.Itoa(year))
	reqCtx, cancel := context.WithTimeout(ctx, packageHolidayFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: packageHolidayFetchTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, packageHolidayMaxBody))
	if err != nil {
		return nil, err
	}
	return parseHolidayCN(year, body)
}
