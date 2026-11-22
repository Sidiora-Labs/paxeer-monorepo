import { fetchWithPolicy as fetch } from "@paxeer-network/paxeer-backend/core/net/fetch";
import { activePoll } from "@paxeer-network/paxeer-backend/core/net/realtime";
/**
 * UDF-compatible datafeed for TradingView — HyperPax / Sidiora candle stack.
 *
 * Historical bars (`getBars`) read catalog-listed per-symbol UDF REST endpoints
 * directly through data.feed.chainflowtrading.com, using the browser Origin.
 *
 * Live bars (`subscribeBars`) poll the same candle history endpoint for
 * every symbol every 2 seconds while the page is active.
 */

import type {
  IBasicDataFeed,
  LibrarySymbolInfo,
  ResolutionString,
  Bar,
  PeriodParams,
  HistoryCallback,
  DatafeedErrorCallback,
  OnReadyCallback,
  ResolveCallback,
  SubscribeBarsCallback,
  SearchSymbolsCallback,
} from "@paxeer-network/paxeer-backend/types/tradingview";
import { tvResolutionToKlineInterval } from "@paxeer-network/paxeer-backend/hooks/perps/useSidPaxMarketStream";

import {
  candleFeedUrl,
  getCandleFeedSymbols,
  normalizeFeedSymbol,
} from "@paxeer-network/paxeer-backend/core/feeds";

// Read the public gateway directly. App-domain /api routing must not be a
// dependency for charts. Browser Origin authorizes these keyless requests.

interface BarSubscription {
  cleanup: () => void;
}

function barSeriesKey(symbol: string, resolution: ResolutionString): string {
  const interval =
    tvResolutionToKlineInterval(String(resolution)) ?? String(resolution);
  return `${normalizeFeedSymbol(symbol)}:${interval}`;
}

export class PerpsDatafeed implements IBasicDataFeed {
  private lastBarCache: Record<string, Bar> = {};
  private subscribers: Record<string, BarSubscription> = {};

  onReady(callback: OnReadyCallback): void {
    // Matches the per-symbol candle servers' `/config.supported_resolutions`.
    // Kept in sync with `INTERVALS` in `Chart.tsx` and
    // `tvResolutionToKlineInterval` in `useSidPaxMarketStream.ts`.
    const DEFAULT_RESOLUTIONS = [
      "1",
      "2",
      "3",
      "5",
      "15",
      "30",
      "60",
      "240",
      "480",
      "D",
      "W",
    ] as ResolutionString[];

    Promise.resolve()
      .then(() => fetch(candleFeedUrl("config")))
      .then((r) => r.json())
      .then((cfg) => {
        setTimeout(
          () =>
            callback({
              supported_resolutions:
                cfg.supported_resolutions ?? DEFAULT_RESOLUTIONS,
              supports_marks: cfg.supports_marks ?? false,
              supports_time: cfg.supports_time ?? true,
              supports_timescale_marks: cfg.supports_timescale_marks ?? false,
            }),
          0,
        );
      })
      .catch(() => {
        setTimeout(
          () =>
            callback({
              supported_resolutions: DEFAULT_RESOLUTIONS,
              supports_marks: false,
              supports_time: true,
              supports_timescale_marks: false,
            }),
          0,
        );
      });
  }

  searchSymbols(
    userInput: string,
    _exchange: string,
    _symbolType: string,
    onResult: SearchSymbolsCallback,
  ): void {
    const matches = getCandleFeedSymbols()
      .filter((symbol) => symbol.includes(userInput.toUpperCase()))
      .map((symbol) => ({
        symbol,
        ticker: symbol,
        full_name: symbol,
        description: symbol,
        exchange: "HyperPax",
        type: "crypto",
      }));
    setTimeout(() => onResult(matches), 0);
  }

  resolveSymbol(
    symbolName: string,
    onResolve: ResolveCallback,
    onError: DatafeedErrorCallback,
  ): void {
    let url: string;
    try {
      url = candleFeedUrl("symbols", symbolName);
    } catch (error) {
      setTimeout(() => onError(String(error)), 0);
      return;
    }
    fetch(url)
      .then((r) => {
        if (!r.ok) throw new Error(`Symbol not found: ${symbolName}`);
        return r.json();
      })
      .then((data) => {
        const producerSymbol = data.ticker ?? data.name;
        if (
          data.s === "error" ||
          typeof producerSymbol !== "string" ||
          normalizeFeedSymbol(producerSymbol) !==
            normalizeFeedSymbol(symbolName)
        ) {
          throw new Error(`Producer has no candle feed for ${symbolName}`);
        }
        const symbolInfo: LibrarySymbolInfo = {
          name: data.name || symbolName,
          ticker: data.ticker || symbolName,
          description: data.description || symbolName,
          type: data.type || "crypto",
          session: data.session || "24x7",
          exchange: data.exchange || "HyperPax",
          listed_exchange: data.listed_exchange || "HyperPax",
          timezone: (data.timezone ||
            "Etc/UTC") as LibrarySymbolInfo["timezone"],
          format: data.format || "price",
          pricescale: data.pricescale || 100,
          minmov: data.minmov || 1,
          has_intraday: data.has_intraday !== false,
          has_daily: data.has_daily !== false,
          has_weekly_and_monthly: data.has_weekly_and_monthly !== false,
          supported_resolutions:
            data.supported_resolutions ||
            ([
              "1",
              "5",
              "15",
              "30",
              "60",
              "240",
              "D",
              "W",
            ] as ResolutionString[]),
          data_status: data.data_status || "streaming",
        };
        onResolve(symbolInfo);
      })
      .catch((err) =>
        onError(`Candle feed unavailable for ${symbolName}: ${err.message}`),
      );
  }

  getBars(
    symbolInfo: LibrarySymbolInfo,
    resolution: ResolutionString,
    periodParams: PeriodParams,
    onResult: HistoryCallback,
    onError: DatafeedErrorCallback,
  ): void {
    const { from, to, countBack } = periodParams;
    const seriesKey = barSeriesKey(
      symbolInfo.ticker ?? symbolInfo.name,
      resolution,
    );
    const qs = new URLSearchParams({
      symbol: symbolInfo.ticker ?? symbolInfo.name,
      resolution: String(resolution),
      from: String(from),
      to: String(to),
    });
    if (countBack) qs.set("countback", String(countBack));

    let url: string;
    try {
      url = candleFeedUrl("history", symbolInfo.ticker ?? symbolInfo.name, qs);
    } catch (error) {
      setTimeout(() => onError(String(error)), 0);
      return;
    }
    fetch(url)
      .then((r) => r.json())
      .then((data) => {
        if (data.s === "no_data") {
          onResult([], { noData: true, nextTime: data.nextTime });
          return;
        }
        if (data.s !== "ok") {
          onError(
            `Candle feed unavailable for ${symbolInfo.ticker ?? symbolInfo.name}: ${data.errmsg || data.error || data.s || "Producer has no candle feed"}`,
          );
          return;
        }

        const bars: Bar[] = [];
        const len = data.t?.length ?? 0;
        for (let i = 0; i < len; i++) {
          bars.push({
            time: data.t[i] * 1000,
            open: data.o[i],
            high: data.h[i],
            low: data.l[i],
            close: data.c[i],
            volume: data.v?.[i],
          });
        }

        if (bars.length > 0) {
          const lastBar = bars[bars.length - 1];
          const cached = this.lastBarCache[seriesKey];
          if (lastBar && (!cached || lastBar.time > cached.time)) {
            this.lastBarCache[seriesKey] = lastBar;
          }
        }

        onResult(bars, { noData: bars.length === 0 });
      })
      .catch((err) =>
        onError(
          `Candle feed unavailable for ${symbolInfo.ticker ?? symbolInfo.name}: ${err.message}`,
        ),
      );
  }

  subscribeBars(
    symbolInfo: LibrarySymbolInfo,
    resolution: ResolutionString,
    onTick: SubscribeBarsCallback,
    listenerGuid: string,
    _onResetCacheNeededCallback: () => void,
  ): void {
    const symbolKey = symbolInfo.ticker ?? symbolInfo.name;
    const seriesKey = barSeriesKey(symbolKey, resolution);

    const pushBar = (bar: Bar) => {
      const cached = this.lastBarCache[seriesKey];
      if (!cached || bar.time >= cached.time) {
        this.lastBarCache[seriesKey] = bar;
        onTick(bar);
      }
    };

    const poll = () => {
      const now = Math.floor(Date.now() / 1000);
      const qs = new URLSearchParams({
        symbol: symbolKey,
        resolution: String(resolution),
        from: String(now - 300),
        to: String(now + 1),
        countback: "1",
      });

      return fetch(candleFeedUrl("history", symbolKey, qs))
        .then((r) => r.json())
        .then((data) => {
          if (data.s !== "ok" || !data.t?.length) return;

          const idx = data.t.length - 1;
          pushBar({
            time: data.t[idx] * 1000,
            open: data.o[idx],
            high: data.h[idx],
            low: data.l[idx],
            close: data.c[idx],
            volume: data.v?.[idx],
          });
        })
        .catch(() => {});
    };

    const stopPoll = activePoll(poll, 2000);
    this.subscribers[listenerGuid] = {
      cleanup: stopPoll,
    };
  }

  unsubscribeBars(listenerGuid: string): void {
    const sub = this.subscribers[listenerGuid];
    if (sub) {
      sub.cleanup();
      delete this.subscribers[listenerGuid];
    }
  }
}
