"use client";

import { useState } from "react";

import { copyEntry } from "../../../copy/runtime.ts";
import { KitOptionList } from "../../kit/display";
import { TextField } from "../../kit/field";
import { WalletFeeController, type WalletFeeSnapshot } from "./model.ts";

export function WalletFeeChoice({ snapshot, onChange }: Readonly<{
  snapshot: WalletFeeSnapshot;
  onChange: (currency: WalletFeeSnapshot["currency"], maximum: string) => void;
}>) {
  return (
    <fieldset className="flex flex-col gap-3 border border-border p-4">
      <legend>{copyEntry("gas.sidiora.title").message}</legend>
      <p>{copyEntry("gas.sidiora.scope").message}</p>
      <KitOptionList
        aria-label={copyEntry("gas.sidiora.title").message}
        value={snapshot.currency}
        items={[
          { value: "paxeer", label: copyEntry("gas.sidiora.paxeer").message },
          { value: "sidiora", label: copyEntry("gas.sidiora.choose").message },
        ]}
        onValueChange={(value) => { onChange(value === "sidiora" ? "sidiora" : "paxeer", snapshot.maximum); }}
      />
      {snapshot.currency === "sidiora" ? (
        <>
          <TextField
            label={copyEntry("gas.sidiora.maximum").message}
            inputMode="decimal"
            value={snapshot.maximum}
            errorMessage={snapshot.valid ? undefined : copyEntry("gas.sidiora.invalid").message}
            onChange={(event) => { onChange("sidiora", event.target.value); }}
          />
          <p>{copyEntry("gas.sidiora.review").message}</p>
        </>
      ) : null}
    </fieldset>
  );
}

export function WalletFeeSettings() {
  const [controller] = useState(() => new WalletFeeController());
  const [snapshot, setSnapshot] = useState(controller.snapshot);
  return <WalletFeeChoice snapshot={snapshot} onChange={(currency, maximum) => {
    setSnapshot(controller.choose(currency, maximum));
  }} />;
}
