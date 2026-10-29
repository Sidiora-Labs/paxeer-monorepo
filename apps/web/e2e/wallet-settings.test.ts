import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { registerHooks } from "node:module";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

import { copyEntry } from "../copy/runtime.ts";

registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier.startsWith(".") && context.parentURL !== undefined) {
      const candidate = new URL(specifier, context.parentURL);
      for (const suffix of [".ts", ".tsx", "/index.ts"]) {
        const source = candidate.href.endsWith(".js") ? new URL(candidate.href.slice(0, -3) + suffix) : new URL(candidate.href + suffix);
        if (existsSync(source)) return nextResolve(source.href, context);
      }
    }
    if (specifier === "server-only") return { url: "data:text/javascript,", shortCircuit: true };
    if (specifier === "next/navigation") return nextResolve("next/navigation.js", context);
    if (specifier === "next/link") return nextResolve("next/link.js", context);
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (/\.(ts|tsx)$/u.test(url) && !url.includes("/node_modules/")) {
      return {
        format: "module", shortCircuit: true,
        source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
          fileName: fileURLToPath(url),
          compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX },
        }).outputText,
      };
    }
    const loaded = nextLoad(url, context);
    return loaded.format === "commonjs" ? { ...loaded, source: undefined } : loaded;
  },
});

const { createElement } = await import("react");
const { renderToStaticMarkup } = await import("react-dom/server");
const { default: WalletSettingsPage } = await import("../src/app/app/settings/wallet/page.tsx");
const walletSettings = await import("../src/settings/wallet/index.ts");
const settings = await import("../src/settings/index.ts");
const model = await import("../src/settings/wallet/model.ts");
const screen = await import("../src/settings/wallet/wallet-screen.tsx");
const { walletFeePreference } = await import("../src/api/wallet.ts");

function wallet_settings_render_fee_choice_only() {
  const controller = new walletSettings.WalletFeeController();
  const previous = controller.snapshot;
  try {
    assert.equal(WalletSettingsPage().type, walletSettings.WalletFeeSettings);
    for (const currency of ["paxeer", "sidiora"] as const) {
      const selected = controller.choose(currency, "2.100001");
      assert.equal(selected.valid, true);
      const html = renderToStaticMarkup(createElement(WalletSettingsPage));
      assert.ok(html.includes(copyEntry("gas.sidiora.title").message));
      assert.ok(html.includes(copyEntry("gas.sidiora.paxeer").message));
      assert.ok(html.includes(copyEntry("gas.sidiora.choose").message));
      assert.equal(html.split('role="radiogroup"').length - 1, 1);
      assert.equal(html.split('role="radio"').length - 1, 2);
      assert.equal(html.split('aria-checked="true"').length - 1, 1);
      assert.doesNotMatch(html, /wallet-binding|\/app\/deposit|<a\b/iu);
      for (const label of ["Change wallet", "Add money", "Try wallet again", "Current wallet"]) {
        assert.ok(!html.includes(label), label);
      }
      if (currency === "sidiora") {
        assert.deepEqual(walletFeePreference(), { currency: "sidiora", maximum: 2_100_001n });
        assert.ok(html.includes(copyEntry("gas.sidiora.maximum").message));
        assert.ok(html.includes('value="2.100001"'));
      } else {
        assert.deepEqual(walletFeePreference(), { currency: "paxeer" });
        assert.doesNotMatch(html, /inputmode="decimal"/u);
      }
    }
  } finally {
    controller.choose(previous.currency, previous.maximum);
  }
}

function wallet_settings_module_exports_no_binding() {
  assert.deepEqual(Object.keys(walletSettings).sort(), ["WalletFeeChoice", "WalletFeeController", "WalletFeeSettings"]);
  assert.equal(walletSettings.WalletFeeController, model.WalletFeeController);
  assert.equal(walletSettings.WalletFeeChoice, screen.WalletFeeChoice);
  assert.equal(walletSettings.WalletFeeSettings, screen.WalletFeeSettings);
  assert.equal(settings.WalletFeeController, model.WalletFeeController);
  assert.equal(settings.WalletFeeChoice, screen.WalletFeeChoice);
  assert.equal(settings.WalletFeeSettings, screen.WalletFeeSettings);
  assert.equal(typeof model.newestWalletSecurityNotification, "function");
  for (const module of [walletSettings, settings, model, screen]) {
    for (const name of [
      "WALLET_BINDING_DOMAIN", "WalletBindingController", "WalletBindingScreen",
      "browserBindingWalletBridge", "receiptBackedBinding", "validBindingStatement",
      "wallet_binding", "human_web_wallet_binding",
    ]) {
      assert.equal(Object.hasOwn(module, name), false, name);
    }
  }
  assert.equal(existsSync(new URL("../src/settings/wallet/bridge.ts", import.meta.url)), false);
}

test(wallet_settings_render_fee_choice_only);
test(wallet_settings_module_exports_no_binding);
