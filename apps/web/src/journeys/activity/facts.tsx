"use client";

import { copyEntry } from "../../../copy/runtime.ts";
import { formatCopy } from "../../../copy/format.ts";
import type { ActivityEntryDetail, Money, MoneyDirection } from "../../api/index.ts";
import { LabelValue, SignedWordedAmount } from "../../kit/money";
import { PrivateFigure } from "../../settings/privacy";
import { formatEntryTimestamp, plainSentence, signedBaseUnits, toKitDirection } from "./model.ts";

export function ActivityDetailAmount({ money, direction, fee = false }: Readonly<{
  money: Money;
  direction?: MoneyDirection;
  fee?: boolean;
}>) {
  const value = fee ? signedBaseUnits(money, "out") : signedBaseUnits(money, direction);
  return (
    <span title={formatCopy("move.amount.hint", { currency: money.currency })}>
      <SignedWordedAmount
        value={value}
        currency={money.currency}
        locale="en"
        decimals={0}
        direction={fee ? "outbound" : toKitDirection(direction)}
      />
    </span>
  );
}

export function ActivityDetailFacts({ detail }: Readonly<{ detail: ActivityEntryDetail }>) {
  return (
    <section aria-labelledby="activity-what-happened" className="flex flex-col gap-3">
      <h2 id="activity-what-happened" className="text-lg font-bold text-foreground">{copyEntry("activity.detail.what_happened").message}</h2>
      <p className="text-foreground-secondary">{plainSentence(detail)}</p>
      <div className="grid gap-4 sm:grid-cols-3">
        <LabelValue
          label={detail.money === undefined ? copyEntry("activity.detail.amount").message : formatCopy("move.amount.hint", { currency: detail.money.currency })}
          value={detail.money === undefined ? copyEntry("activity.detail.no_amount").message : (
            <PrivateFigure><ActivityDetailAmount money={detail.money} {...(detail.direction === undefined ? {} : { direction: detail.direction })} /></PrivateFigure>
          )}
        />
        <LabelValue
          label={copyEntry("activity.detail.fee").message}
          value={detail.fees === undefined ? copyEntry("activity.detail.no_fee").message : (
            <PrivateFigure><ActivityDetailAmount money={detail.fees} fee /></PrivateFigure>
          )}
        />
        <LabelValue label={copyEntry("activity.detail.when").message} value={formatEntryTimestamp(detail.occurred_at)} />
      </div>
    </section>
  );
}
