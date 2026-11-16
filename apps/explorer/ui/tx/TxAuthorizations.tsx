import React from 'react';

import DataListDisplay from 'ui/shared/DataListDisplay';
import TxPendingAlert from 'ui/tx/TxPendingAlert';
import TxSocketAlert from 'ui/tx/TxSocketAlert';
import ActivityViews from 'ui/txs/ActivityViews';

import TxAuthorizationsList from './authorizations/TxAuthorizationsList';
import TxAuthorizationsTable from './authorizations/TxAuthorizationsTable';
import type { TxQuery } from './useTxQuery';

interface Props {
  txQuery: TxQuery;
}

const TxAuthorizations = ({ txQuery }: Props) => {

  if (!txQuery.isPlaceholderData && !txQuery.isError && !txQuery.data?.status) {
    return txQuery.socketStatus ? <TxSocketAlert status={ txQuery.socketStatus }/> : <TxPendingAlert/>;
  }

  const content = (
    <ActivityViews
      list={
        <TxAuthorizationsList data={ txQuery.data?.authorization_list } isLoading={ txQuery.isPlaceholderData }/>
      }
      table={
        <TxAuthorizationsTable data={ txQuery.data?.authorization_list } isLoading={ txQuery.isPlaceholderData }/>
      }
    />
  );

  return (
    <DataListDisplay
      isError={ txQuery.isError }
      itemsNum={ txQuery.data?.authorization_list?.length }
      emptyText="There are no authorizations for this transaction."
    >
      { content }
    </DataListDisplay>
  );
};

export default TxAuthorizations;
