import type { ApiResource } from '../../types';
import type { PaxeerXArchiveAnchorsResponse, PaxeerXArchiveReceipt, PaxeerXArchiveReceiptsResponse } from 'types/api/paxeerXArchive';
import type { PaxeerXTxStatus } from 'types/api/paxeerXLists';
import type { PaxeerXTxSignature } from 'types/api/paxeerXSignature';

export const GENERAL_API_PAXEER_X_LISTS_RESOURCES = {
  paxeer_x_tx_status: {
    path: '/api/v2/transactions/:hash/status',
    pathParams: [ 'hash' as const ],
  },
  paxeer_x_tx_signature: {
    path: '/api/v2/transactions/:hash/signature',
    pathParams: [ 'hash' as const ],
  },
  paxeer_x_anchors: {
    path: '/api/v2/paxeer-x/archive/anchors',
    filterFields: [],
    paginated: true,
  },
  paxeer_x_receipts: {
    path: '/api/v2/paxeer-x/archive/receipts',
    filterFields: [],
    paginated: true,
  },
  paxeer_x_receipt: {
    path: '/api/v2/paxeer-x/archive/receipts/:id',
    pathParams: [ 'id' as const ],
  },
} satisfies Record<string, ApiResource>;

export type GeneralApiPaxeerXListsResourceName = `general:${ keyof typeof GENERAL_API_PAXEER_X_LISTS_RESOURCES }`;

export type GeneralApiPaxeerXListsResourcePayload<R extends GeneralApiPaxeerXListsResourceName> =
  R extends 'general:paxeer_x_tx_status' ? PaxeerXTxStatus :
    R extends 'general:paxeer_x_tx_signature' ? PaxeerXTxSignature :
      R extends 'general:paxeer_x_anchors' ? PaxeerXArchiveAnchorsResponse :
        R extends 'general:paxeer_x_receipts' ? PaxeerXArchiveReceiptsResponse :
          R extends 'general:paxeer_x_receipt' ? PaxeerXArchiveReceipt :
            never;
