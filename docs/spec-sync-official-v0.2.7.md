---
title: '同步官方 Sub2API v0.2.7 并保留 LinkCode 个性化'
type: 'chore'
created: '2026-09-19'
status: 'done'
baseline_commit: 'a9a320fef'
context:
  - 'linkcode/CLAUDE.md'
  - 'linkcode/DEV_GUIDE.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** 当前 LinkCode 基于官方 v0.2.6 并带有 Codex ticket、Codex 指纹、智力检测、模型广场、SSO、支付和 OVH 部署等二开。官方 v0.2.7 已发布较大增量，直接覆盖会误删现有个性化能力；官方标签自身还保留了过期的 `VERSION` 值，不能原样照搬。

**Approach:** 以官方 `v0.2.7`（提交 `aea725f2ea644d5592d0bbb1d63b607efa7e200a`）为输入，在隔离分支逐文件吸收官方增量；冲突区域以当前 LinkCode 行为为基线，按函数/接口补入官方新增能力。保留现有 Codex ticket、智力检测、模型广场、SSO、支付、图片/对象存储、审计、部署和题库逻辑，并把运行版本明确设置为 `0.2.7`。

## Boundaries & Constraints

**Always:** 保留现有个性化功能和默认行为；官方新增 Seedance、插件宿主服务/只读状态通道、Gemini thinking 变体、CN Coding Plan 403 限时暂停、Responses→Anthropic 工具 schema 兼容和 DeepSeek reasoning_content 回退修复；官方删除的 Codex ticket 文件不得删除；所有冲突、取舍、测试结果和未合入项写入同步记录。

**Ask First:** 若发现需要数据库迁移、生产配置变更、OVH/新对话站发布、删除既有 API 或改变生产默认开关，先停止并请求单独确认；本次默认只做源码同步、测试和记录，不发布。

**Never:** 不使用 fork 私有提交；不 reset、clean、强制覆盖工作树；不把官方 v0.2.7 过期的 `VERSION=0.2.5` 带入运行版本；不删除 LinkCode 的 Codex ticket/指纹、智力检测、题库样本、SSO、支付、图片转存、审计或部署定制；不执行生产数据库迁移或部署。

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|-----------------------------|----------------|
| 官方增量无冲突 | 官方 v0.2.6→v0.2.7 文件 | 吸收官方实现，保留现有测试和接口 | `git diff --check` 失败则停止 |
| Codex ticket 被官方删除 | 官方删除 ticket/生命周期/设置测试文件 | 保留 LinkCode 文件和行为，不跟随删除 | 编译/测试验证 ticket 仍可用 |
| 网关/设置/插件文件冲突 | 本地有 compact、指纹、审计、智检和 UI 定制 | 手工按函数合并，保留本地行为并补官方能力 | 记录每个取舍，发现行为不确定则停止 |
| 官方版本字段过期 | `v0.2.7` 标签内 VERSION 仍为旧值 | 显示版本改为 `0.2.7`，提交记录说明上游异常 | 版本检查不一致则停止 |
| 需要迁移或部署 | 官方增量出现迁移或发布要求 | 本次不执行，单独列为待确认项 | 不触碰生产环境 |

</frozen-after-approval>

## Code Map

- `backend/internal/service/` -- 官方网关、Seedance、插件宿主、Gemini、限流和兼容层；同时保留现有 Codex ticket/指纹/智力检测逻辑。
- `backend/internal/handler/`、`backend/internal/server/routes/` -- 官方新增 Seedance/API 路由和插件状态通道，与现有 LinkCode 路由合并。
- `backend/pkg/pluginapi/`、`backend/internal/repository/plugin_kv_store.go` -- 官方插件宿主服务和 KV/状态桥接接口。
- `frontend/src/` -- 官方 Seedance/插件状态/账号 UI 变更，与现有模型广场、智力检测、Codex 设置页面合并。
- `backend/cmd/server/wire.go`、`wire_gen.go` -- 依赖注入与生成代码，冲突后重新生成/校验。
- `backend/cmd/server/VERSION` -- 保持运行版本显示为 `0.2.7`。
- `docs/同步开源版本0.2.6并兼容二开记录-20260918.md` -- 追加 v0.2.7 同步过程、冲突取舍、验证和未发布项。

## Tasks & Acceptance

**Execution:**
- [x] 创建隔离工作分支，记录当前提交和官方标签提交，生成官方增量文件清单。
- [x] 吸收官方 v0.2.7 后端、插件宿主、Seedance、Gemini、限流和协议兼容改动；保留并适配现有 LinkCode 代码。
- [x] 保留官方删除的 Codex ticket 及相关测试，解决其与新网关/设置/账号接口的编译和行为冲突。
- [x] 合并前端账号、插件、顶栏和设置变更，保留智力检测、模型广场、Codex 和 LinkCode 个性化 UI。
- [x] 修正运行版本为 `0.2.7`，更新同步记录并列出所有未合入/需确认项。

**Acceptance Criteria:**
- Given 当前 LinkCode 工作树，when 同步完成，then 所有官方 v0.2.7 新增能力已合入或明确记录为未合入，且现有个性化功能没有被删除。
- Given 官方 v0.2.7 删除 Codex ticket 文件，when 构建和测试，then LinkCode 的 ticket 捕获、注入、fail-closed、审计脱敏和管理端状态仍可编译并通过对应测试。
- Given 官方标签版本字段错误，when 查看运行版本，then 代码与文档明确显示 `0.2.7` 而不是 `0.2.5`。
- Given 本次用户未要求发布，when 同步完成，then 没有执行数据库迁移、OVH 发布、新对话站发布或 DNS 变更。

## Spec Change Log

- Review finding: official v0.2.7 removed the existing WebSocket ticket injection and compact gate call-site arguments. Amendment: restored `applyOpenAICodexTicket` on WS output and passed the real compact flag through production scheduler paths. Known-bad state avoided: ticketless WebSocket/compact requests bypassing LinkCode fail-closed behavior. KEEP: preserve the existing Codex ticket lifecycle and model-specific outbound mapping.
- Review finding: Seedance capability, content-type validation, sticky transport, and disabled-plugin/inactive-account guards were incomplete at the merge boundary. Amendment: added the capability/HTTP checks, selected ordinary HTTP transport for Seedance, and restricted sensitive plugin host services to enabled bindings and active accounts. Known-bad state avoided: malformed Seedance requests and stale credentials entering new paths. KEEP: official Seedance API surface and plugin host protocol remain available.
- Review finding: existing LinkCode image proxy, API-key manifest snapshot, and Codex image SSE behavior needed local compatibility preservation. Amendment: restored manifest snapshot fields, kept proxy fail-closed behavior while making delivery fail-open when no storage is configured, and kept normalized `image_edit.*` events. Known-bad state avoided: upstream URL leakage, lost fixed-account manifest settings, and duplicate/raw image SSE. KEEP: existing image/object-storage and Codex client contracts.

## Design Notes

官方 v0.2.7 的最大风险不是新增文件，而是其清理/重构会删除 v0.2.6 的 Codex ticket 代码。该部分属于 LinkCode 已验证的个性化能力，必须采用“保留本地实现 + 补齐官方新增接口”的策略，而不是以官方删除结果为准。

## Verification

**Commands:**
- `git diff --check` -- expected: no whitespace errors.
- `go test -tags=unit ./...` -- expected: backend unit tests pass or every environment limitation is recorded.
- `pnpm --dir frontend typecheck` -- expected: frontend typecheck passes.
- `pnpm --dir frontend build` -- expected: production frontend build passes.
- `git diff --name-only 49a39b6dc1abed30fd227611e8af1108bc427610..HEAD` and official delta review -- expected: no unreviewed deletion of LinkCode custom files.

## Suggested Review Order

**Gateway entry points**

- Seedance and root aliases share one authenticated, allowlisted middleware chain.
  [`gateway.go:372`](../backend/internal/server/routes/gateway.go#L372)

- Seedance validates protocol access before entering billing and upstream forwarding.
  [`seedance.go:15`](../backend/internal/handler/seedance.go#L15)

- Native Ark task forwarding preserves model mapping, task ownership, and usage billing.
  [`seedance.go:75`](../backend/internal/service/seedance.go#L75)

**Plugin host boundary**

- Plugin runtime wiring adds Redis KV and account-directory capabilities through the host.
  [`plugin_manager.go:65`](../backend/internal/service/plugin_manager.go#L65)

- Host services enforce plugin namespaces, value limits, and capability-bound account access.
  [`plugin_host_services.go:64`](../backend/internal/service/plugin_host_services.go#L64)

- Redis persistence keeps plugin state across restarts and replicas.
  [`plugin_kv_store.go:25`](../backend/internal/repository/plugin_kv_store.go#L25)

**Codex preservation**

- WebSocket output still injects the existing Codex ticket before upstream connection.
  [`openai_ws_forwarder_payload.go:144`](../backend/internal/service/openai_ws_forwarder_payload.go#L144)

- Compact runtime blocks use the actual outbound model and compact requirement.
  [`openai_account_runtime_block_fastpath.go:557`](../backend/internal/service/openai_account_runtime_block_fastpath.go#L557)

**Compatibility fixes**

- Gemini thinking variants normalize provider-specific model suffixes.
  [`antigravity_gemini_thinking_variant.go:10`](../backend/internal/service/antigravity_gemini_thinking_variant.go#L10)

- Kimi Coding Plan quota exhaustion becomes a windowed pause instead of permanent disable.
  [`ratelimit_cn_providers.go:40`](../backend/internal/service/ratelimit_cn_providers.go#L40)

**Verification and UI**

- Seedance tests cover forwarding, task status/delete, capability, validation, and errors.
  [`seedance_test.go:23`](../backend/internal/service/seedance_test.go#L23)

- Plugin management UI exposes testing, enable/disable, configuration, and status behavior.
  [`PluginsView.vue:245`](../frontend/src/views/admin/PluginsView.vue#L245)

- Runtime version is explicitly corrected to the requested v0.2.7.
  [`VERSION:1`](../backend/cmd/server/VERSION#L1)
