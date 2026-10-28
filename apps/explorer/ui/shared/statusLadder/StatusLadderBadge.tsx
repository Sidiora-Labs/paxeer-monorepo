import { Box, Text } from '@chakra-ui/react';
import React from 'react';

import type { PaxeerXStatusRung } from 'types/api/paxeerXLists';
import { PAXEER_X_NATIVE_RUNGS } from 'types/api/paxeerXLists';

import type { BadgeProps } from 'toolkit/chakra/badge';
import { Badge } from 'toolkit/chakra/badge';
import { Tooltip } from 'toolkit/chakra/tooltip';
import IconSvg from 'ui/shared/IconSvg';

import { NATIVE_RUNGS, NATIVE_RUNG_ORDER, RUNGS, RUNG_ORDER } from './rungs';

export interface Props extends Omit<BadgeProps, 'children'> {
  rung: PaxeerXStatusRung;
  isLoading?: boolean;
}

const StatusLadderBadge = ({ rung, isLoading, ...rest }: Props) => {
  const descriptor = RUNGS[rung];

  const tooltipContent = (
    <Box display="flex" flexDirection="column" gap={ 1 }>
      { RUNG_ORDER.map((item) => (
        <Text key={ item.rung } textStyle="xs" fontWeight={ item.rung === rung ? 600 : 400 }>
          { item.label }: { item.description }
        </Text>
      )) }
    </Box>
  );

  return (
    <Tooltip content={ tooltipContent } contentProps={{ maxW: '320px' }}>
      <Badge
        colorPalette={ descriptor.colorPalette }
        loading={ isLoading }
        startElement={ <IconSvg name={ descriptor.icon } boxSize={ 2.5 } display="inline-block"/> }
        data-rung={ rung }
        { ...rest }
      >
        { descriptor.label }
      </Badge>
    </Tooltip>
  );
};

export default StatusLadderBadge;

export interface NativeSettlementBadgeProps extends Omit<Props, 'rung'> {
  rung: string;
}

export function NativeSettlementBadge({ rung, isLoading, ...rest }: NativeSettlementBadgeProps) {
  const nativeRung = PAXEER_X_NATIVE_RUNGS.find((item) => item === rung);

  if (!nativeRung) {
    return null;
  }

  const descriptor = NATIVE_RUNGS[nativeRung];
  const tooltipContent = (
    <Box display="flex" flexDirection="column" gap={ 1 }>
      { NATIVE_RUNG_ORDER.map((item) => (
        <Text key={ item.rung } textStyle="xs" fontWeight={ item.rung === rung ? 600 : 400 }>
          { item.label }: { item.description }
        </Text>
      )) }
    </Box>
  );

  return (
    <Tooltip content={ tooltipContent } contentProps={{ maxW: '320px' }}>
      <Badge
        colorPalette={ descriptor.colorPalette }
        loading={ isLoading }
        startElement={ <IconSvg name={ descriptor.icon } boxSize={ 2.5 } display="inline-block"/> }
        data-rung={ rung }
        { ...rest }
      >
        { descriptor.label }
      </Badge>
    </Tooltip>
  );
}
