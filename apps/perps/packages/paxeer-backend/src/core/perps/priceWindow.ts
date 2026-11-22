export function priceWindowChange(
  price: number | null | undefined,
  price24h: number | null | undefined,
): number | null {
  if (
    price == null ||
    price24h == null ||
    !Number.isFinite(price) ||
    !Number.isFinite(price24h) ||
    price <= 0 ||
    price24h <= 0
  )
    return null;
  return (price / price24h - 1) * 100;
}
