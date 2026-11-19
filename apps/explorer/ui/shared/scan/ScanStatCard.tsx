import { chakra } from '@chakra-ui/react';
import React from 'react';

import { Skeleton } from 'toolkit/chakra/skeleton';
import { Stat } from 'toolkit/chakra/stat';
import { Hint } from 'toolkit/components/Hint/Hint';

export type ScanStatDeltaDirection = 'up' | 'down';

export interface ScanStatDelta {
  value: string;
  direction: ScanStatDeltaDirection;
}

export interface ScanStatCardProps {
  label: string;
  value: React.ReactNode;
  secondary?: React.ReactNode;
  delta?: ScanStatDelta;
  hint?: string;
  icon?: React.ReactNode;
  isLoading?: boolean;
  className?: string;
}

const ScanStatCard = ({ label, value, secondary, delta, hint, icon, isLoading, className }: ScanStatCardProps) => {
  const valueText = typeof value === 'string' || typeof value === 'number' ? String(value) : undefined;

  return (
    <Stat.Root variant="scan" className={ className } data-scan-stat={ label } minW={ 0 } h="100%">
      <Stat.Label h={ 4 } minH="fit-content" flexShrink={ 0 } maxW="full" flexWrap="nowrap" whiteSpace="nowrap" alignItems="flex-start">
        { icon }
        <chakra.span data-label minW={ 0 } whiteSpace="normal" lineClamp={ 2 } textOverflow="ellipsis" overflowWrap="anywhere" title={ label }>
          { label }
        </chakra.span>
        { hint && (
          <chakra.span position="relative" boxSize={ 4 } flexShrink={ 0 }>
            <Hint label={ hint } isLoading={ isLoading } boxSize={ 4 } position="absolute" top="50%" transform="translateY(-50%)"/>
          </chakra.span>
        ) }
      </Stat.Label>
      <Stat.ValueText mt="auto" minW={ 0 } maxW="100%">
        <Skeleton
          loading={ isLoading }
          data-value
          whiteSpace="nowrap"
          minW={ 0 }
          maxW="100%"
          truncate={ valueText !== undefined }
          title={ valueText }
        >
          { value }
        </Skeleton>
        { secondary !== undefined && secondary !== null && (
          <chakra.span data-secondary>({ secondary })</chakra.span>
        ) }
        { delta && <chakra.span data-delta={ delta.direction }>({ delta.value })</chakra.span> }
      </Stat.ValueText>
    </Stat.Root>
  );
};

export default React.memo(ScanStatCard);
