import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { ScanTableCardProps } from 'ui/shared/scan';
import { ScanTableCard } from 'ui/shared/scan';

const StyledCard = chakra(ScanTableCard);

const AccountTableCard = (props: ScanTableCardProps) => (
  <StyledCard { ...props } css={{
    '& button': {
      minWidth: 'touchTarget',
      minHeight: 'touchTarget',
      _motionReduce: { transitionProperty: 'none' },
    },
    '& button[role="combobox"]': { focusVisibleRing: 'outside', focusRingColor: 'focusRing' },
  }}/>
);

export default React.memo(AccountTableCard);
