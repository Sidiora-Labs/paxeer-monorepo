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
};
