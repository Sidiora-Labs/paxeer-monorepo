import { TOKEN_INFO_APPLICATION } from 'mocks/account/verifiedAddresses';
import * as addressMock from 'mocks/address/address';
import { forContract } from 'mocks/address/counters';
import { base as tabCounters } from 'mocks/address/tabCounters';
import { unifiedAccount } from 'mocks/paxeerX/unifiedAccount';
import { base as stats } from 'mocks/stats';
import { tokenCounters, tokenInfo } from 'mocks/tokens/tokenInfo';

export const addressDetailFixtures = new Map<string, unknown>([
  [ '/api/v2/stats', stats ],
  [ `/api/v2/addresses/${ addressMock.hash }`, addressMock.contract ],
  [ `/api/v2/addresses/${ addressMock.hash }/counters`, forContract ],
  [ `/api/v2/addresses/${ addressMock.hash }/tabs-counters`, tabCounters ],
  [ `/api/v2/addresses/${ addressMock.hash }/unified`, unifiedAccount ],
  [ `/api/v2/addresses/${ addressMock.hash }/transactions`, { items: [], next_page_params: null } ],
  [ '/v1/trpc/names.getNamesByOwnerAddress', { result: { data: [] } } ],
  [ `/api/v1/chains/1/token-infos/${ addressMock.hash }`, TOKEN_INFO_APPLICATION.APPROVED ],
]);

export const tokenDetailFixtures = new Map<string, unknown>([
  [ '/api/v2/stats', stats ],
  [ `/api/v2/tokens/${ tokenInfo.address_hash }`, tokenInfo ],
  [ `/api/v2/tokens/${ tokenInfo.address_hash }/counters`, tokenCounters ],
  [ `/api/v2/tokens/${ tokenInfo.address_hash }/transfers`, { items: [], next_page_params: null } ],
  [ `/api/v2/addresses/${ tokenInfo.address_hash }`, { ...addressMock.token, hash: tokenInfo.address_hash } ],
]);

// The notifier serializes Decimal supplies with to_string, including these exact wire values.
export const tokenSupplyTranscript = [
  { total_supply: '100000000000000000000' },
  { total_supply: '9007199254740993000000000000000001' },
  { total_supply: '0' },
] as const;
