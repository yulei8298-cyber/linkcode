import { BUILTIN_PLATFORM_CATALOG, type VideoProtocolInfo } from './platformCatalog'

export type { VideoProtocolInfo }

/**
 * 视频平台账号的上游协议与厂商品牌。协议清单（默认基址、原生透传前缀、示例模型）
 * 来自后端生成的平台清单 video_protocols；这里只补充前端展示用的厂商名与配色。
 */

export interface VideoVendorStyle {
  /** 品牌图标键，见 components/icons/brands/brandIcons.ts。 */
  icon: string
  label: string
  /** 浅色胶囊徽标（与平台徽标同一风格）。 */
  badge: string
  /** 单一品牌色，用于卡片描边、强调色。 */
  accent: string
}

const VENDOR_STYLES: Record<string, VideoVendorStyle> = {
  seedance: { icon: 'seedance', label: 'Seedance', badge: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300', accent: '#1664ff' },
  kling: { icon: 'kling', label: '可灵 Kling', badge: 'bg-lime-100 text-lime-800 dark:bg-lime-900/30 dark:text-lime-300', accent: '#65a30d' },
  hailuo: { icon: 'hailuo', label: '海螺 Hailuo', badge: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300', accent: '#ec4899' },
  wan: { icon: 'wan', label: '万相 Wan', badge: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300', accent: '#615ced' },
  veo: { icon: 'veo', label: 'Veo', badge: 'bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-300', accent: '#4285f4' },
  vidu: { icon: 'vidu', label: 'Vidu', badge: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300', accent: '#06b6d4' },
  runway: { icon: 'runway', label: 'Runway', badge: 'bg-zinc-100 text-zinc-800 dark:bg-zinc-800 dark:text-zinc-200', accent: '#52525b' },
  luma: { icon: 'luma', label: 'Luma', badge: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300', accent: '#8b5cf6' },
  pixverse: { icon: 'pixverse', label: 'PixVerse', badge: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-300', accent: '#c026d3' },
  openai: { icon: 'openai', label: 'OpenAI 兼容', badge: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300', accent: '#10b981' },
  newapi: { icon: 'newapi', label: 'new-api', badge: 'bg-teal-100 text-teal-700 dark:bg-teal-900/30 dark:text-teal-300', accent: '#14b8a6' },
}

const FALLBACK_STYLE: VideoVendorStyle = {
  icon: '',
  label: 'Video',
  badge: 'bg-fuchsia-100 text-fuchsia-700 dark:bg-fuchsia-900/30 dark:text-fuchsia-300',
  accent: '#d946ef',
}

/** 全部视频协议，按后端展示顺序。 */
export function listVideoProtocols(): VideoProtocolInfo[] {
  return BUILTIN_PLATFORM_CATALOG.video_protocols ?? []
}

export function getVideoProtocol(protocol: string | null | undefined): VideoProtocolInfo | undefined {
  if (!protocol) return undefined
  return listVideoProtocols().find(p => p.protocol === protocol)
}

/** 按协议返回厂商展示样式；未知协议回落通用视频样式。 */
export function videoVendorStyle(protocol: string | null | undefined): VideoVendorStyle {
  const vendor = getVideoProtocol(protocol)?.vendor
  return (vendor && VENDOR_STYLES[vendor]) || FALLBACK_STYLE
}

/** 按厂商标识（seedance、kling…）返回展示样式，供模型广场等按模型推断厂商的场景。 */
export function videoVendorStyleByVendor(vendor: string): VideoVendorStyle {
  return VENDOR_STYLES[vendor] || FALLBACK_STYLE
}

/** 由模型名推断厂商，用于模型广场与定价页的图标展示。 */
export function inferVideoVendor(model: string): string {
  const m = model.toLowerCase()
  const rules: Array<[RegExp, string]> = [
    [/seedance|doubao/, 'seedance'],
    [/kling/, 'kling'],
    [/hailuo|minimax/, 'hailuo'],
    [/^wan|wanx/, 'wan'],
    [/veo/, 'veo'],
    [/vidu/, 'vidu'],
    [/gen-?4|runway/, 'runway'],
    [/^ray|luma/, 'luma'],
    [/pixverse/, 'pixverse'],
    [/sora/, 'openai'],
  ]
  return rules.find(([re]) => re.test(m))?.[1] ?? ''
}
