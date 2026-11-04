import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';
import type { PaginationParams } from 'ui/shared/pagination/types';

import { formatScanTableCount } from 'ui/shared/scan/ScanTableCard';

export function getTokenTransferTitle(itemsNum: number, transfersCount?: number) {
  if (transfersCount === undefined) {
    return `${ itemsNum.toLocaleString() } token transfers on this page`;
  }

  return formatScanTableCount(itemsNum < transfersCount ?
    { kind: 'latest', value: transfersCount, itemsName: 'token transfers', shownValue: itemsNum } :
    { kind: 'total', value: transfersCount, itemsName: 'token transfers' });
}

export default function useTokenTransferPages(data: Array<TokenTransfer> | undefined, pagination: PaginationParams) {
  const [ rowsToShow, setRowsToShow ] = React.useState(50);
  const [ position, setPosition ] = React.useState({ serverPage: pagination.page, localPage: 0, page: pagination.page });
  const localPageCount = Math.max(1, Math.ceil((data?.length ?? 0) / rowsToShow));
  const current = position.serverPage === pagination.page ? position : { serverPage: pagination.page, localPage: 0, page: pagination.page };
  const localPage = Math.min(current.localPage, localPageCount - 1);
  const hasLocalNextPage = localPage < localPageCount - 1;

  const onNextPageClick = () => {
    if (pagination.isLoading) {
      return;
    }
    if (hasLocalNextPage) {
      setPosition({ serverPage: pagination.page, localPage: localPage + 1, page: current.page + 1 });
    } else if (pagination.hasNextPage) {
      setPosition({ serverPage: pagination.page + 1, localPage: 0, page: current.page + 1 });
      pagination.onNextPageClick();
    }
  };

  const onPrevPageClick = () => {
    if (pagination.isLoading) {
      return;
    }
    if (localPage > 0) {
      setPosition({ serverPage: pagination.page, localPage: localPage - 1, page: current.page - 1 });
    } else if (pagination.canGoBackwards) {
      setPosition({ serverPage: pagination.page - 1, localPage: Infinity, page: current.page - 1 });
      pagination.onPrevPageClick();
    }
  };

  const resetPage = () => {
    setPosition({ serverPage: 1, localPage: 0, page: 1 });
    if (pagination.page !== 1) {
      pagination.resetPage();
    }
  };

  const onRowsChange = (rows: number) => {
    setRowsToShow(rows);
    resetPage();
  };

  return {
    items: data?.slice(localPage * rowsToShow, (localPage + 1) * rowsToShow),
    rowsToShow,
    onRowsChange,
    pagination: {
      ...pagination,
      page: current.page,
      hasPages: pagination.hasPages || localPageCount > 1,
      hasNextPage: hasLocalNextPage || pagination.hasNextPage,
      canGoBackwards: localPage > 0 || pagination.canGoBackwards,
      isVisible: pagination.isVisible || localPageCount > 1,
      onNextPageClick,
      onPrevPageClick,
      resetPage,
    },
  };
}
