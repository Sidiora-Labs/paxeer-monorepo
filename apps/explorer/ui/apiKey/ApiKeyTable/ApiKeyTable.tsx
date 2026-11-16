import React from 'react';

import type { ApiKeys, ApiKey } from 'types/api/account';

import { TableBody, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';

import ApiKeyTableItem from './ApiKeyTableItem';

interface Props {
  data?: ApiKeys;
  isLoading?: boolean;
  onEditClick: (item: ApiKey) => void;
  onDeleteClick: (item: ApiKey) => void;
  limit: number;
}

const ApiKeyTable = ({ data, isLoading, onDeleteClick, onEditClick, limit }: Props) => {
  return (
    <ResponsiveTableRegion label="API keys" minWidth="xl">
      <TableRoot minWidth="xl">
        <TableHeader>
          <TableRow>
            <TableColumnHeader>{ `API key token (limit ${ limit } keys)` }</TableColumnHeader>
            <TableColumnHeader width="28"></TableColumnHeader>
          </TableRow>
        </TableHeader>
        <TableBody>
          { data?.map((item, index) => (
            <ApiKeyTableItem
              key={ item.api_key + (isLoading ? index : '') }
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

export default ApiKeyTable;
