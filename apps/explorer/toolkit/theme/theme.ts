import { createSystem, defaultConfig, defineConfig } from '@chakra-ui/react';

import { keyframes } from './foundations/animations';
import * as borders from './foundations/borders';
import breakpoints from './foundations/breakpoints';
import colors from './foundations/colors';
import durations from './foundations/durations';
import easings from './foundations/easings';
import semanticTokens from './foundations/semanticTokens';
import shadows from './foundations/shadows';
import sizes from './foundations/sizes';
import spacing from './foundations/spacing';
import { fonts, fontSizes, lineHeights, textStyles } from './foundations/typography';
import zIndex from './foundations/zIndex';
import globalCss from './globalCss';
import { recipes, slotRecipes } from './recipes';

export const customConfig = defineConfig({
  globalCss,
  theme: {
    breakpoints,
    keyframes,
    recipes,
    slotRecipes,
    semanticTokens,
    textStyles,
    tokens: {
      ...borders,
      colors,
      durations,
      easings,
      fonts,
      fontSizes,
      lineHeights,
      spacing,
      shadows,
      sizes,
      zIndex,
      fontWeights: {
        normal: { value: '400' },
        medium: { value: '500' },
        semibold: { value: '600' },
        bold: { value: '700' },
      },
    },
  },
});

export default createSystem(defaultConfig, customConfig);
