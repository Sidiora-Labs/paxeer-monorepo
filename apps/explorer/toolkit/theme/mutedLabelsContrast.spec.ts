import { expect, test } from 'vitest';

import { DARK as DARK_ROLES, LIGHT as LIGHT_ROLES } from './foundations/palette';

function relativeLuminance(hex: string): number {
  return [ 0.2126, 0.7152, 0.0722 ].reduce((luminance, weight, index) => {
    const channel = parseInt(hex.slice(1 + index * 2, 3 + index * 2), 16) / 255;
    const linear = channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
    return luminance + weight * linear;
  }, 0);
}

function contrastRatio(foreground: string, background: string): number {
  const foregroundLuminance = relativeLuminance(foreground);
  const backgroundLuminance = relativeLuminance(background);
  return (Math.max(foregroundLuminance, backgroundLuminance) + 0.05) /
    (Math.min(foregroundLuminance, backgroundLuminance) + 0.05);
}

function lightMutedLabelsMeetTextContrast() {
  expect(contrastRatio(LIGHT_ROLES.textMuted, LIGHT_ROLES.surface)).toBeGreaterThanOrEqual(4.5);
}

function darkMutedLabelsMeetTextContrast() {
  expect(contrastRatio(DARK_ROLES.textMuted, DARK_ROLES.surface)).toBeGreaterThanOrEqual(4.5);
}

function formerLightMutedLabelFailsThreeToOneContrast() {
  expect(contrastRatio('#8A91A0', LIGHT_ROLES.surface)).toBeLessThan(3);
}

function semanticStatusTextAndButtonLabelsMeetTextContrast() {
  for (const palette of [ LIGHT_ROLES, DARK_ROLES ]) {
    for (const foreground of [ palette.textMuted, palette.positive, palette.negative, palette.warning ]) {
      expect(contrastRatio(foreground, palette.surface)).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrastRatio(palette.onAccent, palette.accent)).toBeGreaterThanOrEqual(4.5);
  }
}

test(semanticStatusTextAndButtonLabelsMeetTextContrast, semanticStatusTextAndButtonLabelsMeetTextContrast);
test(lightMutedLabelsMeetTextContrast, lightMutedLabelsMeetTextContrast);
test(darkMutedLabelsMeetTextContrast, darkMutedLabelsMeetTextContrast);
test(formerLightMutedLabelFailsThreeToOneContrast, formerLightMutedLabelFailsThreeToOneContrast);
