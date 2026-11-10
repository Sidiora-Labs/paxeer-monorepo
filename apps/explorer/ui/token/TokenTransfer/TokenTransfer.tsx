import { Box } from '@chakra-ui/react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useRouter } from 'next/router';
import React from 'react';

import type { SocketMessage } from 'lib/socket/types';
import type { TokenInfo, TokenInstance } from 'types/api/token';

import type { ResourceError } from 'lib/api/resources';
import useGradualIncrement from 'lib/hooks/useGradualIncrement';
import useIsMobile from 'lib/hooks/useIsMobile';
import useIsMounted from 'lib/hooks/useIsMounted';
import useSocketChannel from 'lib/socket/useSocketChannel';
import useSocketMessage from 'lib/socket/useSocketMessage';
import DataListDisplay from 'ui/shared/DataListDisplay';
import Pagination from 'ui/shared/pagination/Pagination';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import { ScanShowRows, ScanTableCard, SCAN_ROWS_PER_PAGE } from 'ui/shared/scan';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';
import TokenAdvancedFilterLink from 'ui/token/TokenAdvancedFilterLink';
import TokenTransferList from 'ui/token/TokenTransfer/TokenTransferList';
import TokenTransferTable from 'ui/token/TokenTransfer/TokenTransferTable';

import useTokenTransferPages, { getTokenTransferTitle } from './useTokenTransferPages';

type Props = {
  transfersQuery: QueryWithPagesResult<'general:token_transfers'> | QueryWithPagesResult<'general:token_instance_transfers'>;
  tokenId?: string;
  tokenInstance?: TokenInstance;
  tokenQuery: UseQueryResult<TokenInfo, ResourceError<unknown>>;
  shouldRender?: boolean;
  transfersCount?: number;
};

const TokenTransfer = ({ transfersQuery, tokenId, tokenQuery, tokenInstance, shouldRender = true, transfersCount }: Props) => {
  const isMobile = useIsMobile();
  const isMounted = useIsMounted();
  const router = useRouter();
  const { isError, isPlaceholderData, data, pagination } = transfersQuery;
  const { data: token, isPlaceholderData: isTokenPlaceholderData, isError: isTokenError } = tokenQuery;

  const [ newItemsCount, setNewItemsCount ] = useGradualIncrement(0);
  const [ showSocketErrorAlert, setShowSocketErrorAlert ] = React.useState(false);
  const { items, rowsToShow, onRowsChange, pagination: displayPagination } = useTokenTransferPages(data?.items, pagination);

  const handleNewTransfersMessage: SocketMessage.TokenTransfers['handler'] = (payload) => {
    setNewItemsCount(payload.token_transfer);
  };

  const handleSocketClose = React.useCallback(() => {
    setShowSocketErrorAlert(true);
  }, []);

  const handleSocketError = React.useCallback(() => {
    setShowSocketErrorAlert(true);
  }, []);

  const channel = useSocketChannel({
    topic: `tokens:${ router.query.hash?.toString().toLowerCase() }`,
    onSocketClose: handleSocketClose,
    onSocketError: handleSocketError,
    isDisabled: isPlaceholderData || isError || displayPagination.page !== 1,
  });
  useSocketMessage({
    channel,
    event: 'token_transfer',
    handler: handleNewTransfersMessage,
  });

  if (!isMounted || !shouldRender) {
    return null;
  }

  const isLoading = isPlaceholderData || isTokenPlaceholderData;

  const content = items && token ? (
    <>
      <Box display={{ base: 'none', lg: 'block' }}>
        <TokenTransferTable
          data={ items }
          top={ 0 }
          showSocketInfo={ displayPagination.page === 1 }
          showSocketErrorAlert={ showSocketErrorAlert }
          socketInfoNum={ newItemsCount }
          tokenId={ tokenId }
          token={ token }
          instance={ tokenInstance }
          isLoading={ isLoading }
        />
      </Box>
      <Box display={{ base: 'block', lg: 'none' }}>
        { displayPagination.page === 1 && (
          <SocketNewItemsNotice.Mobile
            num={ newItemsCount }
            showErrorAlert={ showSocketErrorAlert }
            type="token_transfer"
            isLoading={ isLoading }
          />
        ) }
        <TokenTransferList data={ items } tokenId={ tokenId } instance={ tokenInstance } isLoading={ isLoading }/>
      </Box>
    </>
  ) : null;

  const itemsNum = items?.length ?? 0;
  const title = getTokenTransferTitle(itemsNum, transfersCount);

  return (
    <ScanTableCard
      title={ title }
      actions={ !isMobile ? <TokenAdvancedFilterLink token={ token } isLoading={ isLoading }/> : null }
      pagination={ displayPagination.isVisible ? <Pagination { ...displayPagination }/> : null }
      showRows={ (
        <ScanShowRows
          value={ rowsToShow }
          onValueChange={ onRowsChange }
          options={ SCAN_ROWS_PER_PAGE }
          label="Show"
          suffix="Records"
          isLoading={ isLoading }
        />
      ) }
    >
      <DataListDisplay
        isError={ isError || isTokenError }
        itemsNum={ itemsNum }
        emptyText="There are no token transfers."
      >
        { content }
      </DataListDisplay>
    </ScanTableCard>
  );
};

export default React.memo(TokenTransfer);
