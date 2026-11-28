export interface PositionDetailFields {
  liquidationPrice?: string;
  fundingOwedUsd?: string;
}

export type PositionDetailsById = Record<string, PositionDetailFields>;

export function positionMarginRate(
  leverage: number,
  collateral: number,
  size: number,
): number {
  if (Number.isFinite(leverage) && leverage > 0) return 1 / leverage;
  const rate = size > 0 ? collateral / size : 0;
  return Number.isFinite(rate) && rate > 0 ? rate : 0;
}

export function rowFundingFee(fundingOwedUsd: number | undefined) {
  if (fundingOwedUsd === undefined || !Number.isFinite(fundingOwedUsd))
    return undefined;
  return fundingOwedUsd === 0 ? 0 : -fundingOwedUsd;
}

export function positionDetailFields(answer: unknown): PositionDetailFields {
  const fields: PositionDetailFields = {};
  if (typeof answer !== "object" || answer === null) return fields;
  if (
    "liquidationPrice" in answer &&
    typeof answer.liquidationPrice === "bigint" &&
    answer.liquidationPrice > 0n
  )
    fields.liquidationPrice = answer.liquidationPrice.toString();
  if ("fundingOwedUsd" in answer && typeof answer.fundingOwedUsd === "bigint")
    fields.fundingOwedUsd = answer.fundingOwedUsd.toString();
  return fields;
}

export async function readPositionDetails(
  ids: readonly string[],
  read: (id: string) => Promise<unknown>,
  previous: Readonly<PositionDetailsById> = {},
): Promise<PositionDetailsById> {
  const details: PositionDetailsById = {};
  for (const id of ids) {
    try {
      details[id] = positionDetailFields(await read(id));
    } catch {
      if (Object.hasOwn(previous, id)) details[id] = previous[id];
    }
  }
  return details;
}

export function mergePositionDetails<
  T extends { positionId: string | number | bigint },
>(
  positions: readonly T[],
  details: Readonly<PositionDetailsById>,
): (T & PositionDetailFields)[] {
  return positions.map((position) => {
    const id = String(position.positionId);
    return Object.hasOwn(details, id)
      ? { ...position, ...details[id] }
      : position;
  });
}

export function oneWaveAtATime(wave: () => Promise<void>): () => void {
  let running = false;
  let pending = false;
  async function run() {
    running = true;
    try {
      await wave();
    } catch {
    } finally {
      running = false;
      if (pending) {
        pending = false;
        void run();
      }
    }
  }
  return () => {
    if (running) pending = true;
    else void run();
  };
}
