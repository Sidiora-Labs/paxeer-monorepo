export const launchpadColors = {
  dark: {
    'surface-inverse': '#f5f5f7',
    'text': '#f5f5f7',
    'canvas': '#0b0b0e',
    'surface-1': '#121216',
    'surface-2': '#19191f',
    'surface-3': '#23232b',
    'surface-hover': '#2a2a33',
    'surface-active': '#34343f',
    'text-muted': '#a6a6b0',
    'text-faint': '#9f9faa',
    'text-disabled': '#5c5c66',
    'accent': '#44e8c8',
    'accent-hover': '#6bedd5',
    'accent-active': '#2bb89e',
    'on-accent': '#06201b',
    'accent-soft': '#11302b',
    'accent-soft-strong': '#1a4a42',
    'danger': '#ff6f73',
    'danger-strong': '#ff8d90',
    'on-danger': '#240708',
    'danger-soft': '#381a1d',
    'warning': '#ffd24a',
    'warning-strong': '#ffb25b',
    'on-warning': '#201a00',
    'warning-soft': '#2e2a18',
    'info': '#a6baff',
    'on-info': '#0b1030',
    'info-soft': '#1f2438',
    'focus': '#8bffc5',
    'volume-up': '#44e8c8',
    'volume-down': '#ff6f73',
    'rank-2': '#c9d1e0',
    'rank-3': '#ffa861',
    'rank-1-soft': '#282820',
    'text-inverse': '#0b0b0e',
    'control-edge': '#81818e',
  },
  light: {
    'surface-inverse': '#101c29',
    'text': '#101c29',
    'canvas': '#f7f9fc',
    'surface-1': '#ffffff',
    'surface-2': '#eaf0f5',
    'surface-3': '#dfe7ee',
    'surface-hover': '#d3dee8',
    'surface-active': '#c7d6e2',
    'text-muted': '#34495e',
    'text-faint': '#435568',
    'text-disabled': '#657587',
    'accent': '#006852',
    'accent-hover': '#005844',
    'accent-active': '#004735',
    'on-accent': '#ffffff',
    'accent-soft': '#d7f4e9',
    'accent-soft-strong': '#b9e7d8',
    'danger': '#ac1826',
    'danger-strong': '#a11422',
    'on-danger': '#ffffff',
    'danger-soft': '#ffe5e7',
    'warning': '#795400',
    'warning-strong': '#794800',
    'on-warning': '#ffffff',
    'warning-soft': '#fff0c2',
    'info': '#2d49a9',
    'on-info': '#ffffff',
    'info-soft': '#e6ebff',
    'focus': '#005a48',
    'volume-up': '#006852',
    'volume-down': '#ac1826',
    'rank-2': '#435568',
    'rank-3': '#884500',
    'rank-1-soft': '#fff0c2',
    'text-inverse': '#ffffff',
    'control-edge': '#657587',
  },
} as const;

/** Canvas roles for browser chrome, which does not resolve CSS custom properties. */
export const launchpadCanvasColors = {
  light: launchpadColors.light.canvas,
  dark: launchpadColors.dark.canvas,
} as const;

/** Pixel values for resizable workspaces and chart APIs. */
export const launchpadGeometry = {
  images: {
    brand: 28,
    wallet: 48,
    preview: 120,
    banner: { width: 1440, height: 240 },
    card: { width: 1200, height: 630 },
    stream: { width: 640, height: 360 },
    streamCoin: 320,
    submission: { width: 640, height: 480 },
    attachment: 56,
  },
  smallBadge: 14,
  hairline: 1,
  chartStroke: 2,
  iconStroke: 1.5,
  columns: {
    leftMin: 240,
    leftDefault: 296,
    rightMin: 280,
    rightDefault: 312,
    centerMin: 560,
    sideMax: 640,
    collapsed: 24,
    gutter: 4,
    frameInset: 4,
  },
  chart: {
    minHeight: 320,
    defaultHeight: 380,
    maxHeight: 960,
    fullscreenChrome: 140,
    portfolioHeight: 200,
    portfolioCompactHeight: 180,
  },
  streamSparkline: { overlay: { width: 90, height: 30 }, hero: { width: 460, height: 120 } },
  marker: { minSize: 18, borderWidth: 0, hoveredBorderWidth: 2 },
  statusPanel: { width: 360, inset: 4 },
} as const;

export const launchpadMotion = {
  fast: 120,
  base: 180,
  slow: 260,
  value: 600,
  toastLifetime: 6000,
  copyConfirmation: 1500,
} as const;

/** Viewport tiers shared by responsive behavior and the application shell. */
export const launchpadMedia = {
  medium: '(min-width: 768px)',
  large: '(min-width: 1024px)',
  extraLarge: '(min-width: 1280px)',
  compact: '(max-width: 1023px)',
} as const;

/** Default chart marker roles; mounted charts supply the active scheme palette. */
export const launchpadMarkColors = {
  buy: launchpadColors.dark.accent,
  sell: launchpadColors.dark.danger,
  label: launchpadColors.dark['on-accent'],
} as const;

/** A dimension expressed in steps of the surface spacing scale. */
export function surfaceSpacing(steps: number): string {
  return `calc(var(--sl-space) * ${steps})`;
}

/** Surface roles consumed by the embedded chart's toolbar and menus. */
export const launchpadChartRoles = {
  '--tv-color-pane-background': '--sl-canvas',
  '--tv-color-platform-background': '--sl-canvas',
  '--tv-color-toolbar-button-background-hover': '--sl-surface-hover',
  '--tv-color-toolbar-button-background-secondary-hover': '--sl-surface-hover',
  '--tv-color-toolbar-button-background-expanded': '--sl-surface-3',
  '--tv-color-toolbar-button-text': '--sl-text-muted',
  '--tv-color-toolbar-button-text-hover': '--sl-text',
  '--tv-color-toolbar-button-text-active': '--sl-accent',
  '--tv-color-toolbar-button-text-active-hover': '--sl-accent-hover',
  '--tv-color-toolbar-toggle-button-background-active': '--sl-accent-soft',
  '--tv-color-toolbar-toggle-button-background-active-hover': '--sl-surface-active',
  '--tv-color-toolbar-divider-background': '--sl-canvas',
  '--tv-color-item-active-text': '--sl-accent',
  '--tv-color-popup-background': '--sl-surface-3',
  '--tv-color-popup-element-background-hover': '--sl-surface-hover',
  '--tv-color-popup-element-background-active': '--sl-surface-active',
  '--tv-color-popup-element-text': '--sl-text',
  '--tv-color-popup-element-text-hover': '--sl-text',
  '--tv-color-popup-element-secondary-text': '--sl-text-muted',
  '--tv-color-popup-element-hint-text': '--sl-text-faint',
  '--tv-color-popup-element-divider-background': '--sl-surface-3',
} as const;
