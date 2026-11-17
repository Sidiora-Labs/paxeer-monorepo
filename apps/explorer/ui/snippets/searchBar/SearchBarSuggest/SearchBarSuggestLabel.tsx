import { Grid, Text, Flex } from '@chakra-ui/react';
import React from 'react';

import type { ItemsProps } from './types';
import type { SearchResultLabel } from 'types/api/search';

import { toBech32Address } from 'lib/address/bech32';
import highlightText from 'lib/highlightText';
import HashStringShortenDynamic from 'ui/shared/HashStringShortenDynamic';
import IconSvg from 'ui/shared/IconSvg';

import { resultLayout } from './resultLayout';

const SearchBarSuggestLabel = ({ data, searchTerm, addressFormat }: ItemsProps<SearchResultLabel>) => {
  const icon = <IconSvg aria-hidden={ true } name="publictags" boxSize={ 5 } color="icon.primary"/>;
  const hash = data.filecoin_robust_address || (addressFormat === 'bech32' ? toBech32Address(data.address_hash) : data.address_hash);

  const name = (
    <Text
      fontWeight="bold"
      overflow="hidden"
      whiteSpace="nowrap"
      textOverflow="ellipsis"
    >
      <span dangerouslySetInnerHTML={{ __html: highlightText(data.name, searchTerm) }}/>
    </Text>
  );

  const address = (
    <Text
      overflow="hidden"
      whiteSpace="nowrap"
      color="text.secondary"
    >
      <HashStringShortenDynamic fontFamily="mono" fontVariantNumeric="tabular-nums" hash={ hash } noTooltip/>
    </Text>
  );

  const isContractVerified = data.is_smart_contract_verified && (
    <IconSvg
      aria-hidden={ true }
      name="status/success"
      boxSize={ 'var(--chakra-spacing-3\\.5)' }
      color="positive"
      flexShrink={ 0 }
    />
  );

  return (
    <Grid alignItems="center" css={ resultLayout } gap={ 2 }>
      <Flex alignItems="center" gap={ 2 }>
        { icon }
        { name }
      </Flex>
      <Flex alignItems="center" overflow="hidden" gap={ 1 }>
        { address }
        { isContractVerified }
      </Flex>
    </Grid>
  );
};

export default React.memo(SearchBarSuggestLabel);
