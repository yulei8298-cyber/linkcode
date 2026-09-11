export interface ModelsListConfig {
  enabled: boolean
  models: string[]
  plaza_enabled?: boolean
  plaza_models?: string[] | null
}

// 上游 0.2.4 将模型配置命名为 ModelAllowlist；保留别名以兼容其调用方。
export type ModelAllowlistConfig = ModelsListConfig

export interface ModelsListItem {
  id: string
  selected: boolean
}

export interface ModelsListState {
  enabled: boolean
  savedModels: string[]
  items: ModelsListItem[]
  plazaEnabled?: boolean
  plazaModels?: string[]
  plazaConfigured?: boolean
  plazaCustomModels?: boolean
}

// 兼容上游重命名后的调用方。
export type ModelAllowlistState = ModelsListState
export type ModelAllowlistItem = ModelsListItem
export type ModelAllowlistAddError = 'empty' | 'invalid_wildcard' | 'duplicate'

export const createModelsListState = (
  config?: Partial<ModelsListConfig> | null,
): ModelsListState => ({
  enabled: config?.enabled ?? false,
  savedModels: normalizeModels(config?.models ?? []),
  items: [],
  plazaEnabled: config?.plaza_enabled ?? true,
  plazaModels: config?.plaza_models ?? [],
  plazaConfigured: config?.plaza_enabled !== undefined || config?.plaza_models !== undefined,
  plazaCustomModels: Array.isArray(config?.plaza_models),
})

export const createModelAllowlistState = (
  config?: Partial<ModelAllowlistConfig> | null,
): ModelAllowlistState => createModelsListState(config)

export const hydrateModelsListState = (
  config: Partial<ModelsListConfig> | null | undefined,
  candidates: string[],
): ModelsListState => {
  const state = createModelsListState(config)
  setModelsListCandidates(state, candidates)
  return state
}

export const hydrateModelAllowlistState = (
  config: Partial<ModelAllowlistConfig> | null | undefined,
  candidates: string[],
): ModelAllowlistState => hydrateModelsListState(config, candidates)

export const setModelsListCandidates = (
  state: ModelsListState,
  candidates: string[],
) => {
  const normalizedCandidates = normalizeModels(candidates)
  const currentSelected = new Set(
    state.items.filter(item => item.selected).map(item => item.id),
  )
  const currentKnown = new Set(state.items.map(item => item.id))
  const savedSelected = new Set(state.savedModels)
  const hasExistingItems = state.items.length > 0
  const selectionOrder = normalizeModels([
    ...state.items.map(item => item.id),
    ...state.savedModels,
    ...normalizedCandidates,
  ])

  state.items = selectionOrder.map(id => {
    const selected = hasExistingItems
      ? currentSelected.has(id)
      : state.savedModels.length > 0
        ? savedSelected.has(id)
        : normalizedCandidates.includes(id)

    return {
      id,
      selected: selected && (currentKnown.has(id) || savedSelected.has(id) || state.savedModels.length === 0),
    }
  })
}

export const setModelAllowlistCandidates = setModelsListCandidates

export const toggleModelAllowlistItem = (
  state: ModelAllowlistState,
  modelID: string,
) => {
  const item = state.items.find(item => item.id === modelID)
  if (item) {
    item.selected = !item.selected
  }
}

export const selectAllModelAllowlistItems = (state: ModelAllowlistState) => {
  state.items.forEach(item => {
    item.selected = true
  })
}

export const invertModelAllowlistSelection = (state: ModelAllowlistState) => {
  state.items.forEach(item => {
    item.selected = !item.selected
  })
}

export const moveModelAllowlistItem = (
  state: ModelAllowlistState,
  fromIndex: number,
  toIndex: number,
) => {
  if (
    fromIndex === toIndex ||
    fromIndex < 0 ||
    toIndex < 0 ||
    fromIndex >= state.items.length ||
    toIndex >= state.items.length
  ) {
    return
  }
  const [item] = state.items.splice(fromIndex, 1)
  state.items.splice(toIndex, 0, item)
}

// 把手工输入的条目追加到白名单末尾，并校验通配符及重复项。
export const addCustomModelAllowlistItem = (
  state: ModelAllowlistState,
  raw: string,
): ModelAllowlistAddError | null => {
  const entry = raw.trim()
  if (!entry) return 'empty'
  if (entry.slice(0, -1).includes('*')) return 'invalid_wildcard'
  if (
    state.items.some(item => item.id.toLowerCase() === entry.toLowerCase()) ||
    state.savedModels.some(model => model.toLowerCase() === entry.toLowerCase())
  ) return 'duplicate'
  state.items.push({ id: entry, selected: true })
  return null
}

export const buildModelsListConfig = (state: ModelsListState): ModelsListConfig => ({
  enabled: state.enabled,
  models: state.items.length > 0
    ? state.items.filter(item => item.selected).map(item => item.id)
    : [...state.savedModels],
  ...(state.plazaConfigured ? { plaza_enabled: state.plazaEnabled ?? true, plaza_models: state.plazaCustomModels ? normalizeModels(state.plazaModels ?? []) : null } : {}),
})

export const buildModelAllowlistConfig = (state: ModelAllowlistState): ModelAllowlistConfig =>
  buildModelsListConfig(state)

export const selectedModelAllowlistCount = (state: ModelAllowlistState): number =>
  state.items.filter(item => item.selected).length

const normalizeModels = (models: string[]): string[] => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const raw of models) {
    const model = raw.trim()
    if (!model || seen.has(model)) {
      continue
    }
    seen.add(model)
    out.push(model)
  }
  return out
}
