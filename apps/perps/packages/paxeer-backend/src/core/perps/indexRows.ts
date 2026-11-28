export function indexTimeMs(value: unknown): number | undefined {
  if (typeof value === "number" || typeof value === "bigint") {
    const milliseconds = Number(value) * 1000;
    return Number.isFinite(milliseconds) ? milliseconds : undefined;
  }
  if (typeof value !== "string") return undefined;
  if (/^\d+$/.test(value)) {
    const milliseconds = Number(value) * 1000;
    return Number.isFinite(milliseconds) ? milliseconds : undefined;
  }
  const parts =
    /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})$/.exec(
      value,
    );
  if (!parts) return undefined;
  const year = Number(parts[1]);
  const month = Number(parts[2]);
  const day = Number(parts[3]);
  const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const daysInMonth = [
    31,
    leapYear ? 29 : 28,
    31,
    30,
    31,
    30,
    31,
    31,
    30,
    31,
    30,
    31,
  ];
  const zone = parts[8];
  if (
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > daysInMonth[month - 1] ||
    Number(parts[4]) > 23 ||
    Number(parts[5]) > 59 ||
    Number(parts[6]) > 59 ||
    (zone !== "Z" &&
      (Number(zone.slice(1, 3)) > 23 || Number(zone.slice(4)) > 59))
  )
    return undefined;
  const milliseconds = Date.parse(
    `${value.slice(0, 19)}.${(parts[7] ?? "").slice(0, 3).padEnd(3, "0")}${zone}`,
  );
  return Number.isFinite(milliseconds) ? milliseconds : undefined;
}

export function quantityAtPrice(
  size: bigint,
  price: bigint,
): number | undefined {
  if (price <= 0n || size < 0n) return undefined;
  const rounded = (size * 100000000n + price / 2n) / price;
  return Number(rounded) / 100000000;
}

export function triggerIdFields<T>(type: string, id: T): { algo_order_id?: T } {
  return type === "LIMIT" || type === "MARKET" ? {} : { algo_order_id: id };
}

export function orderListVariables(
  userAddress: string,
  status: string | null,
): { userAddress: string; limit: 500; status?: "active" } {
  return status === "INCOMPLETE"
    ? { userAddress, status: "active", limit: 500 }
    : { userAddress, limit: 500 };
}
