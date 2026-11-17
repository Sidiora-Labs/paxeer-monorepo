import { Grid, chakra, Flex } from '@chakra-ui/react';
import React from 'react';

import type { ItemsProps } from './types';
import type { SearchResultUserOp } from 'types/api/search';

import * as UserOpEntity from 'ui/shared/entities/userOp/UserOpEntity';
import HashStringShortenDynamic from 'ui/shared/HashStringShortenDynamic';
import Time from 'ui/shared/time/Time';

import { resultLayout } from './resultLayout';

const SearchBarSuggestUserOp = ({ data }: ItemsProps<SearchResultUserOp>) => {
  const icon = <UserOpEntity.Icon/>;
  const hash = (
    <chakra.mark overflow="hidden" whiteSpace="nowrap" fontWeight="bold">
      <HashStringShortenDynamic
        fontFamily="mono"
        fontVariantNumeric="tabular-nums"
        hash={ data.user_operation_hash }
        noTooltip
      />
    </chakra.mark>
  );

  return (
    <Grid css={ resultLayout } gap={ 2 }>
      <Flex alignItems="center" minW={ 0 }>
        { icon }
        { hash }
      </Flex>
      <Time timestamp={ data.timestamp } color="text.secondary" textAlign="end" flexShrink={ 0 } ml="auto" format="lll_s"/>
    </Grid>
  );
};

export default React.memo(SearchBarSuggestUserOp);
