package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// openAIModelRoutingPolicy reserves routing accounts within one OpenAI group.
// A matched model prefers its configured accounts and may fall back to other
// non-reserved accounts; a non-matching model must not select an account
// reserved by any routing rule.
type openAIModelRoutingPolicy struct {
	matchedAccounts  map[int64]struct{}
	reservedAccounts map[int64]struct{}
}

func (p openAIModelRoutingPolicy) hasMatchedAccounts() bool {
	return len(p.matchedAccounts) > 0
}

func (p openAIModelRoutingPolicy) isPreferred(accountID int64) bool {
	_, ok := p.matchedAccounts[accountID]
	return ok
}

func (p openAIModelRoutingPolicy) allows(accountID int64) bool {
	if p.isPreferred(accountID) {
		return true
	}
	_, reserved := p.reservedAccounts[accountID]
	return !reserved
}

func (p openAIModelRoutingPolicy) rejectionReason() string {
	if len(p.matchedAccounts) > 0 {
		return "model_routing_target_only"
	}
	return "model_routing_reserved"
}

func (s *OpenAIGatewayService) openAIModelRoutingForRequest(ctx context.Context, groupID *int64, platform, requestedModel string) openAIModelRoutingPolicy {
	if groupID == nil || requestedModel == "" || normalizeOpenAICompatiblePlatform(platform) != PlatformOpenAI {
		return openAIModelRoutingPolicy{}
	}

	var group *Group
	if current, ok := ctx.Value(ctxkey.Group).(*Group); ok && current != nil && current.ID == *groupID {
		group = current
	} else if s != nil && s.schedulerSnapshot != nil {
		group, _ = s.schedulerSnapshot.GetGroupByID(ctx, *groupID)
	} else if s != nil && s.channelService != nil && s.channelService.groupRepo != nil {
		group, _ = s.channelService.groupRepo.GetByIDLite(ctx, *groupID)
	}
	if group == nil || group.Platform != PlatformOpenAI || !group.ModelRoutingEnabled || len(group.ModelRouting) == 0 {
		return openAIModelRoutingPolicy{}
	}

	policy := openAIModelRoutingPolicy{reservedAccounts: make(map[int64]struct{})}
	for _, accountIDs := range group.ModelRouting {
		for _, accountID := range accountIDs {
			if accountID > 0 {
				policy.reservedAccounts[accountID] = struct{}{}
			}
		}
	}
	if matched := group.GetRoutingAccountIDs(requestedModel); len(matched) > 0 {
		policy.matchedAccounts = make(map[int64]struct{}, len(matched))
		for _, accountID := range matched {
			if accountID > 0 {
				policy.matchedAccounts[accountID] = struct{}{}
			}
		}
	}
	return policy
}
