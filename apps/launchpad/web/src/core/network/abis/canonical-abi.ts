import type feesRouterAbi from '../../../../../../../precompiles/launchpad/abi/FeesRouter.json';
import type quoterAbi from '../../../../../../../precompiles/launchpad/abi/Quoter.json';
import type routerAbi from '../../../../../../../precompiles/launchpad/abi/Router.json';

type AbiEntry =
  | (typeof feesRouterAbi)[number]
  | (typeof quoterAbi)[number]
  | (typeof routerAbi)[number];

export function canonicalAbi(abi: readonly AbiEntry[]): string[] {
  return abi.map((entry) => JSON.stringify(entry, (_key, value: unknown) => {
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      return Object.fromEntries(
        Object.entries(value).sort(([left], [right]) => left.localeCompare(right)),
      );
    }
    return value;
  })).sort();
}
