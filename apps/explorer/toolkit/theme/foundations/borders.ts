import type { ThemingConfig } from '@chakra-ui/react';

import type { ExcludeUndefined } from 'types/utils';

// Compatibility steps share the explorer's control, card and pill radii.
export const radii: ExcludeUndefined<ThemingConfig['tokens']>['radii'] = {
  none: { value: '0' },
  xs: { value: '6px' },
  sm: { value: '6px' },
  base: { value: '6px' },
  md: { value: '8px' },
  lg: { value: '8px' },
  xl: { value: '8px' },
  '2xl': { value: '8px' },
  full: { value: '999px' },
  control: { value: '6px' },
  card: { value: '8px' },
  pill: { value: '999px' },
  indicator: { value: '0.125rem' },
  scrollbar: { value: '1.25rem' },
};

export const borderWidths = {
  '0': { value: '0px' },
  '1': { value: '1px' },
  '2': { value: '2px' },
  '3': { value: '3px' },
  '8': { value: '8px' },
} as const;
