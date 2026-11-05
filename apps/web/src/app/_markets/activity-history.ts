import { paxeerHistory, unifiedHistory, type HistoryOptions, type HistoryPage } from "../../api/gateway.ts";

export type ActivitySide = "unified" | "paxeer";

export function activitySide(value: string | undefined): ActivitySide {
  return value === "paxeer" ? value : "unified";
}

/** Read the activity feed through the node-root history client. */
export async function activityHistory(
  side: ActivitySide,
  account: string,
  options: HistoryOptions,
): Promise<HistoryPage> {
  switch (side) {
    case "paxeer":
      return paxeerHistory(account, options);
    case "unified":
      return unifiedHistory(account, options);
  }
}
