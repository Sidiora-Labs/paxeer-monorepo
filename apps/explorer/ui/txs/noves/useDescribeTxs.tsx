import { uniq, chunk } from 'es-toolkit';
import React from 'react';

import config from 'configs/app';
import type { ReturnType } from 'lib/api/useApiQueries';
import useApiQueries from 'lib/api/useApiQueries';

import type { TransactionFeedItem } from '../TxPriceUpdate';
import { isPriceUpdate } from '../TxPriceUpdate';

const feature = config.features.txInterpretation;

const translateEnabled = feature.isEnabled && feature.provider === 'noves';

export type TxsTranslationQuery = ReturnType<'general:noves_describe_txs'> | undefined;

export default function useDescribeTxs(
  items: Array<TransactionFeedItem> | undefined,
  viewAsAccountAddress: string | undefined,
  isPlaceholderData: boolean,
): TxsTranslationQuery {
  const enabled = translateEnabled && !isPlaceholderData;
  const chunks = React.useMemo(() => {
    if (!enabled) {
      return [];
    }

    const txsHash = items ? uniq(items.filter((item) => !isPriceUpdate(item)).map(({ hash }) => hash)) : [];
    return chunk(txsHash, 10);
  }, [ items, enabled ]);

  const query = useApiQueries(
    'general:noves_describe_txs',
    chunks.map((hashes) => {
      return {
        queryParams: {
          viewAsAccountAddress,
          hashes,
        },
      };
    }),
    { enabled },
  );

  return enabled ? query : undefined;
}
