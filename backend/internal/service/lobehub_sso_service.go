package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrLobeHubSSODisabled     = infraerrors.Forbidden("LOBEHUB_SSO_DISABLED", "LobeHub SSO is disabled")
	ErrLobeHubSSOUnauthorized = infraerrors.Unauthorized("LOBEHUB_SSO_UNAUTHORIZED", "invalid LobeHub SSO credentials")
	ErrLobeHubSSOCodeInvalid  = infraerrors.Unauthorized("LOBEHUB_SSO_CODE_INVALID", "SSO code is invalid or expired")
)

const (
	lobeHubProviderGPT = "gpt"

	lobeHubGroupOpenAI = "OpenAI-chat"
)

type LobeHubSSOCodeStore interface {
	Store(ctx context.Context, code string, payload LobeHubSSOCodePayload, ttl time.Duration) error
	Take(ctx context.Context, code string) (*LobeHubSSOCodePayload, error)
}

type LobeHubSSOCodePayload struct {
	UserID    int64     `json:"user_id"`
	ReturnTo  string    `json:"return_to,omitempty"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type LobeHubSSOAuthorizeInput struct {
	UserID   int64
	ReturnTo string
}

type LobeHubSSOAuthorizeResult struct {
	RedirectURL string    `json:"redirect_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type LobeHubSSOExchangeInput struct {
	Code         string
	SharedSecret string
}

type LobeHubSSOExchangeResult struct {
	User       LobeHubSSOUser     `json:"user"`
	APIBaseURL string             `json:"api_base_url"`
	Keys       []LobeHubSSOAPIKey `json:"keys"`
}

type LobeHubSSOUser struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
}

type LobeHubSSOAPIKey struct {
	Provider string `json:"provider"`
	Platform string `json:"platform"`
	Key      string `json:"key"`
	GroupID  int64  `json:"group_id,omitempty"`
}

type LobeHubSSOService struct {
	cfg                *config.Config
	codeStore          LobeHubSSOCodeStore
	userRepo           UserRepository
	groupRepo          GroupRepository
	userSubRepo        UserSubscriptionRepository
	apiKeyService      *APIKeyService
	channelService     *ChannelService
	defaultSubAssigner DefaultSubscriptionAssigner
}

func NewLobeHubSSOService(
	cfg *config.Config,
	codeStore LobeHubSSOCodeStore,
	userRepo UserRepository,
	groupRepo GroupRepository,
	userSubRepo UserSubscriptionRepository,
	apiKeyService *APIKeyService,
	channelService *ChannelService,
	defaultSubAssigner DefaultSubscriptionAssigner,
) *LobeHubSSOService {
	return &LobeHubSSOService{
		cfg:                cfg,
		codeStore:          codeStore,
		userRepo:           userRepo,
		groupRepo:          groupRepo,
		userSubRepo:        userSubRepo,
		apiKeyService:      apiKeyService,
		channelService:     channelService,
		defaultSubAssigner: defaultSubAssigner,
	}
}

func (s *LobeHubSSOService) Authorize(ctx context.Context, input LobeHubSSOAuthorizeInput) (*LobeHubSSOAuthorizeResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.IsActive() {
		return nil, ErrUserNotActive
	}

	code, err := randomURLToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate sso code: %w", err)
	}
	now := time.Now()
	ttl := s.codeTTL()
	expiresAt := now.Add(ttl)
	if err := s.codeStore.Store(ctx, code, LobeHubSSOCodePayload{
		UserID:    user.ID,
		ReturnTo:  strings.TrimSpace(input.ReturnTo),
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}, ttl); err != nil {
		return nil, fmt.Errorf("store sso code: %w", err)
	}

	redirectURL, err := s.buildCallbackURL(code, input.ReturnTo)
	if err != nil {
		return nil, err
	}
	return &LobeHubSSOAuthorizeResult{RedirectURL: redirectURL, ExpiresAt: expiresAt}, nil
}

func (s *LobeHubSSOService) Exchange(ctx context.Context, input LobeHubSSOExchangeInput) (*LobeHubSSOExchangeResult, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.SharedSecret) == "" || strings.TrimSpace(input.SharedSecret) != strings.TrimSpace(s.cfg.LobeHubSSO.SharedSecret) {
		return nil, ErrLobeHubSSOUnauthorized
	}
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return nil, ErrLobeHubSSOCodeInvalid
	}
	payload, err := s.codeStore.Take(ctx, code)
	if err != nil {
		if errors.Is(err, ErrLobeHubSSOCodeInvalid) {
			return nil, err
		}
		return nil, fmt.Errorf("take sso code: %w", err)
	}
	if payload == nil || payload.UserID <= 0 || time.Now().After(payload.ExpiresAt) {
		return nil, ErrLobeHubSSOCodeInvalid
	}
	user, err := s.userRepo.GetByID(ctx, payload.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil || !user.IsActive() {
		return nil, ErrUserNotActive
	}

	keys := []LobeHubSSOAPIKey{}
	if s.cfg.LobeHubSSO.AutoCreateAPIKeys {
		keys, err = s.prepareProviderKeys(ctx, user.ID)
		if err != nil {
			return nil, err
		}
	}

	return &LobeHubSSOExchangeResult{
		User: LobeHubSSOUser{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			AvatarURL: user.AvatarURL,
		},
		APIBaseURL: s.apiBaseURL(),
		Keys:       keys,
	}, nil
}

func (s *LobeHubSSOService) prepareProviderKeys(ctx context.Context, userID int64) ([]LobeHubSSOAPIKey, error) {
	availableGroups, err := s.listLobeHubCandidateGroups(ctx)
	if err != nil {
		return nil, err
	}
	groupByPlatform := make(map[string]Group)
	for _, g := range availableGroups {
		if g.Status != StatusActive {
			continue
		}
		if !isLobeHubChatGroup(g.Platform, g.Name) {
			continue
		}
		current, ok := groupByPlatform[g.Platform]
		if !ok || preferLobeHubGroup(g, current) {
			groupByPlatform[g.Platform] = g
		}
	}

	activePlatforms, err := s.activeChannelPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	targets := []struct {
		provider string
		platform string
	}{
		{provider: lobeHubProviderGPT, platform: PlatformOpenAI},
	}

	out := make([]LobeHubSSOAPIKey, 0, len(targets))
	for _, target := range targets {
		if _, ok := activePlatforms[target.platform]; !ok {
			continue
		}
		group, ok := groupByPlatform[target.platform]
		if !ok {
			continue
		}
		groupID := group.ID
		if err := s.ensureLobeHubGroupAccess(ctx, userID, group); err != nil {
			return nil, err
		}
		key, err := s.findOrCreateProviderKey(ctx, userID, target.provider, target.platform, groupID)
		if err != nil {
			return nil, err
		}
		out = append(out, LobeHubSSOAPIKey{
			Provider: target.provider,
			Platform: target.platform,
			Key:      key.Key,
			GroupID:  groupID,
		})
	}
	return out, nil
}

func (s *LobeHubSSOService) listLobeHubCandidateGroups(ctx context.Context) ([]Group, error) {
	if s.groupRepo == nil {
		return nil, fmt.Errorf("group repository is not configured")
	}
	groups, err := s.groupRepo.ListActiveByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		return nil, fmt.Errorf("list lobehub chat groups: %w", err)
	}
	return groups, nil
}

func isLobeHubChatGroup(platform, name string) bool {
	switch platform {
	case PlatformOpenAI:
		return strings.TrimSpace(name) == lobeHubGroupOpenAI
	default:
		return false
	}
}

func preferLobeHubGroup(candidate, current Group) bool {
	if candidate.SortOrder == current.SortOrder {
		return candidate.ID < current.ID
	}
	return candidate.SortOrder < current.SortOrder
}

func (s *LobeHubSSOService) activeChannelPlatforms(ctx context.Context) (map[string]struct{}, error) {
	channels, err := s.channelService.ListAvailable(ctx)
	if err != nil {
		return nil, fmt.Errorf("list available channels: %w", err)
	}
	out := make(map[string]struct{})
	for _, ch := range channels {
		if ch.Status != StatusActive {
			continue
		}
		for _, g := range ch.Groups {
			if g.Platform != "" {
				out[g.Platform] = struct{}{}
			}
		}
	}
	return out, nil
}

func (s *LobeHubSSOService) ensureLobeHubGroupAccess(ctx context.Context, userID int64, group Group) error {
	if !group.IsSubscriptionType() {
		if group.IsExclusive {
			if s.userRepo == nil {
				return fmt.Errorf("lobehub user repository is not configured")
			}
			if err := s.userRepo.AddGroupToAllowedGroups(ctx, userID, group.ID); err != nil {
				return fmt.Errorf("assign lobehub chat group access: %w", err)
			}
		}
		return nil
	}
	if s.userSubRepo == nil || s.defaultSubAssigner == nil {
		return fmt.Errorf("lobehub subscription assigner is not configured")
	}
	if _, err := s.userSubRepo.GetActiveByUserIDAndGroupID(ctx, userID, group.ID); err == nil {
		return nil
	}
	validityDays := group.DefaultValidityDays
	if validityDays <= 0 {
		validityDays = MaxValidityDays
	}
	_, _, err := s.defaultSubAssigner.AssignOrExtendSubscription(ctx, &AssignSubscriptionInput{
		UserID:       userID,
		GroupID:      group.ID,
		ValidityDays: validityDays,
		AssignedBy:   0,
		Notes:        "auto assigned by LobeHub SSO",
	})
	if err != nil {
		return fmt.Errorf("assign lobehub chat subscription: %w", err)
	}
	return nil
}

func (s *LobeHubSSOService) findOrCreateProviderKey(ctx context.Context, userID int64, provider, platform string, groupID int64) (*APIKey, error) {
	name := s.providerKeyName(provider)
	existing, err := s.apiKeyService.SearchAPIKeys(ctx, userID, name, 20)
	if err != nil {
		return nil, err
	}
	for i := range existing {
		k := &existing[i]
		if k.Name != name || !k.IsActive() {
			continue
		}
		if k.GroupID != nil && *k.GroupID == groupID {
			return k, nil
		}
	}
	return s.apiKeyService.Create(ctx, userID, CreateAPIKeyRequest{
		Name:    name,
		GroupID: &groupID,
	})
}

func (s *LobeHubSSOService) providerKeyName(provider string) string {
	prefix := strings.TrimSpace(s.cfg.LobeHubSSO.APIKeyNamePrefix)
	if prefix == "" {
		prefix = "LobeHub"
	}
	switch provider {
	case lobeHubProviderGPT:
		return prefix + " GPT"
	default:
		return prefix + " " + provider
	}
}

func (s *LobeHubSSOService) ensureEnabled() error {
	if s == nil || s.cfg == nil || !s.cfg.LobeHubSSO.Enabled {
		return ErrLobeHubSSODisabled
	}
	if strings.TrimSpace(s.cfg.LobeHubSSO.SharedSecret) == "" {
		return ErrLobeHubSSODisabled
	}
	return nil
}

func (s *LobeHubSSOService) codeTTL() time.Duration {
	seconds := s.cfg.LobeHubSSO.CodeTTLSeconds
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func (s *LobeHubSSOService) apiBaseURL() string {
	if v := strings.TrimRight(strings.TrimSpace(s.cfg.LobeHubSSO.APIBaseURL), "/"); v != "" {
		return v
	}
	if v := strings.TrimRight(strings.TrimSpace(s.cfg.Server.FrontendURL), "/"); v != "" {
		return v
	}
	return ""
}

func (s *LobeHubSSOService) buildCallbackURL(code, returnTo string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(s.cfg.LobeHubSSO.LobeHubBaseURL))
	if err != nil {
		return "", fmt.Errorf("parse lobehub base url: %w", err)
	}
	callbackPath := strings.TrimSpace(s.cfg.LobeHubSSO.CallbackPath)
	if callbackPath == "" {
		callbackPath = "/linkcode/sso/callback"
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + strings.TrimLeft(callbackPath, "/")
	q := base.Query()
	q.Set("code", code)
	if strings.TrimSpace(returnTo) != "" {
		q.Set("returnTo", strings.TrimSpace(returnTo))
	}
	base.RawQuery = q.Encode()
	return base.String(), nil
}

func randomURLToken(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
