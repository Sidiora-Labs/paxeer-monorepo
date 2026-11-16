import { Box, chakra, Flex } from '@chakra-ui/react';
import React from 'react';

import { IconButton } from 'toolkit/chakra/icon-button';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tooltip } from 'toolkit/chakra/tooltip';
import IconSvg from 'ui/shared/IconSvg';

interface Props {
  className?: string;
  onClick: (direction: 'prev' | 'next') => void;
  prevLabel?: string;
  nextLabel?: string;
  isPrevDisabled?: boolean;
  isNextDisabled?: boolean;
  isLoading?: boolean;
}

const PrevNext = ({ className, onClick, prevLabel, nextLabel, isPrevDisabled, isNextDisabled, isLoading }: Props) => {
  const handelPrevClick = React.useCallback(() => {
    onClick('prev');
  }, [ onClick ]);

  const handelNextClick = React.useCallback(() => {
    onClick('next');
  }, [ onClick ]);

  if (isLoading) {
    return (
      <Flex columnGap={ 1 } className={ className }>
        <Skeleton loading boxSize="touchTarget" borderRadius="sm"/>
        <Skeleton loading boxSize="touchTarget" borderRadius="sm"/>
      </Flex>
    );
  }

  return (
    <Box className={ className } display="flex" data-prev-next>
      <Tooltip content={ prevLabel }>
        <IconButton
          aria-label="prev"
          borderRadius="sm"
          variant="scan_control"
          boxSize="touchTarget"
          minW="touchTarget"
          minH="touchTarget"
          onClick={ handelPrevClick }
          disabled={ isPrevDisabled }
          data-control="prev"
        >
          <IconSvg name="arrows/east-mini"/>
        </IconButton>
      </Tooltip>
      <Tooltip content={ nextLabel }>
        <IconButton
          aria-label="next"
          borderRadius="sm"
          variant="scan_control"
          boxSize="touchTarget"
          minW="touchTarget"
          minH="touchTarget"
          ml={ 1 }
          onClick={ handelNextClick }
          disabled={ isNextDisabled }
          data-control="next"
        >
          <IconSvg name="arrows/east-mini" transform="rotate(180deg)"/>
        </IconButton>
      </Tooltip>
    </Box>
  );
};

export default chakra(PrevNext);
