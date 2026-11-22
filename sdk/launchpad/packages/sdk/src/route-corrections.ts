import type { OperationName } from './operations.js';
import type { ServedOperationName } from './served-routes.js';

export const ROUTE_CORRECTIONS: Readonly<Partial<Record<OperationName, string>>> = Object.freeze({
  IF_REWARD_CLAIM: '/v2/me/rewards:prepare-claim',
} satisfies Partial<Record<ServedOperationName, `/v${1 | 2 | 3}/${string}`>>);
