export function marketLabel(
  marketId: bigint,
  catalog: readonly { readonly marketId: bigint; readonly symbol: string }[],
): string {
  return catalog.find((market) => market.marketId === marketId)?.symbol ?? marketId.toString(10);
}
