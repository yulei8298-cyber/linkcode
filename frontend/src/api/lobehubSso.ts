import { apiClient } from './client'

export interface LobeHubSSOAuthorizeResponse {
  redirect_url: string
  expires_at: string
}

async function authorize(returnTo?: string): Promise<LobeHubSSOAuthorizeResponse> {
  const { data } = await apiClient.post<LobeHubSSOAuthorizeResponse>('/lobehub-sso/authorize', {
    return_to: returnTo || '/',
  })
  return data
}

export const lobeHubSSOAPI = {
  authorize,
}
