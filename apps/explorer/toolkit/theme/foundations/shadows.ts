import type { ThemingConfig } from '@chakra-ui/react';

import type { ExcludeUndefined } from 'types/utils';

const shadows: ExcludeUndefined<ThemingConfig['tokens']>['shadows'] = {
  action_bar: { value: '0 0 0 1px {colors.hairline}' },
  // Cards carry an explicit border; other elevations reduce to a hairline.
  card: { value: 'none' },
  overlay: { value: '0 0 0 1px {colors.hairline}' },
  size: {
    xs: { value: '0 0 0 1px {colors.hairline}' },
    sm: { value: '0 0 0 1px {colors.hairline}' },
    base: { value: '0 0 0 1px {colors.hairline}' },
    md: { value: '0 0 0 1px {colors.hairline}' },
    lg: { value: '0 0 0 1px {colors.hairline}' },
    xl: { value: '0 0 0 1px {colors.hairline}' },
    '2xl': { value: '0 0 0 1px {colors.hairline}' },
  },
  'dark-lg': { value: '0 0 0 1px {colors.hairline}' },
};

export default shadows;
