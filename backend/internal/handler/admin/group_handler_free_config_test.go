//go:build unit

package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type groupFreeConfigAdminStub struct {
	service.AdminService
	created *service.CreateGroupInput
	updated *service.UpdateGroupInput
}

func (s *groupFreeConfigAdminStub) CreateGroup(_ context.Context, input *service.CreateGroupInput) (*service.Group, error) {
	s.created = input
	return &service.Group{
		ID:                10,
		Name:              input.Name,
		SubscriptionType:  input.SubscriptionType,
		IsHidden:          input.IsHidden,
		IsFree:            input.IsFree,
		DailyFreeLimitUSD: input.DailyFreeLimitUSD,
		ChatStationOnly:   input.ChatStationOnly,
	}, nil
}

func (s *groupFreeConfigAdminStub) UpdateGroup(_ context.Context, id int64, input *service.UpdateGroupInput) (*service.Group, error) {
	s.updated = input
	return &service.Group{ID: id, Name: input.Name}, nil
}

func TestGroupHandlerCreateMapsChatStationFreeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &groupFreeConfigAdminStub{AdminService: newStubAdminService()}
	handler := NewGroupHandler(stub, nil, nil)
	router := gin.New()
	router.POST("/groups", handler.Create)

	body := []byte(`{"name":"chat-free","rate_multiplier":1,"subscription_type":"standard","is_hidden":true,"is_free":true,"daily_free_limit_usd":0.5,"chat_station_only":true}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, stub.created)
	require.True(t, stub.created.IsHidden)
	require.True(t, stub.created.IsFree)
	require.NotNil(t, stub.created.DailyFreeLimitUSD)
	require.InDelta(t, 0.5, *stub.created.DailyFreeLimitUSD, 1e-12)
	require.True(t, stub.created.ChatStationOnly)
}

func TestGroupHandlerUpdateMapsExplicitFreeDisableAndNullLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &groupFreeConfigAdminStub{AdminService: newStubAdminService()}
	handler := NewGroupHandler(stub, nil, nil)
	router := gin.New()
	router.PUT("/groups/:id", handler.Update)

	body := []byte(`{"is_hidden":false,"is_free":false,"daily_free_limit_usd":null,"chat_station_only":false}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/groups/10", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, stub.updated)
	require.NotNil(t, stub.updated.IsHidden)
	require.False(t, *stub.updated.IsHidden)
	require.NotNil(t, stub.updated.IsFree)
	require.False(t, *stub.updated.IsFree)
	require.NotNil(t, stub.updated.DailyFreeLimitUSD)
	require.Zero(t, *stub.updated.DailyFreeLimitUSD)
	require.NotNil(t, stub.updated.ChatStationOnly)
	require.False(t, *stub.updated.ChatStationOnly)
}
