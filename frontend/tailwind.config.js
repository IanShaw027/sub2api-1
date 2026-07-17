/** @type {import('tailwindcss').Config} */
/**
 * Clomio-aligned Tailwind theme for sub2api.
 * - primary.* stays brand-teal (P1: protect ~1700 primary-* usages)
 * - accent.* is interactive blue (was slate; only ~1 consumer)
 * - brand / ink / page / line tokens for new Clomio UI
 * - dark.* kept for existing dark:bg-dark-* classes
 */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // P1: full primary ladder = Clomio brand teal (exact anchors + interpolation)
        primary: {
          50: '#eafaf8',
          100: '#cff4ef',
          200: '#99ede3',
          300: '#5ed9cc',
          400: '#2dc9b8',
          500: '#17b8a6',
          600: '#0fa393',
          700: '#0b8578',
          800: '#0a6b60',
          900: '#0a544c',
          950: '#042f2c'
        },
        // Brand alias (CSS-variable driven for alpha)
        brand: {
          DEFAULT: 'rgb(var(--c-brand) / <alpha-value>)',
          50: 'rgb(var(--c-brand-50) / <alpha-value>)',
          100: 'rgb(var(--c-brand-100) / <alpha-value>)',
          200: 'rgb(var(--c-brand-200) / <alpha-value>)',
          300: 'rgb(var(--c-brand-300) / <alpha-value>)',
          400: 'rgb(var(--c-brand-400) / <alpha-value>)',
          500: 'rgb(var(--c-brand) / <alpha-value>)',
          600: 'rgb(var(--c-brand-600) / <alpha-value>)',
          700: 'rgb(var(--c-brand-700) / <alpha-value>)',
          800: 'rgb(var(--c-brand-800) / <alpha-value>)',
          900: 'rgb(var(--c-brand-900) / <alpha-value>)',
          950: 'rgb(var(--c-brand-950) / <alpha-value>)',
          cyan: 'rgb(var(--c-brand-cyan) / <alpha-value>)'
        },
        // Interactive blue (Clomio accent) — replaces former slate accent
        accent: {
          DEFAULT: 'rgb(var(--c-accent) / <alpha-value>)',
          50: 'rgb(var(--c-accent-50) / <alpha-value>)',
          100: 'rgb(var(--c-accent-100) / <alpha-value>)',
          200: 'rgb(var(--c-accent-200) / <alpha-value>)',
          300: 'rgb(var(--c-accent-300) / <alpha-value>)',
          400: 'rgb(var(--c-accent-400) / <alpha-value>)',
          500: 'rgb(var(--c-accent) / <alpha-value>)',
          600: 'rgb(var(--c-accent-600) / <alpha-value>)',
          700: 'rgb(var(--c-accent-700) / <alpha-value>)',
          800: 'rgb(var(--c-accent-800) / <alpha-value>)',
          900: 'rgb(var(--c-accent-900) / <alpha-value>)',
          950: 'rgb(var(--c-accent-950) / <alpha-value>)'
        },
        gold: 'rgb(var(--c-gold) / <alpha-value>)',
        success: {
          DEFAULT: 'rgb(var(--c-success) / <alpha-value>)',
          soft: 'rgb(var(--c-success-soft) / <alpha-value>)'
        },
        warning: {
          DEFAULT: 'rgb(var(--c-warning) / <alpha-value>)',
          soft: 'rgb(var(--c-warning-soft) / <alpha-value>)'
        },
        danger: {
          DEFAULT: 'rgb(var(--c-danger) / <alpha-value>)',
          soft: 'rgb(var(--c-danger-soft) / <alpha-value>)'
        },
        info: 'rgb(var(--c-info) / <alpha-value>)',
        page: 'rgb(var(--c-page) / <alpha-value>)',
        card: 'rgb(var(--c-card) / <alpha-value>)',
        line: 'rgb(var(--c-line) / <alpha-value>)',
        divider: 'rgb(var(--c-divider) / <alpha-value>)',
        ink: {
          DEFAULT: 'rgb(var(--c-ink) / <alpha-value>)',
          body: 'rgb(var(--c-ink-body) / <alpha-value>)',
          soft: 'rgb(var(--c-ink-soft) / <alpha-value>)',
          faint: 'rgb(var(--c-ink-faint) / <alpha-value>)'
        },
        // Legacy dark neutrals — keep for existing dark:bg-dark-* classes
        dark: {
          50: '#f8fafc',
          100: '#f1f5f9',
          200: '#e2e8f0',
          300: '#cbd5e1',
          400: '#94a3b8',
          500: '#64748b',
          600: '#475569',
          700: '#334155',
          800: '#1e293b',
          900: '#0f172a',
          950: '#020617'
        }
      },
      fontFamily: {
        sans: [
          'Inter Variable',
          'Inter',
          'system-ui',
          '-apple-system',
          'BlinkMacSystemFont',
          'Segoe UI',
          'Roboto',
          'Helvetica Neue',
          'Arial',
          'PingFang SC',
          'HarmonyOS Sans SC',
          'Hiragino Sans GB',
          'Microsoft YaHei',
          'sans-serif'
        ],
        mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
      },
      fontSize: {
        aux: ['12px', '18px'],
        'base-lg': ['15px', '24px'],
        title: ['16px', '24px'],
        'title-md': ['18px', '26px'],
        'title-lg': ['20px', '28px'],
        'title-xl': ['22px', '30px'],
        display: ['28px', '36px']
      },
      borderRadius: {
        control: '10px',
        card: '16px',
        hero: '20px',
        chip: '9999px',
        '4xl': '2rem'
      },
      boxShadow: {
        // Clomio elevation (docs/13)
        xs: '0 1px 2px rgba(22, 49, 79, 0.05)',
        md: '0 6px 16px -4px rgba(22, 49, 79, 0.08)',
        lg: '0 16px 40px -8px rgba(22, 49, 79, 0.12)',
        overlay:
          '0 8px 24px rgba(22, 49, 79, 0.12), 0 -4px 16px rgba(22, 49, 79, 0.06), 0 24px 48px rgba(22, 49, 79, 0.16)',
        glass: '0 8px 32px rgba(22, 49, 79, 0.08)',
        'glass-sm': '0 4px 16px rgba(22, 49, 79, 0.06)',
        // Brand teal glow (updated from #14b8a6 → #17b8a6)
        glow: '0 0 20px rgba(23, 184, 166, 0.25)',
        'glow-lg': '0 0 40px rgba(23, 184, 166, 0.35)',
        card: '0 1px 2px rgba(22, 49, 79, 0.05)',
        'card-hover': '0 6px 16px -4px rgba(22, 49, 79, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary':
          'linear-gradient(135deg, rgb(var(--c-brand)) 0%, rgb(var(--c-brand-600)) 100%)',
        'brand-gradient':
          'linear-gradient(135deg, rgb(var(--c-grad-from)) 0%, rgb(var(--c-grad-to)) 100%)',
        'gradient-dark': 'linear-gradient(135deg, #1e293b 0%, #0f172a 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgba(23, 184, 166, 0.12) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(31, 162, 214, 0.08) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(23, 184, 166, 0.08) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        'clomio-shimmer': 'clomioShimmer 1.2s ease-in-out infinite',
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
        clomioShimmer: {
          '0%': { backgroundPosition: '100% 0' },
          '100%': { backgroundPosition: '-100% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(23, 184, 166, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(23, 184, 166, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      }
    }
  },
  plugins: []
}
