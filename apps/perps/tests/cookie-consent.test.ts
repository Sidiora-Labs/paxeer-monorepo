// The cookie policy controls and banner share the same consent choice.

import { test } from "node:test";
import { strict as assert } from "node:assert";

import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { cookieConsentStore } from "../apps/web/src/legal/useCookieConsent";

function aChoiceFromOneConsumerIsSeenByEveryConsumer() {
  let calls = 0;
  const unsubscribe = cookieConsentStore.subscribe(() => {
    calls += 1;
  });

  assert.equal(cookieConsentStore.getState().loading, true);
  cookieConsentStore.getState().hydrate();
  assert.equal(cookieConsentStore.getState().value, null);
  assert.equal(cookieConsentStore.getState().loading, false);

  const before = calls;
  cookieConsentStore.getState().decline();
  assert.equal(cookieConsentStore.getState().value, "declined");
  assert.ok(calls > before, "listener fired on decline");

  const afterDecline = calls;
  cookieConsentStore.getState().accept();
  assert.equal(cookieConsentStore.getState().value, "accepted");
  assert.ok(calls > afterDecline, "listener fired on accept");

  cookieConsentStore.getState().hydrate();
  assert.equal(
    cookieConsentStore.getState().value,
    "accepted",
    "hydrate after a choice keeps it",
  );

  unsubscribe();
}

test(aChoiceFromOneConsumerIsSeenByEveryConsumer);

function cookieConsentPersistsAcrossConsumerRestarts() {
  const directory = mkdtempSync(join(tmpdir(), "perps-cookie-preferences-"));
  const moduleURL = new URL(
    "../apps/web/src/legal/useCookieConsent.ts",
    import.meta.url,
  ).href;
  const run = (body: string) =>
    execFileSync(
      process.execPath,
      [
        "--experimental-webstorage",
        `--localstorage-file=${join(directory, "preferences")}`,
        "--import",
        "tsx",
        "--input-type=module",
        "--eval",
        `import assert from "node:assert/strict";
     const { cookieConsentStore, COOKIE_CONSENT_STORAGE_KEY } = await import(${JSON.stringify(moduleURL)});
     ${body}`,
      ],
      { stdio: "pipe" },
    );
  try {
    run(`cookieConsentStore.getState().hydrate();
      assert.equal(cookieConsentStore.getState().value, null);
      assert.equal(localStorage.length, 0);
      cookieConsentStore.getState().accept();
      assert.equal(localStorage.getItem(COOKIE_CONSENT_STORAGE_KEY), "accepted");
      assert.equal(localStorage.length, 1);`);
    run(`cookieConsentStore.getState().hydrate();
      assert.equal(cookieConsentStore.getState().value, "accepted");
      cookieConsentStore.getState().decline();
      assert.equal(localStorage.getItem(COOKIE_CONSENT_STORAGE_KEY), "declined");
      assert.equal(localStorage.length, 1);`);
    run(`cookieConsentStore.getState().hydrate();
      assert.equal(cookieConsentStore.getState().value, "declined");
      assert.equal(cookieConsentStore.getState().loading, false);`);
  } finally {
    rmSync(directory, { recursive: true, force: true });
  }
}

test(cookieConsentPersistsAcrossConsumerRestarts);
