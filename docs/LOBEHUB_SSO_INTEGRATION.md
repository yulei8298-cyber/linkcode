# LobeHub 对话站 SSO 集成指南

本文档只说明 LinkCode 侧需要做什么，适用于把 LinkCode 作为统一身份源和 API Key 来源，接入一个基于 LobeHub 的对话站。

## 功能效果

开启后可以实现：

- 用户已登录 LinkCode 时，点击公开门户里的「对话站」入口，会自动进入 LobeHub 并完成登录。
- 用户未登录 LinkCode 时，也可以直接进入 LobeHub，不会先被 LinkCode 登录页拦住。
- LobeHub 需要用户登录时，可以跳回 LinkCode 登录；登录完成后继续回到 LobeHub。
- 首次 SSO 时，LinkCode 可以自动创建或复用 Claude、GPT、Gemini 三类用户 API Key，并返回给 LobeHub。
- 如果 LinkCode 没有某个平台的可用渠道，则不会生成该平台的 key。

## 工作流程

SSO 使用短时一次性 code：

1. 用户在 LinkCode 公开门户点击「对话站」。
2. 如果用户已登录，LinkCode 创建一个一次性 SSO code，并返回 LobeHub callback URL。
3. 浏览器跳转到 LobeHub callback。
4. LobeHub 后端携带 code 和共享密钥请求 LinkCode 的 exchange 接口。
5. LinkCode 校验共享密钥，消费一次性 code，返回用户邮箱、用户名、头像，以及可选的 provider API Key。
6. LobeHub 用邮箱创建或登录本地用户，并保存 LinkCode 返回的 provider key。

SSO code 存在 Redis 中，并通过 `GETDEL` 消费，因此只能使用一次。

## 前置条件

- LinkCode 后端、前端、PostgreSQL、Redis 正常运行。
- LinkCode 公开门户已启用，并配置了 `chat_station_url`。
- LobeHub 后端可以访问 LinkCode 的 `/api/v1/lobehub-sso/exchange` 接口。
- 如果要自动生成 provider key，LinkCode 中需要存在对应平台的 active channel，并且用户有对应的固定分组：`OpenAI-chat`、`Anthropic-chat`、`Gemini-chat`。

## LinkCode 后端配置

在 LinkCode 配置文件中增加：

```yaml
lobehub_sso:
  enabled: true
  shared_secret: "replace-with-a-strong-random-secret-at-least-32-bytes"
  lobehub_base_url: "https://chat.example.com"
  callback_path: "/linkcode/sso/callback"
  code_ttl_seconds: 120
  api_base_url: "https://linkcode.example.com"
  auto_create_api_keys: true
  api_key_name_prefix: "LobeHub"
```

字段说明：

| 字段 | 是否必填 | 默认值 | 说明 |
|---|---:|---|---|
| `enabled` | 是 | `false` | 是否启用 LinkCode -> LobeHub SSO。 |
| `shared_secret` | 启用时必填 | 空 | LobeHub 调用 exchange 接口时使用的共享密钥，至少 32 字节。 |
| `lobehub_base_url` | 启用时必填 | 空 | LobeHub 对话站根地址，例如 `https://chat.example.com`。 |
| `callback_path` | 否 | `/linkcode/sso/callback` | LobeHub 接收 SSO code 的 callback path。 |
| `code_ttl_seconds` | 否 | `120` | SSO code 有效期，允许范围 30 到 600 秒。 |
| `api_base_url` | 建议填写 | 空 | LinkCode 网关根地址，不要带 `/api/v1`。 |
| `auto_create_api_keys` | 否 | `true` | SSO exchange 时是否自动创建或复用 provider API Key。 |
| `api_key_name_prefix` | 否 | `LobeHub` | 自动生成 API Key 的名称前缀。 |

建议用下面的命令生成共享密钥：

```bash
openssl rand -hex 32
```

LobeHub 侧的 `LINKCODE_SSO_SHARED_SECRET` 必须和这里的 `lobehub_sso.shared_secret` 完全一致。

## 公开门户入口配置

在 LinkCode 管理后台中配置公开门户的对话站入口：

```text
chat_station_url = https://chat.example.com
```

行为说明：

- 用户已登录 LinkCode：点击入口后会先调用 `POST /api/v1/lobehub-sso/authorize`，再跳转到 LobeHub callback。
- 用户未登录 LinkCode：点击入口会直接打开 `chat_station_url`。
- 用户从 LobeHub 跳回 LinkCode 登录：登录完成后会进入 `/lobehub-sso/continue`，继续发起 SSO，再返回 LobeHub。

## 自动 API Key 生成规则

当 `auto_create_api_keys=true` 时，LinkCode 会尝试创建或复用以下 key：

| LobeHub provider | LinkCode platform | 固定分组名 | 默认 key 名称 |
|---|---|---|---|
| Claude | Anthropic | `Anthropic-chat` | `LobeHub Claude` |
| GPT | OpenAI | `OpenAI-chat` | `LobeHub GPT` |
| Gemini | Gemini | `Gemini-chat` | `LobeHub Gemini` |

实际名称会使用 `api_key_name_prefix` 作为前缀。

每个平台都必须同时满足下面两个条件，才会返回 key：

- LinkCode 中存在该平台的 active channel。
- 当前用户有该平台对应的固定分组，且该分组为 active。

如果已经存在同名、同 group 的 active API Key，LinkCode 会复用它，不会重复创建。
如果缺少某个平台的固定分组，LinkCode 会跳过该 provider，不会退回到其它分组。

## 新增接口

### `POST /api/v1/lobehub-sso/authorize`

用户已登录后调用，用来创建短时一次性 SSO code。

请求：

```json
{
  "return_to": "/"
}
```

响应：

```json
{
  "redirect_url": "https://chat.example.com/linkcode/sso/callback?code=...",
  "expires_at": "2026-06-20T12:00:00Z"
}
```

### `POST /api/v1/lobehub-sso/exchange`

供 LobeHub 后端调用，用一次性 code 换取用户信息和 provider key。

请求头：

```text
X-LobeHub-SSO-Secret: <shared_secret>
```

请求体：

```json
{
  "code": "one-time-sso-code"
}
```

响应数据：

```json
{
  "user": {
    "id": 123,
    "email": "user@example.com",
    "username": "user",
    "avatar_url": "https://example.com/avatar.png"
  },
  "api_base_url": "https://linkcode.example.com",
  "keys": [
    {
      "provider": "gpt",
      "platform": "openai",
      "key": "sk-...",
      "group_id": 1
    }
  ]
}
```

外层响应格式仍使用 LinkCode 统一 API 响应结构。

## LobeHub 侧配置摘要

本仓库只实现 LinkCode 侧。LobeHub 侧需要配置：

```env
LINKCODE_SSO_ENABLED=1
LINKCODE_BASE_URL=https://linkcode.example.com
LINKCODE_SSO_SHARED_SECRET=<和 lobehub_sso.shared_secret 完全一致>
```

如果默认路径不符合你的部署，可以额外配置：

```env
LINKCODE_LOGIN_URL=https://linkcode.example.com/login
LINKCODE_SSO_EXCHANGE_URL=https://linkcode.example.com/api/v1/lobehub-sso/exchange
```

LobeHub 还需要正确配置自己的 `APP_URL`、数据库，以及用于加密保存 provider key 的 `KEY_VAULTS_SECRET`。

## 上线检查清单

1. LinkCode 后端配置 `lobehub_sso` 并重启。
2. LinkCode 前端部署最新代码。
3. LinkCode 后台配置 `chat_station_url`。
4. LobeHub 配置 `LINKCODE_SSO_ENABLED=1` 和同一个 `LINKCODE_SSO_SHARED_SECRET`。
5. 确认 LinkCode 中 Claude、GPT、Gemini 对应渠道和固定分组状态正确：`Anthropic-chat`、`OpenAI-chat`、`Gemini-chat`。
6. 使用普通用户验证已登录点击入口、未登录直接进入、LobeHub 触发登录、provider key 自动写入。

## 验证方式

| 场景 | 预期结果 |
|---|---|
| 已登录 LinkCode 后点击「对话站」 | 跳到 LobeHub 并自动登录。 |
| 未登录 LinkCode 时点击「对话站」 | 直接打开 LobeHub，不先要求 LinkCode 登录。 |
| 未登录状态在 LobeHub 开始对话 | LobeHub 提示需要登录，并跳转到 LinkCode 登录。 |
| LinkCode 登录完成 | 自动继续 SSO，并回到 LobeHub。 |
| 用户有 active channel 和固定 active group | 自动创建或复用对应 provider API Key。 |
| 某平台没有 active channel | 不生成该平台 key。 |

## 常见问题

| 问题 | 排查方向 |
|---|---|
| LinkCode 启动时报配置错误 | 检查 `shared_secret` 长度、`lobehub_base_url` 是否为合法 HTTP/HTTPS URL、`code_ttl_seconds` 是否在 30 到 600 之间。 |
| LobeHub callback 换码失败 | 检查 LobeHub 的 `LINKCODE_SSO_SHARED_SECRET` 是否和 LinkCode 完全一致；检查 LobeHub 后端是否能访问 exchange 接口。 |
| 刷新 callback 页面后失败 | 正常现象。SSO code 是一次性 code，刷新后不能重复使用。 |
| 没有返回 provider key | 检查对应平台是否有 active channel，以及是否存在对应固定分组：`OpenAI-chat`、`Anthropic-chat`、`Gemini-chat`。 |
| GPT/OpenAI base URL 不正确 | `api_base_url` 应填写 LinkCode 网关根地址，不要填写 `/api/v1`。 |

## 安全建议

- 不要把 `lobehub_sso.shared_secret` 提交到 Git。
- 生产环境建议全站使用 HTTPS。
- Redis 中保存短时 SSO code，应避免暴露到公网。
- 轮换 shared secret 时，需要同时更新 LinkCode 和 LobeHub，并安排维护窗口。
