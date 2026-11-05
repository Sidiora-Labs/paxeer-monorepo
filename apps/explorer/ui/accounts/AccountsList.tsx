import { Box } from '@chakra-ui/react';
import type BigNumber from 'bignumber.js';
import React from 'react';

import type { AddressesItem } from 'types/api/addresses';

import AddressesListItem from 'ui/addresses/AddressesListItem';
import AddressesTable from 'ui/addresses/AddressesTable';

type Props = {
  items: Array<AddressesItem>;
  totalSupply: BigNumber;
  pageStartIndex: number;
  isLoading: boolean;
};

const AccountsList = ({ items, totalSupply, pageStartIndex, isLoading }: Props) => (
  <>
    <Box hideBelow="lg">
      <AddressesTable items={ items } totalSupply={ totalSupply } pageStartIndex={ pageStartIndex } isLoading={ isLoading }/>
    </Box>
    <Box hideFrom="lg">
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
  </>
);

export default AccountsList;
