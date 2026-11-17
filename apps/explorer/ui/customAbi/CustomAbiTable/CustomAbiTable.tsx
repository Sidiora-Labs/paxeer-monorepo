import React from 'react';

import type { CustomAbis, CustomAbi } from 'types/api/account';

import { TableBody, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';

import CustomAbiTableItem from './CustomAbiTableItem';

interface Props {
  data?: CustomAbis;
  isLoading?: boolean;
  onEditClick: (item: CustomAbi) => void;
  onDeleteClick: (item: CustomAbi) => void;
}

const CustomAbiTable = ({ data, isLoading, onDeleteClick, onEditClick }: Props) => {
  return (
    <ResponsiveTableRegion label="Custom ABIs" minWidth="xl">
      <TableRoot minWidth="xl">
        <TableHeader>
          <TableRow>
            <TableColumnHeader>ABI for Smart contract address (0x...)</TableColumnHeader>
            <TableColumnHeader width="28"></TableColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          { data?.map((item, index) => (
            <CustomAbiTableItem
              key={ item.id + (isLoading ? String(index) : '') }
              item={ item }
              isLoading={ isLoading }
              onDeleteClick={ onDeleteClick }
              onEditClick={ onEditClick }
            />
          )) }
        </TableBody>
      </TableRoot>
    </ResponsiveTableRegion>
  );
};

export default React.memo(CustomAbiTable);
