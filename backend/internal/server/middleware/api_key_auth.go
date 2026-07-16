package middleware

import (
	"bytes"
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	chatStationSecretHeader           = "X-LinkCode-Chat-Station-Secret"
	chatStationUserIDHeader           = "X-LinkCode-Chat-Station-User-ID"
	chatStationAPIKeyResponseHeader   = "X-LinkCode-Chat-Station-API-Key"
	chatStationRestrictedErrorCode    = "CHAT_STATION_GROUP_RESTRICTED"
	chatStationRestrictedErrorMessage = "当前分组仅允许通过对话站调用"
	chatStationModelProbeBytes        = 64 * 1024
)

// NewAPIKeyAuthMiddleware 创建 API Key 认证中间件
func NewAPIKeyAuthMiddleware(apiKeyService *service.APIKeyService, subscriptionService *service.SubscriptionService, cfg *config.Config) APIKeyAuthMiddleware {
	return APIKeyAuthMiddleware(apiKeyAuthWithSubscription(apiKeyService, subscriptionService, cfg))
}

// apiKeyAuthWithSubscription API Key认证中间件（支持订阅验证）
//
// 中间件职责分为两层：
//   - 鉴权（Authentication）：验证 Key 有效性、用户状态、IP 限制 —— 始终执行
//   - 计费执行（Billing Enforcement）：过期/配额/订阅/余额检查 —— skipBilling 时整块跳过
//
// /v1/usage、/v1/sub2api/billing 端点与异步生图任务查询只需鉴权，不需要计费执行。
// usage 允许过期/配额耗尽的 Key 查询自身用量，billing 用于读取当前 Key 的倍率配置，
// 异步生图查询允许已耗尽额度的 Key 拉取自身任务结果。
func apiKeyAuthWithSubscription(apiKeyService *service.APIKeyService, subscriptionService *service.SubscriptionService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── 1. 提取 API Key ──────────────────────────────────────────

		queryKey := strings.TrimSpace(c.Query("key"))
		queryApiKey := strings.TrimSpace(c.Query("api_key"))
		if queryKey != "" || queryApiKey != "" {
			AbortWithError(c, 400, "api_key_in_query_deprecated", "API key in query parameter is deprecated. Please use Authorization header instead.")
			return
		}

		// 尝试从Authorization header中提取API key (Bearer scheme)
		authHeader := c.GetHeader("Authorization")
		var apiKeyString string

		if authHeader != "" {
			// 验证Bearer scheme
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				apiKeyString = strings.TrimSpace(parts[1])
			}
		}

		// 如果Authorization header中没有，尝试从x-api-key header中提取
		if apiKeyString == "" {
			apiKeyString = c.GetHeader("x-api-key")
		}

		// 如果x-api-key header中没有，尝试从x-goog-api-key header中提取（Gemini CLI兼容）
		if apiKeyString == "" {
			apiKeyString = c.GetHeader("x-goog-api-key")
		}

		// 如果所有header都没有API key
		if apiKeyString == "" {
			var bootstrapped bool
			apiKeyString, bootstrapped = bootstrapChatStationAPIKey(c, apiKeyService, cfg)
			if c.IsAborted() {
				return
			}
			if !bootstrapped {
				AbortWithError(c, 401, "API_KEY_REQUIRED", "API key is required in Authorization header (Bearer scheme), x-api-key header, or x-goog-api-key header")
				return
			}
			c.Header(chatStationAPIKeyResponseHeader, apiKeyString)
		}

		// ── 2. 验证 Key 存在 ─────────────────────────────────────────

		apiKey, err := apiKeyService.GetByKey(c.Request.Context(), apiKeyString)
		if err != nil {
			if errors.Is(err, service.ErrAPIKeyNotFound) {
				AbortWithError(c, 401, "INVALID_API_KEY", "Invalid API key")
				return
			}
			AbortWithError(c, 500, "INTERNAL_ERROR", "Failed to validate API key")
			return
		}

		// apiKey 已加载（含 User/Group）。即便后续因分组停用/Key 停用/用户停用/
		// IP 限制等早退中断，也让 Ops 错误日志能回退取到 user/group/platform。
		SetOpsFallbackAPIKey(c, apiKey)

		// ── 3. 基础鉴权（始终执行） ─────────────────────────────────

		// disabled / 未知状态 → 无条件拦截（expired 和 quota_exhausted 留给计费阶段）
		if !apiKey.IsActive() &&
			apiKey.Status != service.StatusAPIKeyExpired &&
			apiKey.Status != service.StatusAPIKeyQuotaExhausted {
			AbortWithError(c, 401, "API_KEY_DISABLED", "API key is disabled")
			return
		}

		// 检查 IP 限制（白名单/黑名单）
		// 注意：错误信息故意模糊，避免暴露具体的 IP 限制机制
		clientIP := ip.GetTrustedClientIP(c)
		if cfg.TrustForwardedIPForAPIKeyACL() {
			clientIP = ip.GetClientIP(c)
		}
		if apiKey.Group != nil && (len(apiKey.Group.IPWhitelist) > 0 || len(apiKey.Group.IPBlacklist) > 0) {
			allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, apiKey.Group.CompiledIPWhitelist, apiKey.Group.CompiledIPBlacklist)
			if !allowed {
				if clientIP == "" {
					clientIP = "unknown"
				}
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)
				AbortWithError(c, 403, "ACCESS_DENIED", fmt.Sprintf("Access denied. Your IP is %s", clientIP))
				return
			}
		}
		if len(apiKey.IPWhitelist) > 0 || len(apiKey.IPBlacklist) > 0 {
			allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, apiKey.CompiledIPWhitelist, apiKey.CompiledIPBlacklist)
			if !allowed {
				if clientIP == "" {
					clientIP = "unknown"
				}
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)
				AbortWithError(c, 403, "ACCESS_DENIED", fmt.Sprintf("Access denied. Your IP is %s", clientIP))
				return
			}
		}
		if abortIfChatStationGroupSecretInvalid(c, apiKey, cfg) {
			return
		}

		// 检查关联的用户
		if apiKey.User == nil {
			AbortWithError(c, 401, "USER_NOT_FOUND", "User associated with API key not found")
			return
		}

		// 检查用户状态
		if !apiKey.User.IsActive() {
			AbortWithError(c, 401, "USER_INACTIVE", "User account is not active")
			return
		}
		if abortIfAPIKeyGroupUnavailable(c, apiKey) {
			return
		}
		if abortIfAPIKeyGroupNotAllowed(c, apiKey) {
			return
		}
		dailyFreeUsageDate := timezone.Now()
		if err := apiKeyService.ValidateDailyFreeAllowanceAt(c.Request.Context(), apiKey, dailyFreeUsageDate); err != nil {
			if errors.Is(err, service.ErrDailyFreeLimitExceeded) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
				AbortWithError(c, http.StatusTooManyRequests, "DAILY_FREE_LIMIT_EXCEEDED", "当前每日免费额度已用完，请明天再使用")
				return
			}
			AbortWithError(c, http.StatusInternalServerError, "DAILY_FREE_USAGE_CHECK_FAILED", "每日免费额度检查失败")
			return
		}
		ctx := context.WithValue(c.Request.Context(), ctxkey.UserID, apiKey.User.ID)
		if apiKey.Group != nil && apiKey.Group.IsFree {
			ctx = context.WithValue(ctx, ctxkey.DailyFreeUsageDate, dailyFreeUsageDate)
		}
		c.Request = c.Request.WithContext(ctx)
		billingInfoRequest := c.Request.URL.Path == "/v1/sub2api/billing"
		// Async image task polling only reads data that already belongs to the
		// authenticated key and must remain available after the completed
		// generation consumes the key's remaining balance.
		skipBilling := c.Request.URL.Path == "/v1/usage" || billingInfoRequest || isAsyncImageTaskRead(c.Request.Method, c.Request.URL.Path)

		// ── 4. SimpleMode → early return ─────────────────────────────

		if cfg.RunMode == config.RunModeSimple {
			c.Set(string(ContextKeyAPIKey), apiKey)
			c.Set(string(ContextKeyUser), AuthSubject{
				UserID:      apiKey.User.ID,
				Concurrency: apiKey.User.Concurrency,
			})
			c.Set(string(ContextKeyUserRole), apiKey.User.Role)
			setGroupContext(c, apiKey.Group)
			if !billingInfoRequest {
				_ = apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
			}
			c.Next()
			return
		}

		// ── 5. 按端点需要加载订阅 ───────────────────────────────────

		var subscription *service.UserSubscription
		isSubscriptionType := apiKey.Group != nil && apiKey.Group.IsSubscriptionType()

		// 倍率自省不需要订阅数据；/v1/usage 仍保留原有订阅读取行为。
		if isSubscriptionType && subscriptionService != nil && !billingInfoRequest {
			sub, subErr := subscriptionService.GetActiveSubscription(
				c.Request.Context(),
				apiKey.User.ID,
				apiKey.Group.ID,
			)
			if subErr != nil {
				if !skipBilling {
					AbortWithError(c, 403, "SUBSCRIPTION_NOT_FOUND", "No active subscription found for this group")
					return
				}
				// skipBilling: 订阅不存在也放行，handler 会返回可用的数据
			} else {
				subscription = sub
			}
		}

		// ── 6. 计费执行（skipBilling 时整块跳过） ────────────────────

		if !skipBilling {
			// Key 状态检查
			switch apiKey.Status {
			case service.StatusAPIKeyQuotaExhausted:
				AbortWithError(c, 429, "API_KEY_QUOTA_EXHAUSTED", "API key 额度已用完")
				return
			case service.StatusAPIKeyExpired:
				AbortWithError(c, 403, "API_KEY_EXPIRED", "API key 已过期")
				return
			}

			// 运行时过期/配额检查（即使状态是 active，也要检查时间和用量）
			if apiKey.IsExpired() {
				AbortWithError(c, 403, "API_KEY_EXPIRED", "API key 已过期")
				return
			}
			if apiKey.IsQuotaExhausted() {
				AbortWithError(c, 429, "API_KEY_QUOTA_EXHAUSTED", "API key 额度已用完")
				return
			}

			// 订阅模式：验证订阅限额
			if subscription != nil {
				needsMaintenance, validateErr := subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)
				if needsMaintenance {
					refreshed, maintenanceErr := subscriptionService.EnsureWindowMaintenance(c.Request.Context(), subscription)
					if maintenanceErr != nil {
						AbortWithError(c, 500, "SUBSCRIPTION_MAINTENANCE_FAILED", "Failed to maintain subscription usage windows")
						return
					}
					subscription = refreshed
					_, validateErr = subscriptionService.ValidateAndCheckLimits(subscription, apiKey.Group)
				}
				if validateErr != nil {
					code := "SUBSCRIPTION_INVALID"
					status := 403
					if errors.Is(validateErr, service.ErrDailyLimitExceeded) ||
						errors.Is(validateErr, service.ErrWeeklyLimitExceeded) ||
						errors.Is(validateErr, service.ErrMonthlyLimitExceeded) {
						code = "USAGE_LIMIT_EXCEEDED"
						status = 429
					}
					AbortWithError(c, status, code, validateErr.Error())
					return
				}
			} else {
				// 非订阅模式 或 订阅模式但 subscriptionService 未注入：回退到余额检查
				if (apiKey.Group == nil || !apiKey.Group.IsFree) && apiKeyBalanceBelowAuthThreshold(apiKey.User.Balance, cfg) {
					AbortWithError(c, 403, "INSUFFICIENT_BALANCE", "Insufficient account balance")
					return
				}
			}
		}

		// ── 7. 设置上下文 → Next ─────────────────────────────────────

		if subscription != nil {
			c.Set(string(ContextKeySubscription), subscription)
		}
		c.Set(string(ContextKeyAPIKey), apiKey)
		c.Set(string(ContextKeyUser), AuthSubject{
			UserID:      apiKey.User.ID,
			Concurrency: apiKey.User.Concurrency,
		})
		c.Set(string(ContextKeyUserRole), apiKey.User.Role)
		setGroupContext(c, apiKey.Group)
		if !billingInfoRequest {
			_ = apiKeyService.TouchLastUsed(c.Request.Context(), apiKey.ID)
		}

		c.Next()
	}
}

func isAsyncImageTaskRead(method, path string) bool {
	if method != http.MethodGet {
		return false
	}
	return strings.HasPrefix(path, "/v1/images/tasks/") || strings.HasPrefix(path, "/images/tasks/")
}

// GetAPIKeyFromContext 从上下文中获取API key
func GetAPIKeyFromContext(c *gin.Context) (*service.APIKey, bool) {
	value, exists := c.Get(string(ContextKeyAPIKey))
	if !exists {
		return nil, false
	}
	apiKey, ok := value.(*service.APIKey)
	return apiKey, ok
}

// SetOpsFallbackAPIKey 记录已加载的 API Key，供 Ops 错误日志在鉴权早退时回退使用。
// 与 ContextKeyAPIKey 区分：写入它不代表请求已通过鉴权，因此不影响 handler、
// 审计日志等对“已鉴权”的判断。
func SetOpsFallbackAPIKey(c *gin.Context, apiKey *service.APIKey) {
	if c == nil || apiKey == nil {
		return
	}
	c.Set(string(ContextKeyOpsFallbackAPIKey), apiKey)
}

// GetOpsFallbackAPIKey 读取 Ops 错误日志专用的回退 API Key。
func GetOpsFallbackAPIKey(c *gin.Context) (*service.APIKey, bool) {
	value, exists := c.Get(string(ContextKeyOpsFallbackAPIKey))
	if !exists {
		return nil, false
	}
	apiKey, ok := value.(*service.APIKey)
	return apiKey, ok
}

// GetSubscriptionFromContext 从上下文中获取订阅信息
func GetSubscriptionFromContext(c *gin.Context) (*service.UserSubscription, bool) {
	value, exists := c.Get(string(ContextKeySubscription))
	if !exists {
		return nil, false
	}
	subscription, ok := value.(*service.UserSubscription)
	return subscription, ok
}

func isGatewayUsagePath(path string) bool {
	p := strings.TrimRight(strings.TrimSpace(path), "/")
	switch p {
	case "/v1/usage",
		"/v1/v1/usage",
		"/antigravity/v1/usage",
		"/antigravity/v1/v1/usage":
		return true
	default:
		return false
	}
}

func setGroupContext(c *gin.Context, group *service.Group) {
	if !service.IsGroupContextValid(group) {
		return
	}
	if existing, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group); ok && existing != nil && existing.ID == group.ID && service.IsGroupContextValid(existing) {
		return
	}
	ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
	c.Request = c.Request.WithContext(ctx)
}

// apiKeyBalanceBelowAuthThreshold 保持鉴权层的历史语义：仅在余额耗尽（<=0）时拒绝。
// MinimumBalanceReserve 只作为 billing-cache 预检的保守下限，不得复用为鉴权硬门槛，
// 否则已配置该值的存量部署升级后，0 < balance < reserve 的用户会在所有端点被静默 403。
func apiKeyBalanceBelowAuthThreshold(balance float64, _ *config.Config) bool {
	return balance <= 0
}

func abortIfAPIKeyGroupUnavailable(c *gin.Context, apiKey *service.APIKey) bool {
	code, message, ok := validateAPIKeyGroupAvailable(apiKey)
	if ok {
		return false
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
	status := http.StatusForbidden
	if code == "INVALID_API_KEY" {
		status = http.StatusUnauthorized
	}
	AbortWithError(c, status, code, message)
	return true
}

func abortIfAPIKeyGroupNotAllowed(c *gin.Context, apiKey *service.APIKey) bool {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.IsFree && apiKey.Group.ChatStationOnly {
		return false
	}
	if validateAPIKeyGroupAllowed(apiKey) {
		return false
	}
	service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable)
	AbortWithError(c, 403, "GROUP_NOT_ALLOWED", "API Key 所属专属分组不再允许当前用户使用")
	return true
}

func abortIfChatStationGroupSecretInvalid(c *gin.Context, apiKey *service.APIKey, cfg *config.Config) bool {
	if isChatStationGroupSecretInvalid(c, apiKey, cfg) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonIPRestriction)
		AbortWithError(c, 403, chatStationRestrictedErrorCode, chatStationRestrictedErrorMessage)
		return true
	}
	return false
}

func isChatStationGroupSecretInvalid(c *gin.Context, apiKey *service.APIKey, cfg *config.Config) bool {
	if apiKey == nil || apiKey.Group == nil || !apiKey.Group.ChatStationOnly {
		return false
	}
	return !isChatStationSecretValid(c, cfg)
}

func isChatStationSecretValid(c *gin.Context, cfg *config.Config) bool {
	expected := ""
	if cfg != nil {
		expected = strings.TrimSpace(cfg.LobeHubSSO.SharedSecret)
	}
	actual := strings.TrimSpace(c.GetHeader(chatStationSecretHeader))
	return expected != "" && actual != "" && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func validateAPIKeyGroupAllowed(apiKey *service.APIKey) bool {
	if apiKey == nil || apiKey.GroupID == nil || apiKey.User == nil || apiKey.Group == nil {
		return true
	}
	group := apiKey.Group
	if group.IsSubscriptionType() {
		return true
	}
	return apiKey.User.CanBindGroup(group.ID, group.IsExclusive)
}

func validateAPIKeyGroupAvailable(apiKey *service.APIKey) (string, string, bool) {
	if apiKey == nil || apiKey.GroupID == nil {
		return "", "", true
	}
	group := apiKey.Group
	if group == nil || strings.EqualFold(group.Status, "deleted") {
		return "INVALID_API_KEY", "API 密钥无效或所属分组不存在", false
	}
	if !group.IsActive() {
		return "GROUP_DISABLED", "API Key 所属分组已停用", false
	}
	return "", "", true
}

func bootstrapChatStationAPIKey(c *gin.Context, apiKeyService *service.APIKeyService, cfg *config.Config) (string, bool) {
	secret := strings.TrimSpace(c.GetHeader(chatStationSecretHeader))
	rawUserID := strings.TrimSpace(c.GetHeader(chatStationUserIDHeader))
	if secret == "" && rawUserID == "" {
		return "", false
	}
	if !isChatStationSecretValid(c, cfg) {
		AbortWithError(c, http.StatusForbidden, chatStationRestrictedErrorCode, chatStationRestrictedErrorMessage)
		return "", false
	}
	userID, err := strconv.ParseInt(rawUserID, 10, 64)
	if err != nil || userID <= 0 {
		AbortWithError(c, http.StatusUnauthorized, "CHAT_STATION_USER_INVALID", "对话站用户身份无效，请重新登录")
		return "", false
	}
	platform := chatStationPlatformFromRequest(c.Request)
	if platform == "" {
		AbortWithError(c, http.StatusUnauthorized, "CHAT_STATION_FREE_GROUP_NOT_FOUND", "当前请求不支持自动创建免费密钥，请前往设置密钥")
		return "", false
	}
	key, err := apiKeyService.ResolveChatStationAPIKey(c.Request.Context(), userID, platform)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrChatStationFreeGroupNotFound):
			AbortWithError(c, http.StatusUnauthorized, "CHAT_STATION_FREE_GROUP_NOT_FOUND", "当前没有可用的对话站免费分组，请前往设置密钥")
		case errors.Is(err, service.ErrUserNotActive), errors.Is(err, service.ErrUserNotFound):
			AbortWithError(c, http.StatusUnauthorized, "CHAT_STATION_USER_INVALID", "对话站用户身份无效，请重新登录")
		default:
			AbortWithError(c, http.StatusInternalServerError, "CHAT_STATION_KEY_PROVISION_FAILED", "对话站密钥创建失败")
		}
		return "", false
	}
	return key, true
}

func chatStationPlatformFromRequest(req *http.Request) string {
	if req == nil || req.Method != http.MethodPost {
		return ""
	}
	path := strings.TrimRight(strings.TrimSpace(req.URL.Path), "/")
	var platform string
	switch {
	case strings.HasSuffix(path, "/messages"):
		platform = service.PlatformAnthropic
	case strings.HasSuffix(path, "/chat/completions"),
		strings.Contains(path, "/responses"),
		strings.HasSuffix(path, "/images/generations"),
		strings.HasSuffix(path, "/videos/generations"),
		strings.HasSuffix(path, "/videos/edits"),
		strings.HasSuffix(path, "/videos/extensions"):
		platform = service.PlatformOpenAI
	default:
		return ""
	}

	if platform == service.PlatformOpenAI && chatStationRequestUsesGrokModel(req) {
		return service.PlatformGrok
	}
	return platform
}

// chatStationRequestUsesGrokModel samples the request prefix and restores it
// before the handler runs. The model field is intentionally read before an API
// key exists, so the first Grok request can receive a Grok free-group key.
func chatStationRequestUsesGrokModel(req *http.Request) bool {
	if req == nil || req.Body == nil || req.Body == http.NoBody {
		return false
	}

	remaining := req.Body
	probe, err := io.ReadAll(io.LimitReader(remaining, chatStationModelProbeBytes))
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(probe), remaining))
	if err != nil || len(probe) == 0 {
		return false
	}

	model := strings.ToLower(strings.TrimSpace(gjson.GetBytes(probe, "model").String()))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = strings.TrimSpace(model[slash+1:])
	}
	return model == "grok" || strings.HasPrefix(model, "grok-")
}
