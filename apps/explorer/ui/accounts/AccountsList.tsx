import { Box } from '@chakra-ui/react';
import type BigNumber from 'bignumber.js';
import React from 'react';

import type { AddressesItem } from 'types/api/addresses';

import breakpoints from 'toolkit/theme/foundations/breakpoints';
import AddressesListItem from 'ui/addresses/AddressesListItem';
import AddressesTable from 'ui/addresses/AddressesTable';

type Props = {
  items: Array<AddressesItem>;
  totalSupply: BigNumber;
  pageStartIndex: number;
  isLoading: boolean;
};

const AccountsList = ({ items, totalSupply, pageStartIndex, isLoading }: Props) => (
  <Box containerType="inline-size" containerName="accounts" minW={ 0 }>
    <Box css={{ display: 'none', [`@container accounts (min-width: ${ breakpoints.lg })`]: { display: 'block' } }}>
      <AddressesTable items={ items } totalSupply={ totalSupply } pageStartIndex={ pageStartIndex } isLoading={ isLoading }/>
    </Box>
    <Box css={{ [`@container accounts (min-width: ${ breakpoints.lg })`]: { display: 'none' } }}>
      { items.map((item, index) => (
        <AddressesListItem
          key={ item.hash + (isLoading ? index : '') }
          item={ item }
          index={ pageStartIndex + index }
          totalSupply={ totalSupply }
          isLoading={ isLoading }
        />
      )) }
    </Box>
  </Box>
);

export default React.memo(AccountsList);
