/** @type {import('tailwindcss').Config} */

// 设计稿 A 的低饱和色系。全站页面大量直接使用 Tailwind 彩色类（bg-blue-100、text-emerald-600…），
// 这里把各彩色色阶统一映射到同一组克制色调，保留「绿=正常、红=异常、黄=警告」的语义，
// 不改任何类名即可让所有页面换到新视觉。600 / 400 两档分别对齐 lc-tokens 的浅色 / 深色状态色。
// 品牌橙：primary 与 orange 共用，页面里散落的 orange 类也落到品牌色上
const BRAND = { 50: '#fff5ef', 100: '#ffe8da', 200: '#ffcdb0', 300: '#ffab7d', 400: '#ff8a4c', 500: '#f0702f', 600: '#d4561b', 700: '#b0440f', 800: '#8a3610', 900: '#6f2e11', 950: '#3c1506' }
const OK = { 50: '#eef6f1', 100: '#dcede2', 200: '#b9dbc6', 300: '#96cfac', 400: '#6fb88a', 500: '#4a9a6b', 600: '#2f7a50', 700: '#276643', 800: '#215238', 900: '#1c432f', 950: '#0f261a' }
const INFO = { 50: '#eff3f6', 100: '#dfe7ed', 200: '#c2d2de', 300: '#b0c8d9', 400: '#86a7bf', 500: '#5f8199', 600: '#48667c', 700: '#3c5567', 800: '#334655', 900: '#2c3b47', 950: '#1a242c' }
const WARN = { 50: '#fbf5e9', 100: '#f6eacd', 200: '#edd49e', 300: '#e6bf78', 400: '#e0a85a', 500: '#c38a36', 600: '#a37422', 700: '#8f6416', 800: '#714f15', 900: '#5c4115', 950: '#33230a' }
const BAD = { 50: '#fbf1ef', 100: '#f7e0dc', 200: '#efc0b9', 300: '#e69e94', 400: '#e07d70', 500: '#c85a4d', 600: '#b0453a', 700: '#923a31', 800: '#78322b', 900: '#642c27', 950: '#371411' }
const VIOLET = { 50: '#f5f2f9', 100: '#ebe4f3', 200: '#d6c9e7', 300: '#bda9d8', 400: '#a68cc8', 500: '#8d71b3', 600: '#775c9c', 700: '#634c82', 800: '#52406b', 900: '#453759', 950: '#2a2036' }
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色 - LinkCode 品牌橙：只用于强调（链接、选中、焦点、关键数据），大面积主按钮用墨黑/纸白
        primary: BRAND,
        // 浅色中性色 - 暖灰，替代默认冷灰
        gray: {
          50: '#f6f6f3',
          100: '#efefeb',
          200: '#e3e3de',
          300: '#cfcfc8',
          400: '#a3a39b',
          500: '#6c6c64',
          600: '#57574f',
          700: '#3f3f39',
          800: '#262622',
          900: '#141413',
          950: '#0b0b0a'
        },
        // 辅助色 - 与 gray 同源的中性色（保留色名以兼容既有类名）
        accent: {
          50: '#f6f6f3',
          100: '#efefeb',
          200: '#e3e3de',
          300: '#cfcfc8',
          400: '#a3a39b',
          500: '#6c6c64',
          600: '#57574f',
          700: '#3f3f39',
          800: '#262622',
          900: '#141413',
          950: '#0b0b0a'
        },
        // 深色模式分层：950 页面底 / 900 次级底 / 800 卡片 / 700 描边与悬停 / 400~300 次要文字
        dark: {
          50: '#f2f2ef',
          100: '#e8e8e4',
          200: '#cfcfca',
          300: '#a6a6a1',
          400: '#85857f',
          500: '#5a5a56',
          600: '#38383b',
          700: '#26262a',
          800: '#141415',
          900: '#0f0f10',
          950: '#0b0b0c'
        },
        green: OK, emerald: OK, teal: OK, lime: OK,
        blue: INFO, sky: INFO, cyan: INFO, indigo: INFO,
        amber: WARN, yellow: WARN,
        red: BAD, rose: BAD, pink: BAD,
        violet: VIOLET, purple: VIOLET, fuchsia: VIOLET,
        orange: BRAND
      },
      fontFamily: {
        sans: [
          'PingFang SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'Noto Sans SC',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'sans-serif'
        ],
        mono: ['JetBrains Mono', 'SF Mono', 'SFMono-Regular', 'ui-monospace', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      boxShadow: {
        // 不使用发光与毛玻璃阴影；保留键名以兼容既有类名
        glass: '0 1px 2px rgba(20, 20, 10, 0.04)',
        'glass-sm': '0 1px 2px rgba(20, 20, 10, 0.04)',
        glow: 'none',
        'glow-lg': 'none',
        card: '0 1px 2px rgba(20, 20, 10, 0.04)',
        'card-hover': '0 8px 24px rgba(20, 20, 10, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.05)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(#d4561b, #d4561b)',
        'gradient-dark': 'linear-gradient(#141415, #141415)',
        'gradient-glass': 'none',
        'mesh-gradient': 'none'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { opacity: '1' },
          '100%': { opacity: '1' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      // 设计稿 A 的卡片圆角上限 16px：页面里的大圆角（3xl / 4xl）统一收敛，避免「气泡卡片」观感
      borderRadius: {
        '3xl': '1rem',
        '4xl': '1rem'
      }
    }
  },
  plugins: []
}
