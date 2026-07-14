# LobeHub 对话站 SSO 与首聊密钥集成指南

本文档说明 LinkCode 与 LobeHub 的当前集成协议。SSO 只负责登录和用户绑定；Provider API Key 在用户第一次对话且未配置密钥时，由 LinkCode 按分组字段动态创建。

## 功能效果

- LinkCode 已登录用户可从公开门户自动进入 LobeHub 并完成登录。
- SSO exchange 只返回用户身份和 API 根地址，不再创建密钥或订阅。
- OpenAI 或 Anthropic Provider 没有密钥时，LobeHub 服务端以共享密钥和 LinkCode 用户 ID 发起首次请求。
- LinkCode 从同平台的隐藏、免费、仅对话站分组中选组，创建专用密钥，并在同次回答的响应头中返回。
- LobeHub 服务端使用 KeyVault 加密保存密钥；保存失败时本次请求失败，不伪装成已完成。

## 工作流程

### SSO 登录

1. 用户在 LinkCode 公开门户点击「对话站」。
2. LinkCode 生成 Redis 短时一次性 code，浏览器跳转到 LobeHub callback。
3. LobeHub 服务端携带 code 和共享密钥请求 `POST /api/v1/lobehub-sso/exchange`。
4. LinkCode 返回用户信息和 `api_base_url`，`keys` 为空数组。
5. LobeHub 创建或登录本地用户，并在 Better Auth `accounts` 中保存 LinkCode 用户 ID。

### 首次无密钥对话

1. LobeHub 服务端发现 OpenAI/Anthropic Provider 没有 API Key。
2. LobeHub 删除占位 Authorization，添加以下请求头：

```text
X-LinkCode-Chat-Station-Secret: <shared_secret>
X-LinkCode-Chat-Station-User-ID: <linkcode_user_id>
```

3. LinkCode 选择满足 `active + standard + is_hidden + is_free + chat_station_only` 且每日额度大于 0 的同平台分组，按 `sort_order,id` 排序。
4. LinkCode 复用或创建名为 `LobeHub Chat Station` 的专用密钥，继续完成同一次请求。
5. LinkCode 在响应头返回：

```text
X-LinkCode-Chat-Station-API-Key: sk-...
```

6. LobeHub 服务端加密保存密钥，之后的请求使用已保存密钥。共享密钥和自动密钥均不下发浏览器。

## 前置条件

- LinkCode 后端、PostgreSQL 和 Redis 正常运行。
- LinkCode 公开门户已配置 `chat_station_url`。
- LobeHub 后端可访问 LinkCode exchange 和网关地址。
- 管理员至少配置一个符合条件的 OpenAI 或 Anthropic 分组，并绑定可调度上游账号。
- LobeHub 已配置用于加密 Provider 密钥的 `KEY_VAULTS_SECRET`。

## LinkCode 后端配置

```yaml
lobehub_sso:
  enabled: true
  shared_secret: "replace-with-a-strong-random-secret-at-least-32-bytes"
  lobehub_base_url: "https://chat.example.com"
  callback_path: "/linkcode/sso/callback"
  code_ttl_seconds: 120
  api_base_url: "https://linkcode.example.com"
```

| 字段 | 说明 |
| --- | --- |
| `enabled` | 是否启用 LinkCode -> LobeHub SSO。 |
| `shared_secret` | SSO exchange 和首聊请求共用的服务端密钥，至少 32 字节。 |
| `lobehub_base_url` | LobeHub 对话站根地址。 |
| `callback_path` | LobeHub 接收 SSO code 的路径，默认 `/linkcode/sso/callback`。 |
| `code_ttl_seconds` | SSO code 有效期，允许 30-600 秒。 |
| `api_base_url` | LinkCode 网关根地址，不要带 `/api/v1`。 |

`auto_create_api_keys` 和 `api_key_name_prefix` 仅作为旧配置兼容字段保留，当前 exchange 不再使用它们预创建密钥。

## 分组配置

对话站免费候选分组必须同时满足：

- 状态为启用。
- 订阅类型为标准/余额模式。
- 开启「对用户隐藏」。
- 开启「每日免费」并设置正数额度。
- 开启「仅对话站请求」。
- 平台为 `openai` 或 `anthropic`。

普通用户不能在分组或 API 密钥页看到隐藏分组，也不能创建、改绑、修改或删除其中的密钥。管理员仍可查看和管理。

## LobeHub 环境变量

```env
LINKCODE_SSO_ENABLED=1
LINKCODE_BASE_URL=https://linkcode.example.com
LINKCODE_PROVIDER_API_BASE_URL=https://linkcode.example.com
LINKCODE_SSO_SHARED_SECRET=<与 lobehub_sso.shared_secret 完全一致>
```

可选覆盖：

```env
LINKCODE_LOGIN_URL=https://linkcode.example.com/login
LINKCODE_SSO_EXCHANGE_URL=https://linkcode.example.com/api/v1/lobehub-sso/exchange
```

OpenAI 会使用 `${LINKCODE_PROVIDER_API_BASE_URL}/v1`，Anthropic 使用网关根地址。共享密钥只会发送到 LinkCode 官方 HTTPS 域名或明确配置的内网地址；公网 HTTP 地址失败关闭。

## exchange 响应示例

```json
{
  "user": {
    "id": 123,
    "email": "user@example.com",
    "username": "user",
    "avatar_url": "https://example.com/avatar.png"
  },
  "api_base_url": "https://linkcode.example.com",
  "keys": []
}
```

## 错误契约

| 场景 | HTTP | 提示 |
| --- | ---: | --- |
| 没有可用候选分组 | 401 | 当前没有可用的对话站免费分组，请前往设置密钥 |
| 密钥所属分组已删除 | 401 | API 密钥无效或所属分组不存在 |
| 非对话站使用受限分组 | 403 | 当前分组仅允许通过对话站调用 |
| 当日免费额度用完 | 429 | 当前每日免费额度已用完，请明天再使用 |
| LobeHub 保存返回密钥失败 | 请求失败 | 不返回伪成功结果，保留后端错误日志 |

## 上线检查

1. 备份 `groups` 和 `user_subscriptions`，或创建完整数据库快照。
2. 确认旧 `OpenAI-chat` 自动订阅的 `notes` 为 `auto assigned by LobeHub SSO`。
3. 如该分组存在人工或付费订阅，迁移会保留原订阅分组，只失效旧 SSO 自动订阅；管理员需另行配置新免费分组。
4. 验证 OpenAI 和 Claude 各一次首聊无密钥、第二次已保存密钥、额度耗尽和外部调用拒绝。
5. 数据迁移不会随 Git 回退自动恢复；需要回滚时同时回复数据库快照。

## 安全要求

- 不要把 `lobehub_sso.shared_secret` 或 `LINKCODE_SSO_SHARED_SECRET` 提交到 Git。
- 不接受客户端自报的对话站来源，只信任服务端共享密钥。
- 不向浏览器暴露共享密钥、LinkCode 用户 ID 请求头或自动密钥。
