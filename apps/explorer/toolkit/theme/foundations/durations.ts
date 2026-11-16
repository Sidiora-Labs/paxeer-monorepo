import type { ThemingConfig } from '@chakra-ui/react';

import type { ExcludeUndefined } from 'types/utils';

const durations = {
  chart: { value: '300ms' },
  loader: { value: '700ms' },
  toggle: { value: '120ms' },
  progress: { value: '600ms' },
  cycle: { value: '1000ms' },
  pulse: { value: '1200ms' },
  circular: { value: '1500ms' },
  shimmer: { value: '2000ms' },
  'ultra-fast': { value: '50ms' },
  faster: { value: '100ms' },
  fast: { value: '150ms' },
  normal: { value: '200ms' },
  slow: { value: '300ms' },
  slower: { value: '400ms' },
  'ultra-slow': { value: '500ms' },
} as const satisfies ExcludeUndefined<ThemingConfig['tokens']>['durations'];

export default durations;
