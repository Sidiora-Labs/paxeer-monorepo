import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { Transaction } from 'types/api/transaction';

import { Heading } from 'toolkit/chakra/heading';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'toolkit/chakra/popover';
import AdditionalInfoButton from 'ui/shared/AdditionalInfoButton';

import ActivityDialog from './ActivityDialog';
import TxAdditionalInfoContainer from './TxAdditionalInfoContainer';
import TxAdditionalInfoContent from './TxAdditionalInfoContent';

type Props =
  ({
    hash: string;
    tx?: undefined;
  } |
  {
    hash?: undefined;
    tx: Transaction;
  }) & {
    isMobile?: boolean;
    isLoading?: boolean;
    className?: string;
  };

const TxAdditionalInfo = ({ hash, tx, isMobile, isLoading, className }: Props) => {
  const content = hash !== undefined ? <TxAdditionalInfoContainer hash={ hash }/> : <TxAdditionalInfoContent tx={ tx }/>;

  if (isMobile) {
    return (
      <ActivityDialog
        title="Additional info"
        trigger={ <AdditionalInfoButton loading={ isLoading } className={ className } minW="touchTarget" minH="touchTarget"/> }
      >
        { content }
      </ActivityDialog>
    );
  }
  return (
    <PopoverRoot autoFocus positioning={{ placement: 'right-start' }}>
      <PopoverTrigger>
        <AdditionalInfoButton loading={ isLoading } className={ className } minW="touchTarget" minH="touchTarget"/>
      </PopoverTrigger>
      <PopoverContent
        aria-label="Additional info"
        css={{ '& button': { minW: 'touchTarget', minH: 'touchTarget' } }}
        maxH="min(var(--available-height, 100dvh), calc(100dvh - var(--chakra-spacing-8)))"
        overflowY="auto"
        overscrollBehavior="contain"
        w="var(--chakra-spacing-80)"
        maxW="calc(100dvw - var(--chakra-spacing-8))"
      >
        <PopoverBody>
          <Heading level="3" mb={ 4 }>Additional info </Heading>
          { content }
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(chakra(TxAdditionalInfo));
