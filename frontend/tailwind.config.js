/** @type {import('tailwindcss').Config} */

/*
 * Glass theme bridge for Tailwind.
 *
 * Every colour utility resolves to a CSS token from `src/styles/tokens.css`, so
 * the whole app — including legacy templates that still say `bg-red-50` or
 * `text-emerald-600` — follows the five semantic tones of the design system
 * (accent / success / warning / danger / muted) and flips correctly in dark mode.
 *
 * Colour values are functions so Tailwind's opacity modifier keeps working on tokens:
 *   bg-red-500/15  →  color-mix(in oklch, var(--danger) calc(0.15 * 100%), transparent)
 */

const alpha =
  (token) =>
  ({ opacityValue } = {}) =>
    opacityValue === undefined || opacityValue === '1' || opacityValue === 1
      ? token
      : `color-mix(in oklch, ${token} calc(${opacityValue} * 100%), transparent)`
const tint = (token, pct) => alpha(`color-mix(in oklch, ${token} ${pct}%, var(--surface))`)
const shade = (token, pct) => alpha(`color-mix(in oklch, ${token} ${pct}%, var(--foreground))`)

/** Build a 50–950 scale from a tone (`--x`) and its text variant (`--x-text`). */
function toneScale(base, text = base) {
  return {
    50: tint(base, 8),
    100: tint(base, 14),
    200: tint(base, 28),
    300: tint(base, 45),
    400: alpha(base),
    500: alpha(base),
    600: alpha(text),
    700: alpha(text),
    800: shade(text, 85),
    900: shade(text, 70),
    950: shade(text, 55),
    DEFAULT: alpha(base)
  }
}

const accentScale = toneScale('var(--accent)', 'var(--accent)')
const successScale = toneScale('var(--success)', 'var(--success-text)')
const warningScale = toneScale('var(--warning)', 'var(--warning-text)')
const dangerScale = toneScale('var(--danger)', 'var(--danger-text)')

const neutralScale = {
  50: alpha('var(--surface-secondary)'),
  100: alpha('var(--surface-secondary)'),
  200: alpha('var(--surface-tertiary)'),
  300: alpha('var(--border)'),
  400: alpha('var(--muted)'),
  500: alpha('var(--muted)'),
  600: alpha('var(--muted)'),
  700: shade('var(--muted)', 40),
  800: alpha('var(--foreground)'),
  900: alpha('var(--foreground)'),
  950: alpha('var(--foreground)'),
  DEFAULT: alpha('var(--muted)')
}

export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Glass semantic tokens
        canvas: 'var(--canvas)',
        background: alpha('var(--background)'),
        foreground: alpha('var(--foreground)'),
        surface: {
          DEFAULT: alpha('var(--surface)'),
          2: alpha('var(--surface-secondary)'),
          3: alpha('var(--surface-tertiary)')
        },
        muted: alpha('var(--muted)'),
        line: alpha('var(--border)'),
        success: { ...successScale, text: alpha('var(--success-text)') },
        warning: { ...warningScale, text: alpha('var(--warning-text)') },
        danger: { ...dangerScale, text: alpha('var(--danger-text)') },
        accent: accentScale,
        primary: accentScale,

        // Legacy palette names → semantic tones
        blue: accentScale,
        indigo: accentScale,
        sky: accentScale,
        cyan: accentScale,
        violet: accentScale,
        purple: accentScale,
        fuchsia: accentScale,
        pink: accentScale,
        teal: successScale,
        emerald: successScale,
        green: successScale,
        lime: successScale,
        amber: warningScale,
        yellow: warningScale,
        orange: warningScale,
        red: dangerScale,
        rose: dangerScale,
        gray: neutralScale,
        slate: neutralScale,
        zinc: neutralScale,
        neutral: neutralScale,
        stone: neutralScale,
        dark: neutralScale
      },
      fontFamily: {
        sans: ['var(--font-body)'],
        display: ['var(--display)'],
        mono: ['var(--font-mono)']
      },
      boxShadow: {
        glass: 'var(--shadow)',
        'glass-hover': 'var(--shadow-hover)',
        pop: 'var(--shadow-pop)',
        field: 'var(--field-shadow)',
        'glass-sm': 'var(--shadow)',
        glow: '0 8px 20px -10px var(--accent)',
        'glow-lg': '0 12px 28px -12px var(--accent)',
        card: 'var(--shadow)',
        'card-hover': 'var(--shadow-hover)',
        'inner-glow': 'inset 0 1px 0 var(--btn-hi)',
        sm: '0 1px 2px rgba(16, 24, 40, 0.06)',
        md: 'var(--shadow)',
        lg: 'var(--shadow-pop)',
        xl: 'var(--shadow-pop)',
        '2xl': 'var(--shadow-pop)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'var(--brand-gradient)',
        'gradient-dark': 'linear-gradient(135deg, var(--surface-secondary) 0%, var(--surface) 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient': 'var(--bg-ambient)'
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
          '0%': { boxShadow: '0 8px 20px -10px var(--accent)' },
          '100%': { boxShadow: '0 12px 28px -8px var(--accent)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem',
        sm: 'var(--radius-sm)',
        btn: 'var(--radius-btn)',
        field: 'var(--radius-field)',
        card: 'var(--radius-card)',
        hero: 'var(--radius-hero)',
        panel: 'var(--radius-panel)'
      }
    }
  },
  plugins: []
}
