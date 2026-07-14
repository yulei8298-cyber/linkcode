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

	return &LobeHubSSOExchangeResult{
		User: LobeHubSSOUser{
			ID:        user.ID,
			Email:     user.Email,
			Username:  user.Username,
			AvatarURL: user.AvatarURL,
		},
		APIBaseURL: s.apiBaseURL(),
		Keys:       []LobeHubSSOAPIKey{},
	}, nil
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
