import { themeCatalogue } from './src/theme/catalogue';
import type { Config } from 'tailwindcss';

const config: Config = {
  content: ['./src/**/*.{ts,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        pax: {
          bg: 'var(--color-surface-base)',
          surface: 'var(--color-surface-raised)',
          card: 'var(--color-surface-card)',
          accent: 'var(--color-action-primary)',
          'accent-dim': 'var(--color-action-primary-muted)',
          'mid-dark': 'var(--color-text-disabled)',
          mid: 'var(--color-text-tertiary)',
          subtle: 'var(--color-text-secondary)',
          light: 'var(--color-text-primary)',
          'off-white': 'var(--color-text-strong)',
          muted: 'var(--color-text-secondary)',
          'subtle-bg': 'var(--color-surface-control)',
          success: 'var(--color-status-success)',
          error: 'var(--color-status-danger)',
          warning: 'var(--color-status-warning)',
          info: 'var(--color-status-info)',
          overlay: 'var(--color-surface-overlay)',
          control: 'var(--color-surface-control)',
          'on-accent': 'var(--color-action-on-primary)',
          line: 'var(--color-border-subtle)',
          'line-strong': 'var(--color-border-strong)',
          scrim: 'var(--color-overlay-scrim)',
          glass: 'var(--color-overlay-glass)',
          'error-muted': 'var(--color-status-danger-muted)',
          'success-muted': 'var(--color-status-success-muted)',
          'warning-muted': 'var(--color-status-warning-muted)',
          'info-muted': 'var(--color-status-info-muted)',
          'error-surface': 'var(--color-status-danger-surface)',
          'success-surface': 'var(--color-status-success-surface)',
          'warning-surface': 'var(--color-status-warning-surface)',
          'accent-muted': 'var(--color-action-primary-muted-surface)',
        },
      },
      textColor: { 'pax-accent': 'var(--color-action-text)' },
      spacing: {
        px: 'var(--space-px)',
        ...Object.fromEntries(
          Object.keys(themeCatalogue.layout.space).map((step) => [
            step,
            `var(--space-${step.replace('.', '-')})`,
          ])
        ),
      },
      fontSize: {
        xs: ['var(--type-size-xs)', { lineHeight: 'var(--line-height-body)' }],
        sm: ['var(--type-size-sm)', { lineHeight: 'var(--line-height-body)' }],
        base: ['var(--type-size-md)', { lineHeight: 'var(--line-height-body)' }],
        lg: ['var(--type-size-lg)', { lineHeight: 'var(--line-height-tight)' }],
        xl: ['var(--type-size-xl)', { lineHeight: 'var(--line-height-tight)' }],
        '2xl': ['var(--font-size-2xl)', { lineHeight: 'var(--line-height-tight)' }],
        '3xl': ['var(--font-size-3xl)', { lineHeight: 'var(--line-height-tight)' }],
        display: ['var(--font-size-display)', { lineHeight: 'var(--line-height-tight)' }],
      },
      fontFamily: {
        sans: ['var(--font-sans)'],
        display: ['var(--font-display)'],
        mono: ['var(--font-mono)'],
      },
      fontVariantNumeric: {
        tabular: 'tabular-nums',
      },
      borderRadius: {
        sm: 'var(--radius-sm)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        full: 'var(--radius-full)',
        '2xl': 'var(--radius-lg)',
        '3xl': 'var(--radius-xl)',
      },
      boxShadow: {
        card: 'var(--elevation-card)',
        menu: 'var(--elevation-menu)',
        sheet: 'var(--elevation-sheet)',
        popover: 'var(--elevation-popover)',
      },
      height: {
        'card-compact': 'var(--size-card-compact)',
        'card-row': 'var(--size-card-row)',
        'card-expanded': 'var(--size-card-expanded)',
        'chart-area': 'var(--size-chart-area)',
      },
      minHeight: {
        'card-featured': 'var(--size-card-featured)',
        'card-small': 'var(--size-card-small)',
        'card-medium': 'var(--size-card-medium)',
        'card-token': 'var(--size-card-token)',
      },
      minWidth: {
        table: 'var(--size-table-min)',
      },
      fontWeight: {
        light: 'var(--font-weight-light)',
        normal: 'var(--font-weight-regular)',
        medium: 'var(--font-weight-medium)',
        semibold: 'var(--font-weight-semibold)',
        bold: 'var(--font-weight-bold)',
      },
      lineHeight: {
        tight: 'var(--line-height-tight)',
        snug: 'var(--line-height-body)',
        normal: 'var(--line-height-body)',
        relaxed: 'var(--line-height-body)',
      },
      letterSpacing: {
        label: 'var(--tracking-label)',
      },
      transitionDuration: {
        DEFAULT: 'var(--motion-fast)',
        fast: 'var(--motion-fast)',
        normal: 'var(--motion-normal)',
        slow: 'var(--motion-slow)',
        spinner: 'var(--motion-shimmer)',
      },
      transitionTimingFunction: {
        pax: 'var(--pax-ease)',
      },
      animation: {
        'fade-in': 'fadeIn var(--motion-normal) var(--motion-ease-standard)',
        'slide-up': 'slideUp var(--motion-slow) var(--motion-ease-standard)',
        'scale-in': 'scaleIn var(--motion-fast) var(--motion-ease-standard)',
      },
      keyframes: {
        fadeIn: {
          from: { opacity: '0' },
          to: { opacity: '1' },
        },
        slideUp: {
          from: { transform: 'translateY(100%)' },
          to: { transform: 'translateY(0)' },
        },
        scaleIn: {
          from: { transform: 'scale(0.95)', opacity: '0' },
          to: { transform: 'scale(1)', opacity: '1' },
        },
      },
    },
  },
  plugins: [],
};

export default config;
