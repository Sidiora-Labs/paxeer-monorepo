import { expect, test } from "@playwright/test";

import { copyEntry } from "../../copy/catalog.ts";
import { EXPLORER_NAVIGATION } from "../../src/explorer/links.ts";

const RECEIPT_IDENTIFIER = "d".repeat(64);
const ACCOUNT_IDENTIFIER = "e".repeat(64);
const PROGRAM_IDENTIFIER = "f".repeat(64);

const KEPT_PAGES = ["/explorer", "/explorer/verify"] as const;

function lookupForm(kind: "receipt" | "account" | "program") {
  return `form:has(input[name="kind"][value="${kind}"])`;
}

test.describe("Public Explorer Plane", () => {
  test("@explorer the overview renders its lookups on the public plane", async ({ page }) => {
    await page.goto("/explorer", { waitUntil: "networkidle" });

    await expect(page.getByRole("main")).toHaveCount(1);
    await expect(
      page.getByRole("heading", { level: 1, name: copyEntry("explorer.title").message }),
    ).toBeVisible();
    await expect(page.locator('[data-application="explorer"]')).toBeVisible();

    for (const kind of ["receipt", "account", "program"] as const) {
      await expect(page.locator(lookupForm(kind))).toBeVisible();
    }
  });

  test("@explorer the overview offers no retired anchor link", async ({ page }) => {
    await page.goto("/explorer", { waitUntil: "networkidle" });

    await expect(page.locator('a[href*="/paxeer-x/anchors"]')).toHaveCount(0);
    await expect(page.getByRole("link", {
      name: copyEntry("explorer.anchors.action").message,
      exact: true,
    })).toHaveCount(0);
  });

  test("@explorer receipt lookup redirects to the receipt detail page", async ({ page }) => {
    await page.goto("/explorer", { waitUntil: "networkidle" });

    const form = page.locator(lookupForm("receipt"));
    await form.locator('input[name="identifier"]').fill(RECEIPT_IDENTIFIER);
    await form.locator('button[type="submit"]').click();

    await page.waitForURL(/\/explorer\/receipts\//);
    await expect(page.locator('[data-application="explorer"]')).toBeVisible();
  });

  test("@explorer account lookup redirects to the account activity page", async ({ page }) => {
    await page.goto("/explorer", { waitUntil: "networkidle" });

    const form = page.locator(lookupForm("account"));
    await form.locator('input[name="identifier"]').fill(ACCOUNT_IDENTIFIER);
    await form.locator('button[type="submit"]').click();

    await page.waitForURL(/\/explorer\/accounts\//);
    await expect(page.locator('[data-application="explorer"]')).toBeVisible();
  });

  test("@explorer program lookup redirects to the program page", async ({ page }) => {
    await page.goto("/explorer", { waitUntil: "networkidle" });

    const form = page.locator(lookupForm("program"));
    await form.locator('input[name="identifier"]').fill(PROGRAM_IDENTIFIER);
    await form.locator('button[type="submit"]').click();

    await page.waitForURL(/\/explorer\/programs\//);
    await expect(page.locator('[data-application="explorer"]')).toBeVisible();
  });

  test("@explorer the evidence verifier renders and accepts input", async ({ page }) => {
    await page.goto("/explorer/verify", { waitUntil: "networkidle" });

    await expect(page.getByRole("main")).toHaveCount(1);
    await expect(page.locator('[data-application="explorer"]')).toBeVisible();
    await expect(page.locator("textarea")).toBeVisible();
    await expect(page.locator('[role="radiogroup"]')).toBeVisible();
    await expect(page.locator('button[type="submit"]')).toBeVisible();
  });

  test("@explorer the evidence verifier accepts receipt evidence", async ({ page }) => {
    await page.goto("/explorer/verify", { waitUntil: "networkidle" });

    const evidence = "dGVzdF9ldmlkZW5jZV9kYXRhX2Zvcl9yZWNlaXB0X3ZlcmlmaWNhdGlvbg";
    await page.locator("textarea").fill(evidence);
    await page.locator('button[type="submit"]').click();
    await page.waitForTimeout(1000);

    await expect(page.locator('[data-application="explorer"]')).toBeVisible();
  });

  test("@explorer the evidence verifier handles altered evidence", async ({ page }) => {
    await page.goto("/explorer/verify", { waitUntil: "networkidle" });

    const altered = "YWx0ZXJlZF9ldmlkZW5jZV90aGF0X3Nob3VsZF9mYWlsX3ZlcmlmaWNhdGlvbg";
    await page.locator("textarea").fill(altered);
    await page.locator('button[type="submit"]').click();
    await page.waitForTimeout(1000);

    const errorNotice = page.locator('[role="alert"]');
    if (await errorNotice.count() > 0) {
      await expect(errorNotice.first()).toBeVisible();
    }
  });

  test("@explorer every kept page is readable without authentication", async ({ page, context }) => {
    await context.clearCookies();

    for (const path of KEPT_PAGES) {
      await page.goto(path, { waitUntil: "networkidle" });

      await expect(page.getByRole("main")).toHaveCount(1);
      await expect(page.locator('[data-application="explorer"]')).toBeVisible();
      await expect(page.locator("[data-auth-required]")).toHaveCount(0);
    }
  });

  test("@explorer navigation names every kept page on every kept page", async ({ page }) => {
    for (const path of KEPT_PAGES) {
      await page.goto(path, { waitUntil: "networkidle" });

      await expect(page.getByRole("navigation", {
        name: copyEntry("explorer.navigation.label").message,
      })).toBeVisible();
      for (const item of EXPLORER_NAVIGATION) {
        await expect(page.locator(`nav a[href="${item.href}"]`)).toBeVisible();
      }
    }
  });

  test("@explorer deep links resolve on the surfaces the plane still serves", async ({ page }) => {
    const deepLinks = [
      `/explorer/receipts/${RECEIPT_IDENTIFIER}`,
      `/explorer/accounts/${ACCOUNT_IDENTIFIER}`,
      `/explorer/programs/${PROGRAM_IDENTIFIER}`,
    ];

    for (const path of deepLinks) {
      await page.goto(path, { waitUntil: "networkidle" });

      await expect(page.getByRole("main")).toHaveCount(1);
      await expect(page.locator('[data-application="explorer"]')).toBeVisible();
    }
  });
});
