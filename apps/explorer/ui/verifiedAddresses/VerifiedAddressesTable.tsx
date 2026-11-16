import React from 'react';

import type { TokenInfoApplication, VerifiedAddress } from 'types/api/account';

import { TableBody, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';

import VerifiedAddressesTableItem from './VerifiedAddressesTableItem';

interface Props {
  data: Array<VerifiedAddress>;
  applications: Array<TokenInfoApplication> | undefined;
  onItemAdd: (address: string) => void;
  onItemEdit: (address: string) => void;
  isLoading: boolean;
}

const VerifiedAddressesTable = ({ data, applications, onItemEdit, onItemAdd, isLoading }: Props) => {
  return (
    <TableRoot aria-label="Verified contract addresses">
      <TableHeader>
        <TableRow>
          <TableColumnHeader>Address</TableColumnHeader>
          <TableColumnHeader w="calc({spacing.40} + {spacing.2})" pr={ 1 }>Token info</TableColumnHeader>
          <TableColumnHeader w={ 9 } pl="0" aria-label="Actions"></TableColumnHeader>
          <TableColumnHeader w={ 40 }>Request status</TableColumnHeader>
          <TableColumnHeader w="{spacing.37.5}">Date</TableColumnHeader>
        </TableRow>
      </TableHeader>
      <TableBody>
        { data.map((item, index) => (
          <VerifiedAddressesTableItem
            key={ item.contractAddress + (isLoading ? index : '') }
            item={ item }
            application={ applications?.find(({ tokenAddress }) => tokenAddress.toLowerCase() === item.contractAddress.toLowerCase()) }
            onAdd={ onItemAdd }
            onEdit={ onItemEdit }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </TableRoot>
  );
};

export default React.memo(VerifiedAddressesTable);
