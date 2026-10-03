/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // 主色 - LinkCode 品牌橙：只用于强调（链接、选中、焦点、关键数据），大面积主按钮用墨黑/纸白
        primary: {
          50: '#fff5ef',
          100: '#ffe8da',
          200: '#ffcdb0',
          300: '#ffab7d',
          400: '#ff8a4c',
          500: '#f0702f',
          600: '#d4561b',
          700: '#b0440f',
          800: '#8a3610',
          900: '#6f2e11',
          950: '#3c1506'
        },
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
        }
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
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
