import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { copyEntries } from "../copy/catalog.ts";
import { formatCopy } from "../copy/format.ts";
import { runtimeMessages } from "../copy/messages.generated.ts";
import { copyEntry, human_copy_catalog } from "../copy/runtime.ts";
import { kindLabel, plainSentence } from "../src/journeys/activity/model.ts";

async function separate_interface_copy_uses_paxeer_x_network() {
  assert.equal(copyEntry("application.name").message, "Paxeer X Network");
  assert.equal(formatCopy("explorer.title"), "Paxeer X Network Explorer");
  assert.equal(
    formatCopy("explorer.summary"),
    "Look up public Paxeer X Network activity and verify its records.",
  );
  const product = copyEntries.find(({ key }) => key === "application.name");
  assert.ok(product);
  assert.match(product.context, /Accessible product name and application metadata/u);
  assert.equal(product.surface, "default");
  assert.equal(product.moneyAdjacent, false);

  // Check the actual metadata and onboarding consumers keep using the runtime key.
  const layout = await readFile(new URL("../src/app/layout.tsx", import.meta.url), "utf8");
  assert.match(layout, /title: copyEntry\("application.name"\)\.message/u);
  const onboarding = await readFile(
    new URL("../src/journeys/onboarding/onboarding.tsx", import.meta.url),
    "utf8",
  );
  assert.match(onboarding, /dataApplication=\{copyEntry\("application.name"\)\.message\}/u);
  assert.match(onboarding, /device: \{ label: copyEntry\("application.name"\)\.message, platform: "web" \}/u);
  assert.doesNotMatch(onboarding, /LayerX web app/u);
}

function split_balances_and_settlement_copy_distinguishes_native_from_archive() {
  assert.equal(formatCopy("explorer.account.total_balances"), "Total balances");
  assert.equal(formatCopy("explorer.column.total"), "Total balance");
  assert.equal(
    formatCopy("explorer.account.total_balances.table"),
    "One total balance per asset on Paxeer X Network",
  );
  assert.deepEqual(
    ["submitted", "receipted", "committed"].map((stage) => formatCopy(`explorer.commitment.${stage}`)),
    ["Submitted", "Receipted", "Committed"],
  );
  for (const block of ["0", "125", "9007199254740993"]) {
    assert.equal(
      formatCopy("explorer.commitment.committed.detail", { block }),
      `Receipt included in committed native block ${block}.`,
    );
  }
  assert.throws(
    () => formatCopy("explorer.commitment.committed.detail"),
    /Missing copy value: block/u,
  );
  for (const key of [
    "explorer.account.balances",
    "explorer.account.balances.table",
    "explorer.column.on_paxeer",
    "explorer.column.on_layerx",
    "explorer.column.in_custody",
    "explorer.account.settlement",
    "explorer.account.settlement.table",
    "explorer.settlement.instant",
    "explorer.settlement.sealed",
    "explorer.settlement.final",
    "explorer.account.paxeer_activity",
    "explorer.account.paxeer_activity.table",
    "explorer.account.event.custody_deposit",
    "explorer.account.event.emergency_exit",
    "explorer.anchors.title",
  ]) {
    assert.match(formatCopy(key), /^Archive\b/u, key);
  }
  assert.equal(
    formatCopy("explorer.settlement.instant.detail", { block: "125" }),
    "Archive: applied at block 125",
  );
  assert.equal(
    formatCopy("explorer.settlement.sealed.detail", { batch: "7" }),
    "Archive: sealed through batch 7",
  );
  assert.equal(
    formatCopy("explorer.settlement.final.detail", { batch: "7", status: "final" }),
    "Archive: final through batch 7 on the former anchor (final)",
  );
  assert.equal(formatCopy("explorer.anchors.action"), "View archive material");
  assert.match(formatCopy("explorer.anchors.body"), /Current receipts become final when included in a committed native block/u);
}

function retired_deposit_copy_guides_same_network_funding() {
  assert.equal(formatCopy("deposit.title"), "Fund your account");
  assert.equal(
    formatCopy("deposit.summary"),
    "Send funds to your account address on Paxeer X Network from a wallet on the same network. Check the network and address before sending.",
  );
  assert.equal(formatCopy("deposit.start"), "View funding guidance");
  assert.equal(
    formatCopy("deposit.sign.custody-transaction"),
    "Custody deposits are retired. Do not sign a custody deposit request.",
  );
  assert.match(formatCopy("deposit.pending.not_counted"), /receipt inclusion in a committed native block/u);
  assert.match(formatCopy("deposit.complete.body"), /only when its receipt is included in a committed native block/u);
  assert.match(formatCopy("deposit.safe_to_close"), /check the transaction receipt before sending again/u);
  for (const entry of copyEntries.filter(({ key }) => key.startsWith("deposit.stage."))) {
    assert.match(formatCopy(entry.key), /^Archive:/u, entry.key);
  }
  for (const entry of copyEntries.filter(({ key }) => key.startsWith("deposit."))) {
    assert.doesNotMatch(formatCopy(entry.key), /LayerX|on its side|continues on|finishes the rest/u, entry.key);
  }
  // Drive the production activity consumer through the generated runtime catalogue.
  assert.equal(kindLabel("deposit"), "Archive deposit");
  assert.equal(
    plainSentence({ kind: "deposit", state: "done", summary_copy_key: "activity.deposit.summary" }),
    "Archive: money was added through the retired deposit flow.",
  );
  assert.equal(
    plainSentence({ kind: "deposit", state: "processing", summary_copy_key: "activity.deposit.progress" }),
    "Archive: this retired deposit has no confirmed completion record.",
  );
}

function retired_exit_copy_guides_native_account_recovery() {
  assert.equal(formatCopy("exit.title"), "Account recovery");
  assert.equal(
    formatCopy("exit.summary"),
    "Your funds remain in your account on Paxeer X Network. To restore access, sign in with your passkey and review Account recovery in Security. If you cannot sign in, contact Support.",
  );
  assert.equal(formatCopy("security.recovery.title"), "Account recovery");
  assert.equal(formatCopy("exit.start"), "Emergency exit retired");
  assert.equal(formatCopy("exit.withdraw_instead"), "Review account recovery");
  assert.match(formatCopy("exit.unavailable.network-operating-normally"), /^Emergency exits are retired\./u);
  assert.match(formatCopy("error.exit.network-operating-normally"), /^Emergency exits are retired\./u);
  assert.equal(formatCopy("exit.confirmation.phrase"), "EMERGENCY EXIT RETIRED");
  assert.equal(formatCopy("exit.sign.exit-claim"), "Emergency exits are retired. Do not sign an exit claim.");
  assert.match(formatCopy("exit.irreversible"), /does not move funds or close a separate balance/u);
  assert.match(formatCopy("exit.degraded"), /does not bypass network availability/u);
  for (const entry of copyEntries.filter(({ key }) => key.startsWith("exit.stage."))) {
    assert.match(formatCopy(entry.key), /^Archive:/u, entry.key);
  }
  assert.match(formatCopy("exit.complete"), /^Archive:/u);
  assert.match(formatCopy("exit.complete.body"), /^Archive material/u);
  for (const entry of copyEntries.filter(({ key }) => key.startsWith("exit."))) {
    assert.doesNotMatch(formatCopy(entry.key), /LayerX|exit keeps working|Move everything|Withdraw normally/u, entry.key);
  }
}

async function repaired_copy_projection_reaches_the_runtime_without_drift() {
  // Use the generator's exact serialization contract, including order and newline.
  const serialized = [
    "export const runtimeMessages: readonly (readonly [string, string])[] = [",
    ...copyEntries.map(({ key, message }) => `  ${JSON.stringify([key, message])},`),
    "];",
    "",
  ].join("\n");
  assert.equal(
    await readFile(new URL("../copy/messages.generated.ts", import.meta.url), "utf8"),
    serialized,
  );
  assert.deepEqual(runtimeMessages, copyEntries.map(({ key, message }) => [key, message]));
  assert.equal(human_copy_catalog().size, copyEntries.length);
  assert.equal(new Set(copyEntries.map(({ key }) => key)).size, copyEntries.length);
  for (const { key, message } of copyEntries) {
    assert.deepEqual(copyEntry(key), { key, message });
  }
  assert.throws(() => copyEntry("missing.repair.key"), /Unknown copy key/u);
}

test(separate_interface_copy_uses_paxeer_x_network);
test(split_balances_and_settlement_copy_distinguishes_native_from_archive);
test(retired_deposit_copy_guides_same_network_funding);
test(retired_exit_copy_guides_native_account_recovery);
test(repaired_copy_projection_reaches_the_runtime_without_drift);
