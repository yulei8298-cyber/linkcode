import { keysAPI } from './keys'
import { userGroupsAPI } from './groups'
import type { ApiKey, Group } from '@/types'

export const INFINITE_CANVAS_KEY_NAME = '无限画布 · GPT 生图'
export const INFINITE_CANVAS_GROUP_NAME = 'GPT-生图专用分组'
export const INFINITE_CANVAS_MODEL = 'gpt-image-2'
export const INFINITE_CANVAS_BASE_URL = 'https://api-fast.linkcode.site/v1'

export interface InfiniteCanvasConfig {
  provider: 'linkcode'
  baseUrl: string
  apiKey: string
  apiKeyId: number
  groupId: number
  groupName: string
  model: typeof INFINITE_CANVAS_MODEL
}

function isActiveKey(key: ApiKey): boolean {
  return key.status === 'active' && (!key.expires_at || new Date(key.expires_at).getTime() > Date.now())
}

function isImageGroup(group: Group): boolean {
  return group.platform === 'openai' && group.status === 'active' && group.allow_image_generation
}

function resolveCanvasGroup(groups: Group[]): Group | null {
  return groups.find((group) => group.name.trim() === INFINITE_CANVAS_GROUP_NAME && isImageGroup(group)) || null
}

/**
 * 进入画布时保持专用 Key 幂等：已有活动 Key 直接复用，避免每次进入都生成新 Key。
 */
export async function ensureInfiniteCanvasConfig(): Promise<InfiniteCanvasConfig> {
  const [groups, keyPage] = await Promise.all([
    userGroupsAPI.getAvailable(),
    keysAPI.list(1, 100, { search: INFINITE_CANVAS_KEY_NAME, status: 'active' }),
  ])

  const group = resolveCanvasGroup(groups)
  if (!group) {
    throw new Error(`未找到可用于生图的“${INFINITE_CANVAS_GROUP_NAME}”，请联系管理员启用 GPT 生图分组。`)
  }

  const existing = keyPage.items.find((key) =>
    isActiveKey(key) && key.group_id === group.id && key.key.trim()
  )
  const key = existing || await keysAPI.create(INFINITE_CANVAS_KEY_NAME, group.id)
  if (!key.key?.trim()) {
    throw new Error('画布专用 Key 未返回，请到 API 密钥页面检查该 Key。')
  }

  return {
    provider: 'linkcode',
    baseUrl: INFINITE_CANVAS_BASE_URL,
    apiKey: key.key,
    apiKeyId: key.id,
    groupId: group.id,
    groupName: group.name,
    model: INFINITE_CANVAS_MODEL,
  }
}

export interface GenerateCanvasImageRequest {
  prompt: string
  size: string
  quality: 'auto' | 'low' | 'medium' | 'high'
  count: number
  background: 'auto' | 'transparent' | 'opaque'
  outputFormat: 'png' | 'webp' | 'jpeg'
}

export interface GeneratedCanvasImage {
  url: string
  revisedPrompt?: string
}

async function readError(response: Response): Promise<Error> {
  try {
    const body = await response.json()
    const message = body?.error?.message || body?.message || response.statusText
    const error = new Error(message)
    ;(error as Error & { code?: string | number }).code = body?.error?.code || response.status
    return error
  } catch {
    return new Error(response.statusText || `HTTP ${response.status}`)
  }
}

export async function generateCanvasImage(
  config: InfiniteCanvasConfig,
  request: GenerateCanvasImageRequest,
): Promise<GeneratedCanvasImage[]> {
  const endpoint = `${config.baseUrl.replace(/\/+$/, '')}/images/generations`
  const response = await fetch(endpoint, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${config.apiKey}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      model: INFINITE_CANVAS_MODEL,
      prompt: request.prompt,
      size: request.size,
      quality: request.quality,
      n: request.count,
      background: request.background,
      output_format: request.outputFormat,
    }),
  })

  if (!response.ok) throw await readError(response)
  const body = await response.json() as { data?: Array<{ url?: string; revised_prompt?: string }> }
  const images = (body.data || [])
    .filter((item) => item.url?.trim())
    .map((item) => ({ url: item.url as string, revisedPrompt: item.revised_prompt }))
  if (!images.length) {
    throw new Error('中转站未返回图片 URL，画布只支持 URL 类型的生图结果。')
  }
  return images
}
