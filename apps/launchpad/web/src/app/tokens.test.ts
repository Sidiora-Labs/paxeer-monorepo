import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { afterAll, beforeAll, expect, test } from 'vitest';
import { chromium, type Browser } from 'playwright';
import { launchpadCanvasColors, launchpadColors } from './presentation-tokens';

const css = readFileSync(join(process.cwd(), 'src/app/tokens.css'), 'utf8');
const brief = {
  canvas: ['#0b0b0e', '#f7f9fc'],
  'surface-1': ['#121216', '#ffffff'],
  'surface-2': ['#19191f', '#eaf0f5'],
  'surface-3': ['#23232b', '#dfe7ee'],
  'surface-hover': ['#2a2a33', '#d3dee8'],
  'surface-active': ['#34343f', '#c7d6e2'],
  text: ['#f5f5f7', '#101c29'],
  'text-muted': ['#a6a6b0', '#34495e'],
  'text-faint': ['#9f9faa', '#435568'],
  'text-disabled': ['#5c5c66', '#657587'],
  'control-edge': ['#81818e', '#657587'],
  accent: ['#44e8c8', '#006852'],
  'accent-hover': ['#6bedd5', '#005844'],
  'accent-active': ['#2bb89e', '#004735'],
  'on-accent': ['#06201b', '#ffffff'],
  'accent-soft': ['#11302b', '#d7f4e9'],
  'accent-soft-strong': ['#1a4a42', '#b9e7d8'],
  danger: ['#ff6f73', '#ac1826'],
  'danger-strong': ['#ff8d90', '#a11422'],
  'on-danger': ['#240708', '#ffffff'],
  'danger-soft': ['#381a1d', '#ffe5e7'],
  warning: ['#ffd24a', '#795400'],
  'warning-soft': ['#2e2a18', '#fff0c2'],
  info: ['#a6baff', '#2d49a9'],
  'info-soft': ['#1f2438', '#e6ebff'],
  'surface-inverse': ['#f5f5f7', '#101c29'],
  focus: ['#8bffc5', '#005a48'],
} as const;

let browser: Browser;
beforeAll(async () => { browser = await chromium.launch({ headless: true }); });
afterAll(async () => { await browser.close(); });

function everyBriefRoleHasBothPalettes() {
  for (const [role, values] of Object.entries(brief)) {
    for (const [index, scheme] of (['dark', 'light'] as const).entries()) {
      expect(css, `${role} ${scheme}`).toContain(`--sl-${role}-${scheme}: ${values[index]};`);
      expect(launchpadColors[scheme][role as keyof typeof brief]).toBe(values[index]);
    }
  }
  expect(launchpadCanvasColors.dark).toBe(brief.canvas[0]);
  expect(launchpadCanvasColors.light).toBe(brief.canvas[1]);
}

async function explicitModesOverrideDeviceAndDeviceModeFollowsIt() {
  const page = await browser.newPage();
  try {
    await page.setContent(`<style>${css}</style><span id="probe">Value</span>`);
    for (const device of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: device });
      for (const mode of [null, 'dark', 'light', 'device'] as const) {
        const scheme = mode === 'device' ? device : mode ?? 'dark';
        const result = await page.evaluate(({ mode, roles }) => {
          const root = document.documentElement;
          if (mode === null) root.removeAttribute('data-color-mode');
          else root.setAttribute('data-color-mode', mode);
          const probe = document.querySelector<HTMLElement>('#probe')!;
          const colors: Record<string, string> = {};
          for (const role of roles) {
            probe.style.color = `var(--sl-${role})`;
            colors[role] = getComputedStyle(probe).color;
          }
          return { colors, scheme: getComputedStyle(root).colorScheme };
        }, { mode, roles: Object.keys(brief) });
        expect(result.scheme, `${mode} on ${device}`).toBe(scheme);
        for (const [role, values] of Object.entries(brief)) {
          const hex = values[scheme === 'dark' ? 0 : 1];
          const rgb = [1, 3, 5].map(index => parseInt(hex.slice(index, index + 2), 16));
          expect(result.colors[role], `${role}: ${mode} on ${device}`).toBe(`rgb(${rgb.join(', ')})`);
        }
      }
    }
  } finally { await page.close(); }
}

function typeStepsAndControlAndRowSizesStayWithLighterWeights() {
  const steps = {
    '2xs': [11, 14, 600], xs: [12, 16, 500], sm: [13, 18, 500], base: [14, 20, 500],
    md: [16, 24, 600], lg: [20, 28, 700], xl: [24, 32, 700], '2xl': [32, 40, 700],
    display: [48, 52, 700], 'num-sm': [13, 18, 600], 'num-md': [14, 20, 600],
  };
  for (const [step, [size, line, weight]] of Object.entries(steps)) {
    expect(css).toContain(`--sl-text-${step}: ${size}px;`);
    expect(css).toContain(`--sl-leading-${step}: ${line}px;`);
    expect(css).toContain(`--sl-weight-${step}: ${weight};`);
  }
  for (const [name, value] of Object.entries({
    'control-height-sm': '28px', 'control-height-md': '36px', 'control-height-lg': '44px',
    'table-row-dense': '2.5rem', 'table-row': '3rem', 'table-row-comfortable': '3.5rem',
    'list-row': '3.5rem', 'list-row-lg': '4rem', 'header-height': '48px', 'ticker-height': '32px',
    'radius-xs': '0.5rem', 'radius-sm': '0.75rem', 'radius-md': '1rem',
    'radius-sheet': '1.25rem', 'radius-coin': '22%', 'radius-direction': '0.375rem',
    'duration-fast': '120ms', 'duration-base': '180ms', 'duration-slow': '260ms', 'duration-flash': '600ms',
  })) expect(css).toContain(`--sl-${name}: ${value};`);
  expect(css).not.toMatch(/--sl-weight-[\w-]+: 800/);
}

function luminance(hex: string) {
  const [r, g, b] = [1, 3, 5].map(index => {
    const value = parseInt(hex.slice(index, index + 2), 16) / 255;
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  });
  return r * 0.2126 + g * 0.7152 + b * 0.0722;
}

function contrastAdjustedRolesHoldAcrossRaisedSurfaces() {
  for (const scheme of ['light', 'dark'] as const) {
    const palette = launchpadColors[scheme];
    for (const surface of ['canvas', 'surface-1', 'surface-2', 'surface-3', 'surface-hover', 'surface-active'] as const) {
      for (const [role, minimum] of [['text', 4.5], ['text-muted', 4.5], ['text-faint', 4.5], ['control-edge', 3]] as const) {
        const values = [luminance(palette[role]), luminance(palette[surface])].sort((a, b) => a - b);
        expect((values[1] + 0.05) / (values[0] + 0.05), `${scheme} ${role} on ${surface}`).toBeGreaterThanOrEqual(minimum);
      }
    }
  }
}

for (const check of [everyBriefRoleHasBothPalettes, explicitModesOverrideDeviceAndDeviceModeFollowsIt,
  typeStepsAndControlAndRowSizesStayWithLighterWeights, contrastAdjustedRolesHoldAcrossRaisedSurfaces]) test(check.name, check);
