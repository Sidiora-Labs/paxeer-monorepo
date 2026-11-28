import assert from "node:assert/strict";
import test from "node:test";
import {
  mergePositionDetails,
  oneWaveAtATime,
  positionDetailFields,
  positionMarginRate,
  readPositionDetails,
  rowFundingFee,
} from "../packages/paxeer-backend/src/core/perps/positionDetails";

function theRateComesFromTheLeverageFirst() {
  assert.equal(positionMarginRate(2, 40, 80), 0.5);
  assert.equal(positionMarginRate(1, 4000, 80), 1);
  assert.equal(positionMarginRate(0, 40, 80), 0.5);
  assert.equal(positionMarginRate(NaN, 40, 80), 0.5);
  assert.equal(positionMarginRate(4, 0, 0), 0.25);
  assert.equal(positionMarginRate(0, 0, 80), 0);
  assert.equal(positionMarginRate(0, 40, 0), 0);
  assert.equal(positionMarginRate(Infinity, 0, 80), 0);
  assert.equal(positionMarginRate(0, Infinity, 80), 0);
  assert.equal(positionMarginRate(0, -40, 80), 0);
}

test(theRateComesFromTheLeverageFirst);

function theFeeOfARowIsTheFundingOwedWithItsSignTurned() {
  assert.equal(rowFundingFee(-0.25), 0.25);
  assert.equal(rowFundingFee(0.5), -0.5);
  for (const value of [undefined, NaN, Infinity, -Infinity])
    assert.equal(rowFundingFee(value), undefined);
  for (const value of [0, -0]) assert.ok(Object.is(rowFundingFee(value), 0));
}

test(theFeeOfARowIsTheFundingOwedWithItsSignTurned);

function theFieldsOfAnAnswerLeaveOutAPriceOfZero() {
  assert.deepEqual(
    positionDetailFields({
      liquidationPrice: 3000000000000000000n,
      fundingOwedUsd: -250000000000000000n,
      positionId: 1n,
    }),
    {
      liquidationPrice: "3000000000000000000",
      fundingOwedUsd: "-250000000000000000",
    },
  );
  assert.deepEqual(
    positionDetailFields({ liquidationPrice: 0n, fundingOwedUsd: 0n }),
    { fundingOwedUsd: "0" },
  );
  for (const answer of [
    null,
    undefined,
    5,
    {},
    { liquidationPrice: 3, fundingOwedUsd: -0.25 },
  ])
    assert.deepEqual(positionDetailFields(answer), {});
  assert.deepEqual(positionDetailFields({ liquidationPrice: -1n }), {});
}

test(theFieldsOfAnAnswerLeaveOutAPriceOfZero);

async function theReadsRunOneAfterTheOtherAndAFailedReadKeepsTheEarlierEntry() {
  const events: string[] = [];
  const ids = ["1", "2", "3", "4"];
  const previous = {
    "2": { liquidationPrice: "200", fundingOwedUsd: "-20" },
    "9": { liquidationPrice: "900" },
  };
  function read(id: string): Promise<unknown> {
    events.push(`start ${id}`);
    if (id === "3") {
      events.push(`end ${id}`);
      throw new Error("Position details unavailable");
    }
    return new Promise((resolve, reject) => {
      setTimeout(() => {
        events.push(`end ${id}`);
        if (id === "2") reject(new Error("Position details unavailable"));
        else resolve({ liquidationPrice: BigInt(id), fundingOwedUsd: 0n });
      }, 0);
    });
  }
  const expectedEvents = ids.flatMap((id) => [`start ${id}`, `end ${id}`]);
  const details = await readPositionDetails(ids, read, previous);
  assert.deepEqual(events, expectedEvents);
  assert.deepEqual(details, {
    "1": { liquidationPrice: "1", fundingOwedUsd: "0" },
    "2": previous["2"],
    "4": { liquidationPrice: "4", fundingOwedUsd: "0" },
  });
  assert.equal(details["2"], previous["2"]);
  assert.deepEqual(previous, {
    "2": { liquidationPrice: "200", fundingOwedUsd: "-20" },
    "9": { liquidationPrice: "900" },
  });
  events.length = 0;
  assert.deepEqual(await readPositionDetails(ids, read), {
    "1": { liquidationPrice: "1", fundingOwedUsd: "0" },
    "4": { liquidationPrice: "4", fundingOwedUsd: "0" },
  });
  assert.deepEqual(events, expectedEvents);
  events.length = 0;
  assert.deepEqual(await readPositionDetails([], read, previous), {});
  assert.deepEqual(events, []);
}

test(theReadsRunOneAfterTheOtherAndAFailedReadKeepsTheEarlierEntry);

function theMergeAddsTheFieldsAndLeavesTheOtherPositions() {
  const first = { positionId: "1", marketId: 7 };
  const second = { positionId: 2 };
  const third = { positionId: "3" };
  const positions = [first, second, third];
  const details = {
    "1": {
      liquidationPrice: "3000000000000000000",
      fundingOwedUsd: "-250000000000000000",
    },
    "2": { fundingOwedUsd: "0" },
  };
  const merged = mergePositionDetails(positions, details);
  assert.notEqual(merged, positions);
  assert.equal(merged.length, 3);
  assert.deepEqual(merged, [
    { ...first, ...details["1"] },
    { ...second, ...details["2"] },
    third,
  ]);
  assert.notEqual(merged[0], first);
  assert.notEqual(merged[1], second);
  assert.equal(Object.hasOwn(merged[1], "liquidationPrice"), false);
  assert.equal(merged[2], third);
  assert.equal(positions[0], first);
  assert.equal(positions[1], second);
  assert.equal(positions[2], third);
  assert.deepEqual(positions, [
    { positionId: "1", marketId: 7 },
    { positionId: 2 },
    { positionId: "3" },
  ]);
  for (const position of positions) {
    assert.equal(Object.hasOwn(position, "liquidationPrice"), false);
    assert.equal(Object.hasOwn(position, "fundingOwedUsd"), false);
  }
}

test(theMergeAddsTheFieldsAndLeavesTheOtherPositions);

async function onlyOneWaveRunsAndOneMoreFollows() {
  let starts = 0;
  const settlements: { resolve: () => void; reject: (error: Error) => void }[] =
    [];
  const request = oneWaveAtATime(() => {
    starts++;
    return new Promise<void>((resolve, reject) => {
      settlements.push({ resolve, reject });
    });
  });
  const nextTurn = () => new Promise<void>((resolve) => setTimeout(resolve, 0));
  assert.equal(request(), undefined);
  assert.equal(starts, 1);
  request();
  request();
  request();
  assert.equal(starts, 1);
  settlements[0].resolve();
  await nextTurn();
  assert.equal(starts, 2);
  settlements[1].resolve();
  await nextTurn();
  assert.equal(starts, 2);
  request();
  assert.equal(starts, 3);
  request();
  settlements[2].reject(new Error("Position detail wave unavailable"));
  await nextTurn();
  assert.equal(starts, 4);
  settlements[3].resolve();
  await nextTurn();
  assert.equal(starts, 4);
  const throwsAtCall = oneWaveAtATime(() => {
    throw new Error("Position detail wave unavailable");
  });
  assert.doesNotThrow(() => assert.equal(throwsAtCall(), undefined));
  await nextTurn();
}

test(onlyOneWaveRunsAndOneMoreFollows);
