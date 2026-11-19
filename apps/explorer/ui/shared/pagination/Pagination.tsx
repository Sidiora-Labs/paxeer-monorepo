import type { HTMLChakraProps } from '@chakra-ui/react';
import { Flex } from '@chakra-ui/react';
import React from 'react';

import type { PaginationParams } from './types';

import { Skeleton } from 'toolkit/chakra/skeleton';
import ScanPagination from 'ui/shared/scan/ScanPagination';

interface Props extends PaginationParams, Omit<HTMLChakraProps<'div'>, 'page' | 'direction'> {
  pageCount?: number;
  onLastPageClick?: () => void;
  showRows?: React.ReactNode;
  reserveSpace?: boolean;
}

const Pagination = (props: Props) => {
  const { showRows, page, hasPages, isLoading, isVisible, className, reserveSpace, ...rest } = props;

  if (!isVisible && !reserveSpace) {
    return null;
  }

  const showSkeleton = page === 1 && !hasPages && isLoading;

  return (
    <Flex alignItems="center" columnGap={ 3 } className={ className } data-pagination visibility={ isVisible ? undefined : 'hidden' }>
      <Skeleton loading={ showSkeleton }>
        <ScanPagination
          { ...rest }
          page={ page }
          hasPages={ hasPages }
          isLoading={ isLoading }
          isVisible={ isVisible || Boolean(reserveSpace) }
        />
      </Skeleton>
      { showRows }
    </Flex>
  );
};

export default React.memo(Pagination);
