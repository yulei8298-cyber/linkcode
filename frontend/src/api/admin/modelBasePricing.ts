import { apiClient } from '../client'

export interface AdminModelPrice {
  input_price?: number | null
  output_price?: number | null
  cache_write_price?: number | null
  cache_write_1h_price?: number | null
  cache_read_price?: number | null
  image_input_price?: number | null
  image_output_price?: number | null
  per_image_price?: number | null
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
}

export async function getModelBasePricing(model: string): Promise<ModelBasePricingResponse> {
  const { data } = await apiClient.get<ModelBasePricingResponse>('/admin/channels/pricing/base', { params: { model } })
  return data
}

export async function updateModelBasePricing(model: string, price: AdminModelPrice | null): Promise<void> {
  await apiClient.put('/admin/channels/pricing/base', { model, price })
}

export default { getModelBasePricing, updateModelBasePricing }
