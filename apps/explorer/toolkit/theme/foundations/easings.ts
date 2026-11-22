import type { ThemingConfig } from '@chakra-ui/react';

import type { ExcludeUndefined } from 'types/utils';

export const easings: ExcludeUndefined<ThemingConfig['tokens']>['easings'] = {
  standard: { value: 'cubic-bezier(0.2, 0, 0, 1)' },
  emphasized: { value: 'cubic-bezier(0.2, 0, 0, 1)' },
  entrance: { value: 'cubic-bezier(0, 0, 0.2, 1)' },
  exit: { value: 'cubic-bezier(0.4, 0, 1, 1)' },
};

export default easings;
