package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// 受检分组字段长度上限，与迁移 238 的列宽一致。
// 在 service 层先拦一道，避免把超长值交给数据库换一条 22001 错误——
// 那条错误信息对管理员毫无指导意义。
const (
	maxIntelCheckTargetNameRunes        = 100
	maxIntelCheckTargetDescriptionRunes = 500
	maxIntelCheckTargetBaseURLRunes     = 500
	maxIntelCheckTargetModelRunes       = 200
	maxIntelCheckTargetRateLabelRunes   = 20
)

// IntelCheckTargetParams 受检分组的创建/更新入参。
//
// APIKey 语义随操作不同：创建时必填；更新时留空表示「不改动现有凭据」——
// 管理端拿到的是脱敏后的 key，回填原值提交会把掩码写进库。
type IntelCheckTargetParams struct {
	Name            string
	Description     string
	BaseURL         string
	APIKey          string
	APIMode         string
	Model           string
	ReasoningEffort string
	RateLabel       string
	Enabled         bool
	SortOrder       int
	CreatedBy       int64
}

// normalizeIntelCheckAPIMode 归一化请求风格，无法识别时回落 responses。
//
// 回落 responses 而非 chat_completions：本功能要求「与 Codex CLI 一致的请求方式」，
// 而 Codex CLI 走的是 responses 接口，推理等级也只在该模式下可控。
func normalizeIntelCheckAPIMode(mode string) string {
	if strings.ToLower(strings.TrimSpace(mode)) == IntelCheckAPIModeChatCompletions {
		return IntelCheckAPIModeChatCompletions
	}
	return IntelCheckAPIModeResponses
}

// validateIntelCheckTargetParams 校验长度与必填项。
func validateIntelCheckTargetParams(p IntelCheckTargetParams, requireAPIKey bool) error {
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return fmt.Errorf("分组名称不能为空")
	}
	if n := len([]rune(name)); n > maxIntelCheckTargetNameRunes {
		return fmt.Errorf("分组名称过长：%d 字，上限 %d 字", n, maxIntelCheckTargetNameRunes)
	}
	if n := len([]rune(strings.TrimSpace(p.Description))); n > maxIntelCheckTargetDescriptionRunes {
		return fmt.Errorf("分组说明过长：%d 字，上限 %d 字", n, maxIntelCheckTargetDescriptionRunes)
	}
	if err := validateIntelCheckBaseURL(p.BaseURL); err != nil {
		return err
	}
	model := strings.TrimSpace(p.Model)
	if model == "" {
		return fmt.Errorf("模型名不能为空")
	}
	if n := len([]rune(model)); n > maxIntelCheckTargetModelRunes {
		return fmt.Errorf("模型名过长：%d 字，上限 %d 字", n, maxIntelCheckTargetModelRunes)
	}
	if n := len([]rune(strings.TrimSpace(p.RateLabel))); n > maxIntelCheckTargetRateLabelRunes {
		return fmt.Errorf("倍率文案过长：%d 字，上限 %d 字", n, maxIntelCheckTargetRateLabelRunes)
	}
	if requireAPIKey && strings.TrimSpace(p.APIKey) == "" {
		return fmt.Errorf("API Key 不能为空")
	}
	return nil
}

// validateIntelCheckBaseURL 校验上游地址。
//
// 只做格式与协议校验，SSRF 防护由 intelCheckHTTPClient 的 safeDialContext 负责——
// 那一层在 dial 时才校验解析出的 IP，能拦住 DNS rebinding 这类
// 「域名合法、解析结果指向内网」的情况，是静态校验做不到的。
func validateIntelCheckBaseURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("上游地址不能为空")
	}
	if n := len([]rune(raw)); n > maxIntelCheckTargetBaseURLRunes {
		return fmt.Errorf("上游地址过长：%d 字，上限 %d 字", n, maxIntelCheckTargetBaseURLRunes)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("上游地址格式不正确：%v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("上游地址必须以 http:// 或 https:// 开头")
	}
	if u.Host == "" {
		return fmt.Errorf("上游地址缺少主机名")
	}
	return nil
}

// applyIntelCheckTargetParams 把入参写入领域对象（不含 APIKey 与 CreatedBy）。
func applyIntelCheckTargetParams(t *IntelCheckTarget, p IntelCheckTargetParams) {
	t.Name = strings.TrimSpace(p.Name)
	t.Description = strings.TrimSpace(p.Description)
	t.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	t.APIMode = normalizeIntelCheckAPIMode(p.APIMode)
	t.Model = strings.TrimSpace(p.Model)
	t.ReasoningEffort = NormalizeIntelCheckReasoningEffort(p.ReasoningEffort)
	t.RateLabel = strings.TrimSpace(p.RateLabel)
	t.Enabled = p.Enabled
	if p.SortOrder < 0 {
		p.SortOrder = 0
	}
	t.SortOrder = p.SortOrder
}

// ListTargets 分页查询受检分组，返回的 APIKey 已解密为明文，handler 负责脱敏。
func (s *IntelCheckService) ListTargets(
	ctx context.Context, params IntelCheckTargetListParams,
) ([]*IntelCheckTarget, int64, error) {
	items, total, err := s.repo.ListTargets(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list intel check targets: %w", err)
	}
	for _, it := range items {
		s.decryptTargetInPlace(it)
	}
	return items, total, nil
}

// GetTarget 查询单个受检分组（APIKey 已解密）。
func (s *IntelCheckService) GetTarget(ctx context.Context, id int64) (*IntelCheckTarget, error) {
	t, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.decryptTargetInPlace(t)
	return t, nil
}

// CreateTarget 创建受检分组，内部加密 APIKey。
func (s *IntelCheckService) CreateTarget(
	ctx context.Context, p IntelCheckTargetParams,
) (*IntelCheckTarget, error) {
	if err := validateIntelCheckTargetParams(p, true); err != nil {
		return nil, err
	}
	encrypted, err := s.encryptAPIKey(p.APIKey)
	if err != nil {
		return nil, err
	}

	t := &IntelCheckTarget{APIKey: encrypted, CreatedBy: p.CreatedBy}
	applyIntelCheckTargetParams(t, p)
	if err := s.repo.CreateTarget(ctx, t); err != nil {
		return nil, fmt.Errorf("create intel check target: %w", err)
	}

	// 不重走 GetTarget 的解密链：明文就在手上，再解一次只会在
	// 加解密组件异常时把刚写好的 key 静默清空（同 ChannelMonitor.Create 的取舍）。
	t.APIKey = strings.TrimSpace(p.APIKey)
	return t, nil
}

// UpdateTarget 更新受检分组。p.APIKey 为空表示保留原凭据。
func (s *IntelCheckService) UpdateTarget(
	ctx context.Context, id int64, p IntelCheckTargetParams,
) (*IntelCheckTarget, error) {
	if err := validateIntelCheckTargetParams(p, false); err != nil {
		return nil, err
	}
	// 读原行拿到现有密文：APIKey 留空时要原样写回，
	// 不能让 UpdateTarget 把该列覆盖成空。
	existing, err := s.repo.GetTargetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	plain := strings.TrimSpace(p.APIKey)
	if plain == "" {
		t := existing
		applyIntelCheckTargetParams(t, p)
		if err := s.repo.UpdateTarget(ctx, t); err != nil {
			return nil, fmt.Errorf("update intel check target: %w", err)
		}
		s.decryptTargetInPlace(t)
		return t, nil
	}

	encrypted, err := s.encryptAPIKey(plain)
	if err != nil {
		return nil, err
	}
	t := existing
	t.APIKey = encrypted
	applyIntelCheckTargetParams(t, p)
	if err := s.repo.UpdateTarget(ctx, t); err != nil {
		return nil, fmt.Errorf("update intel check target: %w", err)
	}
	t.APIKey = plain
	return t, nil
}

// DeleteTarget 删除受检分组。
//
// 关联的 results 由外键 ON DELETE CASCADE 一并删除：分组配置已经不存在时，
// 保留它的历史色块只会在公开页上产生无法点开的孤立数据。
func (s *IntelCheckService) DeleteTarget(ctx context.Context, id int64) error {
	if err := s.repo.DeleteTarget(ctx, id); err != nil {
		return fmt.Errorf("delete intel check target: %w", err)
	}
	return nil
}

// ListEnabledTargets 返回全部启用分组（APIKey 已解密），供 runner 与公开页使用。
func (s *IntelCheckService) ListEnabledTargets(ctx context.Context) ([]*IntelCheckTarget, error) {
	items, err := s.repo.ListEnabledTargets(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		s.decryptTargetInPlace(it)
	}
	return items, nil
}
