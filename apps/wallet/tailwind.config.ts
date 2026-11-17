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
        'card-inline': 'var(--space-card-inline)',
        'icon-30': 'var(--size-icon-30)',
        'icon-42': 'var(--size-icon-42)',
        toast: 'var(--size-toast)',
        ...Object.fromEntries(
          Object.keys(themeCatalogue.layout.space).map((step) => [
            step,
            `var(--space-${step.replace('.', '-')})`,
          ])
        ),
      },
      fontSize: {
        xs: ['var(--type-size-xs)', { lineHeight: 'var(--type-line-xs)' }],
        sm: ['var(--type-size-sm)', { lineHeight: 'var(--type-line-sm)' }],
        base: ['var(--type-size-md)', { lineHeight: 'var(--type-line-md)' }],
        lg: ['var(--type-size-lg)', { lineHeight: 'var(--type-line-lg)' }],
        xl: ['var(--type-size-xl)', { lineHeight: 'var(--type-line-xl)' }],
        '2xl': ['var(--font-size-2xl)', { lineHeight: 'var(--line-height-2xl)' }],
        '3xl': ['var(--font-size-3xl)', { lineHeight: 'var(--line-height-3xl)' }],
        display: ['var(--font-size-display)', { lineHeight: 'var(--line-height-display)' }],
        ...Object.fromEntries(
          [8, 9, 10, 11, 13, 15, 22, 32, 45].map((size) => [
            `size-${size}`,
            [`var(--font-size-${size})`, { lineHeight: 'var(--line-height-body)' }],
          ])
        ),
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
        '10': 'var(--radius-10)',
        sm: 'var(--radius-2xs)',
        md: 'var(--radius-xs)',
        lg: 'var(--radius-sm)',
        xl: 'var(--radius-md)',
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
      maxWidth: {
        'size-140': 'var(--size-copy-140)',
        'size-180': 'var(--size-copy-180)',
        'size-260': 'var(--size-copy-260)',
        'size-430': 'var(--size-panel-430)',
      },
      fontWeight: {
        light: 'var(--font-weight-light)',
        normal: 'var(--font-weight-regular)',
        medium: 'var(--font-weight-medium)',
        semibold: 'var(--font-weight-semibold)',
        bold: 'var(--font-weight-bold)',
      },
      lineHeight: {
        tight: 'var(--line-height-compact)',
        snug: 'var(--line-height-snug)',
        normal: 'var(--line-height-body)',
        relaxed: 'var(--line-height-relaxed)',
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
