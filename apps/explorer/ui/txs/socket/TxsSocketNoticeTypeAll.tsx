import React from 'react';

import type { TxsSocketNoticePlace, TxsSocketType } from './types';

import { route } from 'nextjs/routes';

import { useMultichainContext } from 'lib/contexts/multichain';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import SocketNewItemsNotice, { Mobile } from 'ui/shared/SocketNewItemsNotice';

import useNewTxsSocketTypeAll from './useTxsSocketTypeAll';

interface Props {
  type: TxsSocketType;
  place: TxsSocketNoticePlace;
  isLoading?: boolean;
  columnCount?: number;
}

const TxsSocketNoticeTypeAll = ({ type, place, isLoading, columnCount }: Props) => {
  const multichainContext = useMultichainContext();
  const { num, showErrorAlert } = useNewTxsSocketTypeAll({ type, isLoading });

  if (num === undefined) {
    return null;
  }

  const url = (() => {
    if (type === 'txs_home' && multichainContext) {
      return route({ pathname: '/txs', query: { tab: 'txs_local', chain_id: multichainContext.chain.id } });
    }
  })();

  if (place === 'table') {
    return (
      <SocketNewItemsNotice
        borderRadius={ isLoading ? 'sm' : 0 }
        h={ isLoading ? 5 : 'auto' }
        maxW={ isLoading ? '215px' : undefined }
        w="100%"
        mx={ isLoading ? 4 : 0 }
        my={ isLoading ? '6px' : 0 }
        showErrorAlert={ showErrorAlert }
        num={ num }
        isLoading={ isLoading }
        url={ url }
      >
        { ({ content }) => (
          <TableRow>
            <TableCell colSpan={ columnCount } p={ 0 } _first={{ p: 0 }} _last={{ p: 0 }}>{ content }</TableCell>
          </TableRow>
        ) }
      </SocketNewItemsNotice>
    );
  }

  if (place === 'list') {
    return (
      <Mobile
        num={ num }
        showErrorAlert={ showErrorAlert }
        isLoading={ isLoading }
        url={ url }
      />
    );
  }
};

export default React.memo(TxsSocketNoticeTypeAll);
