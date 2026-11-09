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

function lightMutedLabelsMeetThreeToOneContrast() {
  expect(contrastRatio(LIGHT_ROLES.textMuted, LIGHT_ROLES.surface)).toBeGreaterThanOrEqual(3);
}

function darkMutedLabelsMeetThreeToOneContrast() {
  expect(contrastRatio(DARK_ROLES.textMuted, DARK_ROLES.surface)).toBeGreaterThanOrEqual(3);
}

function formerLightMutedLabelFailsThreeToOneContrast() {
  expect(contrastRatio('#8A91A0', LIGHT_ROLES.surface)).toBeLessThan(3);
}

test(lightMutedLabelsMeetThreeToOneContrast, lightMutedLabelsMeetThreeToOneContrast);
test(darkMutedLabelsMeetThreeToOneContrast, darkMutedLabelsMeetThreeToOneContrast);
test(formerLightMutedLabelFailsThreeToOneContrast, formerLightMutedLabelFailsThreeToOneContrast);
