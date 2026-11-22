/** Canvas roles for browser chrome, which does not resolve CSS custom properties. */
export const launchpadCanvasColors = {
  light: '#f7f9fc',
  dark: '#07090d',
} as const;

/** Pixel values for resizable workspaces and chart APIs. */
export const launchpadGeometry = {
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
  slow: 260,
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
  buy: '#006852',
  sell: '#ac1826',
  label: '#ffffff',
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
