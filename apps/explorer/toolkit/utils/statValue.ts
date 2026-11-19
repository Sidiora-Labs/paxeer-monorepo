import type { SystemStyleObject } from '@chakra-ui/react';

import { fontSizes } from 'toolkit/theme/foundations/typography';

const steps = [ 'textMd', 'headingMd', 'textXl', 'statValue' ] as const;

export function getStatValueStyles(characters: number): SystemStyleObject {
  return {
    fontSize: 'body',
    ...Object.fromEntries(steps.map((step) => {
      const scale = parseFloat(fontSizes[step].value) / parseFloat(fontSizes.textMd.value);
      return [ `@container (min-width: ${ characters * scale }ch)`, { fontSize: step } ];
    })),
  };
}
