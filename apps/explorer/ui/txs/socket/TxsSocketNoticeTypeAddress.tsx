import React from 'react';

import type { TxsSocketNoticePlace } from './types';

import { TableCell, TableRow } from 'toolkit/chakra/table';
import SocketNewItemsNotice, { Mobile } from 'ui/shared/SocketNewItemsNotice';

import useTxsSocketTypeAddress from './useTxsSocketTypeAddress';

interface Props {
  place: TxsSocketNoticePlace;
  isLoading?: boolean;
  columnCount?: number;
}

const TxsSocketNoticeTypeAddress = ({ place, isLoading, columnCount }: Props) => {
  const { num, showErrorAlert } = useTxsSocketTypeAddress({ isLoading });

  if (num === undefined) {
    return null;
  }

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
      />
    );
  }
};

export default React.memo(TxsSocketNoticeTypeAddress);
