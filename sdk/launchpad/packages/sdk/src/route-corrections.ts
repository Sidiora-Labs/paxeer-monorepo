import type { OperationName } from './operations.js';

export const ROUTE_CORRECTIONS: Readonly<Partial<Record<OperationName, string>>> = Object.freeze({
  IF_REWARD_CLAIM: '/v2/me/rewards/claim',
});
