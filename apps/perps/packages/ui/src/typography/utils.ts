import {
  Decimal,
  commify,
  getPrecisionByNumber,
  numberToHumanStyle,
  todpIfNeed,
} from "@paxeer-network/utils";

export type RoundingMode = number | "truncate";

export const parseNumber = (
  value: number | string,
  options: {
    rule?: "percentages" | "price" | "human";
    dp?: number;

    tick?: number;
    rm?: RoundingMode;
    padding?: boolean;

    abs?: boolean;
  } = {},
): string => {
  let {
    rule,
    dp,
    tick,
    rm = rule === "price" ? Decimal.ROUND_HALF_UP : Decimal.ROUND_DOWN,
    padding = true,
    abs,
  } = options;

  if (Number.isNaN(value)) {
    return "--";
  }

  dp = dp != null ? dp : tick ? getPrecisionByNumber(tick) : 2;

  if (rule === "human") {
    return numberToHumanStyle(
      typeof value === "number" ? value : Number(value),
      dp,
      // { padding }
    );
  }

  let d = new Decimal(value);

  if (abs) {
    d = d.abs();
  }

  if (rule === "percentages") {
    // return `${d.mul(100).toFixed(2)}%`;
    return rounding(d.mul(100), { dp, rm, padding }) + "%";
  }
  if (rule === "price" && tick != null) {
    if (!Number.isFinite(tick) || tick <= 0)
      throw new RangeError("Price tick must be positive");
    if (rm !== "truncate") d = d.div(tick).toDecimalPlaces(0, rm).mul(tick);
  }
  const truncatedNum = rounding(d, { dp, rm, padding });

  // if (truncate === "round") {
  //   if (padding) {
  //     truncatedNum = d.toFixed(dp, Decimal.ROUND_HALF_EVEN);
  //   } else {
  //     truncatedNum = d.todp(dp, Decimal.ROUND_HALF_EVEN).toString();
  //   }
  // } else {
  //   if (padding) {
  //     truncatedNum = d.toFixed(dp);
  //   } else {
  //     truncatedNum = d.todp(dp).toString();
  //   }
  // }

  if (rule === "price") {
    return commify(truncatedNum);
  }

  if (truncatedNum?.includes("e")) {
    // If dp is omitted, the return value will be unrounded and in normal notation.
    // https://mikemcl.github.io/decimal.js/#toFixed
    return new Decimal(truncatedNum)?.toFixed();
  }

  return truncatedNum;
};

function rounding(
  d: Decimal,
  options: { dp: number; rm: RoundingMode; padding: boolean },
): string {
  const { dp, rm, padding } = options;

  if (rm === "truncate") {
    return todpIfNeed(d.toString(), dp);
  }

  if (padding) {
    return d.toFixed(dp, rm);
  }

  return d.todp(dp, rm).toString();
}

export const NumberReg = /^([0-9]{1,}[.]?[0-9]*)/;

export function formatAddress(address: string, range?: [number, number]) {
  if (address === undefined || address === null) {
    return "";
  }
  const [start, end] = range ?? [6, 4];
  const reg = new RegExp(`^(.{${start}})(.*)(.{${end}})$`);
  return `${address.replace(reg, "$1...$3")}`;
}
