import { Box, chakra } from '@chakra-ui/react';
import React from 'react';

import type { FormattedData } from './types';

import * as mixpanel from 'lib/mixpanel/index';
import { Button } from 'toolkit/chakra/button';
import { space, thinsp } from 'toolkit/utils/htmlEntities';
import IconSvg from 'ui/shared/IconSvg';

import { getTokensTotalInfo } from '../utils/tokenUtils';

interface Props {
  isOpen: boolean;
  isLoading?: boolean;
  data: FormattedData;
}

const TokenSelectButton = ({ isOpen, isLoading, data, ...rest }: Props, ref: React.ForwardedRef<HTMLButtonElement>) => {
  const { usd, num, isOverflow } = getTokensTotalInfo(data);

  const prefix = isOverflow ? ` >${ thinsp }` : '';

  const handleClick = React.useCallback(() => {
    if (isLoading && !isOpen) {
      return;
    }

    mixpanel.logEvent(mixpanel.EventTypes.PAGE_WIDGET, { Type: 'Tokens dropdown' });
  }, [ isLoading, isOpen ]);

  return (
    <Box position="relative" className="group" minW={ 0 } maxW="100%">
      <Button
        ref={ ref }
        size="sm"
        variant="dropdown"
        onClick={ handleClick }
        gap={ 0 }
        minW={ 0 }
        maxW="100%"
        flexWrap="wrap"
        h="auto"
        py={ 1.5 }
        aria-label="Token select"
        loadingSkeleton={ isLoading && !isOpen }
        { ...rest }
      >
        <IconSvg name="tokens" boxSize={ 4 } mr={ 2 }/>
        <chakra.span fontWeight="semibold">{ prefix }{ num }</chakra.span>
        <chakra.span
          whiteSpace="pre"
          color={ isOpen ? 'inherit' : 'text.secondary' }
          fontWeight="normal"
          minW={ 0 }
          maxW="100%"
          flex="1"
          _groupHover={{ color: 'inherit' }}
          overflow="hidden"
          textOverflow="ellipsis"
        >
          { space }({ prefix }${ usd.toFormat(2) })
        </chakra.span>
        <IconSvg name="arrows/east-mini" transform={ isOpen ? 'rotate(90deg)' : 'rotate(-90deg)' } transitionDuration="faster" boxSize={ 5 } ml={ 3 }/>
      </Button>
    </Box>
  );
};

export default React.forwardRef(TokenSelectButton);
