import { chakra, Text, Flex } from '@chakra-ui/react';
import React from 'react';

import type { InteropMessage } from 'types/api/interop';

import { PopoverBody, PopoverCloseTriggerWrapper, PopoverContent, PopoverRoot, PopoverTrigger } from 'ui/marketplace/NativePopover';
import AdditionalInfoButton from 'ui/shared/AdditionalInfoButton';
import CopyToClipboard from 'ui/shared/CopyToClipboard';

type Props = {
  payload: InteropMessage['payload'];
  isLoading?: boolean;
  className?: string;
};

const InteropMessageAdditionalInfo = ({ payload, isLoading, className }: Props) => {
  return (
    <PopoverRoot autoFocus positioning={{ placement: 'right-start' }}>
      <PopoverTrigger>
        <AdditionalInfoButton loading={ isLoading } className={ className }/>
      </PopoverTrigger>
      <PopoverContent
        maxW="calc(100dvw - var(--chakra-spacing-8))"
        maxH="80dvh"
        overflowY="auto"
        overscrollBehavior="contain"
        w="calc(var(--chakra-spacing-1) * 82.5)"
      >
        <PopoverBody>
          <Flex alignItems="center" justifyContent="space-between" mb={ 3 }>
            <Text color="text.secondary" fontWeight="semibold">Message payload</Text>
            <PopoverCloseTriggerWrapper>
              <CopyToClipboard text={ payload }/>
            </PopoverCloseTriggerWrapper>
          </Flex>
          <Text fontFamily="mono" overflowWrap="anywhere">
            { payload }
          </Text>
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(chakra(InteropMessageAdditionalInfo));
