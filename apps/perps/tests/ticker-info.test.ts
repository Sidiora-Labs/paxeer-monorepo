import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  infoFieldsOfTicker,
  tickerOfRow,
} from "../packages/paxeer-backend/src/core/perps/priceWindow";

const root = fileURLToPath(new URL("../", import.meta.url));
const row = {
  symbol: "PERP_PAX_USDC",
  "24h_open": 100,
  "24h_close": 110,
  "24h_high": 120,
  "24h_low": 90,
  "24h_volume": 5,
  "24h_amount": 550,
  mark_price: 110,
  "24h_volumn": 5,
};

function theMessageOfOneMarketCarriesTheAmountOfItsRow() {
  assert.deepEqual(tickerOfRow(row), {
    symbol: "PERP_PAX_USDC",
    open: 100,
    close: 110,
    high: 120,
    low: 90,
    volume: 5,
    amount: 550,
  });
  assert.deepEqual(tickerOfRow({ symbol: row.symbol }), {
    symbol: "PERP_PAX_USDC",
    open: undefined,
    close: undefined,
    high: undefined,
    low: undefined,
    volume: undefined,
    amount: undefined,
  });
}

test(theMessageOfOneMarketCarriesTheAmountOfItsRow);

function aMessageSetsTheSevenFieldsOfTheInfo() {
  const fields = infoFieldsOfTicker(tickerOfRow(row));
  assert.deepEqual(fields, {
    "24h_open": 100,
    "24h_close": 110,
    "24h_high": 120,
    "24h_low": 90,
    "24h_volumn": 5,
    "24h_volume": 5,
    "24h_amount": 550,
  });
  assert.equal(Object.hasOwn(fields, "symbol"), false);
}

test(aMessageSetsTheSevenFieldsOfTheInfo);

function aValueThatIsUndefinedSetsNoField() {
  assert.deepEqual(infoFieldsOfTicker(tickerOfRow({ symbol: row.symbol })), {});
  assert.deepEqual(
    infoFieldsOfTicker({ symbol: row.symbol, volume: 0, amount: null }),
    { "24h_volumn": 0, "24h_volume": 0, "24h_amount": null },
  );
  for (const value of [null, undefined, 5, "ticker"])
    assert.deepEqual(infoFieldsOfTicker(value), {});
}

test(aValueThatIsUndefinedSetsNoField);

function aLaterMessageBringsTheAmountToAnInfoThatWasReadWithoutIt() {
  const info = { symbol: row.symbol, "24h_close": 100 };
  const emptyMessage = tickerOfRow({ symbol: row.symbol });
  const unchanged = Object.assign(
    { ...info },
    infoFieldsOfTicker(emptyMessage),
  );
  assert.equal(Object.hasOwn(unchanged, "24h_amount"), false);
  assert.equal(unchanged["24h_close"], 100);

  const updated = Object.assign(
    { ...info },
    infoFieldsOfTicker(
      tickerOfRow({
        symbol: row.symbol,
        "24h_close": 110,
        "24h_amount": 550,
      }),
    ),
  );
  assert.equal(updated["24h_amount"], 550);
  assert.equal(updated["24h_close"], 110);

  const existing = { ...info, "24h_amount": 7 };
  const preserved = Object.assign(
    { ...existing },
    infoFieldsOfTicker(emptyMessage),
  );
  assert.equal(preserved["24h_amount"], 7);
  assert.equal(existing["24h_amount"], 7);
  assert.deepEqual(info, { symbol: "PERP_PAX_USDC", "24h_close": 100 });
  assert.equal(Object.hasOwn(info, "24h_amount"), false);
}

test(aLaterMessageBringsTheAmountToAnInfoThatWasReadWithoutIt);

function theSocketAndTheStreamCallTheTwoFunctions() {
  const socket = readFileSync(
    join(root, "packages/hooks/src/native/NativeWebSocket.ts"),
    "utf8",
  );
  const stream = readFileSync(
    join(root, "packages/hooks/src/paxeer-network/useTickerStream.ts"),
    "utf8",
  );
  assert.match(socket, /\btickerOfRow\s*\(/);
  assert.match(stream, /\binfoFieldsOfTicker\s*\(/);
  for (const text of ["ticker.volume", "ticker.high", "ticker.low"])
    assert.equal(stream.includes(text), false, text);
}

test(theSocketAndTheStreamCallTheTwoFunctions);
