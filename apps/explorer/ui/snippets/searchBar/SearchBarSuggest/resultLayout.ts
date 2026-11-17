import type { SystemStyleObject } from '@chakra-ui/react';

import breakpoints from 'toolkit/theme/foundations/breakpoints';

const resultQuery = `@container search-results (min-width: ${ breakpoints.sm })` as const;

export const resultLayout: SystemStyleObject = {
  gridTemplateColumns: 'minmax(0, 1fr)',
  alignItems: 'center',
  minWidth: 0,
  '& > *': { minWidth: 0 },
};

resultLayout[resultQuery] = {
  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  '& > :nth-child(n + 3)': { gridColumn: '2', textAlign: 'end' },
};
