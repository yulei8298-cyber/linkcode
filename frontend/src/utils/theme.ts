// 主题偏好：用户手动切换过就以 localStorage 里的选择为准，
// 从未选择过的访客跟随系统的深浅色设置。
export const THEME_STORAGE_KEY = 'theme'

export function readSavedTheme(): 'light' | 'dark' | null {
  try {
    const value = localStorage.getItem(THEME_STORAGE_KEY)
    return value === 'light' || value === 'dark' ? value : null
  } catch {
    return null
  }
}

export function shouldUseDarkTheme(): boolean {
  const saved = readSavedTheme()
  if (saved) return saved === 'dark'
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function applyTheme(dark: boolean, persist = true): void {
  document.documentElement.classList.toggle('dark', dark)
  if (!persist) return
  try {
    localStorage.setItem(THEME_STORAGE_KEY, dark ? 'dark' : 'light')
  } catch {
    // 隐私模式等场景写入失败时仅本次生效
  }
}
