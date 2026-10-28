// Package registry 汇总全部视频协议实现，是网关查找 Provider 的唯一入口。
package registry

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service/video"
	"github.com/Wei-Shaw/sub2api/internal/service/video/cn"
	"github.com/Wei-Shaw/sub2api/internal/service/video/intl"
)

// providers 的顺序即管理后台协议下拉框的展示顺序。
var providers = []video.Provider{
	cn.Ark{},
	cn.Kling{},
	cn.MiniMax{},
	cn.DashScope{},
	cn.Vidu{},
	intl.Veo{},
	intl.Runway{},
	intl.Luma{},
	intl.PixVerse{},
	intl.OpenAIVideos{},
	intl.NewAPI{},
}

var index = func() map[video.Protocol]video.Provider {
	m := make(map[video.Protocol]video.Provider, len(providers))
	for _, p := range providers {
		m[p.Info().Protocol] = p
	}
	return m
}()

// Lookup 返回协议实现；未知协议返回 false。
func Lookup(protocol string) (video.Provider, bool) {
	p, ok := index[video.Protocol(protocol)]
	return p, ok
}

// nativeIndex 以原生透传前缀的协议段（/video/{segment}）为键，与协议 ID 不一定相同
// （OpenAI 兼容协议 ID 为 openai_videos，对外前缀是 /video/openai）。
var nativeIndex = func() map[string]video.Provider {
	m := make(map[string]video.Provider, len(providers))
	for _, p := range providers {
		m[strings.TrimPrefix(p.Info().NativePrefix, nativePrefixRoot)] = p
	}
	return m
}()

const nativePrefixRoot = "/video/"

// LookupNative 按原生透传前缀中的协议段查找协议实现。
func LookupNative(segment string) (video.Provider, bool) {
	p, ok := nativeIndex[segment]
	return p, ok
}

// Infos 返回全部协议的展示信息，按展示顺序。
func Infos() []video.Info {
	out := make([]video.Info, 0, len(providers))
	for _, p := range providers {
		out = append(out, p.Info())
	}
	return out
}
