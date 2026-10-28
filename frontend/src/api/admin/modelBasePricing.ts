import { apiClient } from '../client'

/** 某分辨率档的视频价格（USD）。per_video 的键为时长秒数，"*" 表示任意时长。 */
export interface VideoTierPrice {
  per_second?: number | null
  per_second_with_audio?: number | null
  per_video?: Record<string, number>
}

/** 视频模型计费规则：按秒 / 按次 / 按 token（对应后端 VideoPricingSpec）。 */
export interface VideoPricingSpec {
  unit: 'per_second' | 'per_video' | 'token'
  prices?: Record<string, VideoTierPrice>
  token_price_per_million?: number | null
  default_resolution?: string
  default_duration_seconds?: number
  min_duration_seconds?: number
  max_duration_seconds?: number
}

export interface AdminModelPrice {
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_input_price?: number | null
  image_output_price?: number | null
  per_image_price?: number | null
  video?: VideoPricingSpec | null
}

export interface ModelBasePricingResponse {
  model: string
  edited: boolean
  override: AdminModelPrice
  catalog: {
    input_cost_per_token?: number
    output_cost_per_token?: number
    cache_creation_input_token_cost?: number
    cache_creation_input_token_cost_above_1hr?: number
    cache_read_input_token_cost?: number
    input_cost_per_image_token?: number
    output_cost_per_image_token?: number
    output_cost_per_image?: number
  } | null
  /** 内置视频参考价（仅视频模型返回）。 */
  video_default?: VideoPricingSpec
}

export async function getModelBasePricing(model: string): Promise<ModelBasePricingResponse> {
  const { data } = await apiClient.get<ModelBasePricingResponse>('/admin/channels/pricing/base', { params: { model } })
  return data
}

export async function updateModelBasePricing(model: string, price: AdminModelPrice | null): Promise<void> {
  await apiClient.put('/admin/channels/pricing/base', { model, price })
}

export default { getModelBasePricing, updateModelBasePricing }
