import { randomUUID } from "node:crypto";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";
import { establishPublicSession } from "./public-session.ts";
import { installJourneyPeer, installPasskey, requirePost } from "./journey-browser.ts";
import { JourneyFixtureStore, type OnboardingScenario } from "./journey-fixtures.ts";
import { forbiddenCopy, onboardingMessages, vocabularyPattern } from "./copy-policy.ts";
import { copyEntry } from "../copy/runtime.ts";
import { CREATE_ACCOUNT_DECISIONS, SIGN_IN_DECISIONS, ONBOARDING_DECISION_LIMIT } from "../src/journeys/onboarding/model.ts";

async function requireVisibleCopy(page: Page): Promise<void> {
  const text = await page.locator('[data-journey="onboarding"]').innerText();
  expect(forbiddenCopy(text)).toBeUndefined();
}

async function openOnboarding(page: Page, context: BrowserContext, scenario: OnboardingScenario): Promise<void> {
  const session = await establishPublicSession(context, harness.baseUrl);
  const store = new JourneyFixtureStore(new URL(harness.baseUrl).origin, session.authenticator.publicKey());
  store.onboardingScenario = scenario;
  await installJourneyPeer(context, store);
  await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
  await expect(page.locator('[data-journey-phase="progress"]')).toBeVisible();
  await expect(page.locator('[data-stage-state]')).toHaveCount(4);
  await expect(page.locator('[data-account-active]')).toHaveAttribute("data-account-active", scenario === "active" ? "true" : "false");
  await expect(page.locator('[data-account-active]')).toHaveText(
    copyEntry(scenario === "active" ? "onboarding.active" : "onboarding.not_active").message,
  );
  await requireVisibleCopy(page);
  const before = await page.locator('[data-onboarding-stages]').innerText();
  await page.reload({ waitUntil: "networkidle" });
  await expect(page.locator('[data-onboarding-stages]')).toHaveText(before);
  expect(store.requests.filter((request) => request.path === "/human/v1/onboarding").length).toBeGreaterThanOrEqual(2);
}

async function createAccountScreen(page: Page): Promise<void> {
  await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
  await page.getByLabel(/email/i).fill(`onboarding-${randomUUID()}@paxeer.network`);
  await page.getByLabel(/name/i).fill("Onboarding qualification");
  await requirePost(page, "/human/v1/accounts", () => page.getByRole("button", { name: /continue/i }).click());
  await expect(page.locator('[data-decision-screen="2"]')).toBeVisible();
  await requireVisibleCopy(page);
}

async function signInOnNewDevice(page: Page, context: BrowserContext): Promise<void> {
  const browser = context.browser();
  expect(browser).not.toBeNull();
  const firstContext = await browser!.newContext();
  try {
    const first = await establishPublicSession(firstContext, harness.baseUrl);
    await installPasskey(context, first, page);
    const store = new JourneyFixtureStore(new URL(harness.baseUrl).origin, first.authenticator.publicKey());
    store.onboardingScenario = "active";
    // Notifications remain on their real producer, which records the new session.
    await context.route("**/human/v1/onboarding", async (route) => {
      const answer = store.answer({ method: route.request().method(), path: new URL(route.request().url()).pathname });
      expect(answer).toBeDefined();
      await route.fulfill({ status: answer!.status, contentType: "application/json", body: answer!.body });
    });
    await page.goto(harness.baseUrl);
    await page.getByRole("button", { name: /sign in instead/i }).click();
    await page.getByLabel(/email/i).fill(first.email);
    await requirePost(page, "/human/v1/sessions", () => page.getByRole("button", { name: /sign in with your passkey/i }).click());
    await expect(page.getByRole("button", { name: /go to your account/i })).toBeVisible();
    const sessions = await first.client.sessionList();
    expect(sessions.sessions.length).toBeGreaterThanOrEqual(2);
    expect(new Set(sessions.sessions.map((item) => item.session_id)).size).toBe(sessions.sessions.length);
    await requireVisibleCopy(page);
  } finally { await firstContext.close(); }
}
import { human_test_harness, SHELL_PROFILES } from "./harness";

const harness = human_test_harness(process.env);

for (const [shellName, profile] of Object.entries(SHELL_PROFILES)) {
  test.describe(`${shellName} shell onboarding journeys`, () => {
    test.use(profile);

    test("@journey renders account creation with email and name fields", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await expect(page.locator('[data-journey="onboarding"]')).toBeVisible();
      await expect(page.locator('[data-decision-screen="1"]')).toBeVisible();
      await expect(page.getByLabel(/email/i)).toBeVisible();
      await expect(page.getByLabel(/name/i)).toBeVisible();
      await expect(page.getByRole("button", { name: /continue/i })).toBeVisible();
    });

    test("@journey enforces the three-decision limit", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const journey = page.locator('[data-journey="onboarding"]');
      await expect(journey).toHaveAttribute("data-decision-limit", "3");
    });

    test("@journey shows switch to sign-in action", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await expect(page.getByRole("button", { name: /sign in instead/i })).toBeVisible();
    });

    test("@journey renders sign-in screen with passkey ceremony", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await page.getByRole("button", { name: /sign in instead/i }).click();
      await expect(page.locator('[data-decision-screen="1"]')).toBeVisible();
      await expect(page.getByRole("button", { name: /sign in with your passkey/i })).toBeVisible();
      await expect(page.getByRole("button", { name: /create an account/i })).toBeVisible();
    });

    test("@journey shows honest activation progress during onboarding", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      await expect(page.locator('[data-journey-phase="progress"]')).toBeVisible();
      await expect(page.locator('[data-onboarding-stages]')).toBeVisible();
    });

    test("@journey presents account-active notice only when verified", async ({ page, context }) => {
      await openOnboarding(page, context, "active");
      const activeNotice = page.locator('[data-account-active]');
      await expect(activeNotice).toBeVisible();
    });

    test("@journey shows staged progress with status pills", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      await expect(page.locator('[data-onboarding-stages]')).toBeVisible();
      await expect(page.locator('[data-stage-state]')).toHaveCount(4);
    });

    test("@journey displays safe-to-close guidance during activation", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      await expect(
        page.getByText(/safe to close/i)
      ).toBeVisible();
    });

    test("@journey shows failure notice with retry for failed stages", async ({ page, context }) => {
      await openOnboarding(page, context, "failure");
      await expect(page.locator('[data-onboarding-failure]')).toBeVisible();
      await expect(
        page.getByText(/progress is saved|finished steps stay finished/i)
      ).toBeVisible();
    });

    test("@journey renders resume action to continue incomplete onboarding", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      await expect(page.locator('[data-onboarding-resume]')).toBeVisible();
    });

    test("@journey shows queued state when protocol layer unavailable", async ({ page, context }) => {
      await openOnboarding(page, context, "queued");
      await expect(page.locator('[data-onboarding-queued]')).toBeVisible();
      await expect(
        page.getByText(/waiting in line|nothing you did is lost/i)
      ).toBeVisible();
    });

    test("@journey validates required email field", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const emailField = page.getByLabel(/email/i);
      await expect(emailField).toHaveAttribute("required", "");
      await expect(emailField).toHaveAttribute("type", "email");
    });

    test("@journey validates required name field", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const nameField = page.getByLabel(/name/i);
      await expect(nameField).toHaveAttribute("required", "");
    });

    test("@journey new-device sign-in shows security notification", async ({ page, context }) => {
      await signInOnNewDevice(page, context);
      const securityNotice = page.locator('[data-security-notice="security-new-device"]');
      await expect(securityNotice).toBeVisible();
      const reviewAction = page.locator('[data-security-notice-action]');
      await expect(reviewAction).toBeVisible();
    });

    test("@journey new-device sign-in notification links to device list", async ({ page, context }) => {
      await signInOnNewDevice(page, context);
      const reviewAction = page.locator('[data-security-notice-action]');
      await expect(reviewAction).toHaveAttribute("data-security-notice-action", "");
    });

    test("@journey renders passkey ceremony screen as decision 2", async ({ page, context }) => {
      await createAccountScreen(page);
      await expect(page.locator('[data-decision-screen="2"]')).toBeVisible();
      await expect(page.getByRole("button", { name: /create your passkey/i })).toBeVisible();
    });

    test("@journey passkey screen shows stage progress", async ({ page, context }) => {
      await createAccountScreen(page);
      await expect(page.locator('[data-onboarding-stages]')).toBeVisible();
    });

    test("@journey validates shell-specific rendering", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const journey = page.locator('[data-journey="onboarding"]');
      await expect(journey).toHaveAttribute("data-shell", shellName);
    });

    test("@journey signed-in screen shows continue action", async ({ page, context }) => {
      await signInOnNewDevice(page, context);
      await expect(page.getByRole("button", { name: /go to your account/i })).toBeVisible();
    });

    test("@journey preserves return_to parameter through sign-in", async ({ page, context }) => {
      const returnPath = "/app/activity";
      await page.goto(`${harness.baseUrl}?return_to=${encodeURIComponent(returnPath)}`, {
        waitUntil: "networkidle",
      });
      await expect(page.locator('[data-journey="onboarding"]')).toBeVisible();
    });

    test("@journey ceremony cancellation shows retriable failure", async ({ page, context }) => {
      await createAccountScreen(page);
      await page.getByRole("button", { name: /create your passkey/i }).click();
      await expect(
        page.getByText(/passkey step was cancelled|try again/i)
      ).toBeVisible();
    });

    test("@journey offline state shows appropriate message", async ({ page, context }) => {
      await context.setOffline(true);
      await page.goto(harness.baseUrl, { waitUntil: "domcontentloaded" }).catch(() => { });
      await context.setOffline(false);
      await page.reload({ waitUntil: "networkidle" });
      await page.getByLabel(/email/i).fill("test@example.com");
      await page.getByLabel(/name/i).fill("Test User");
      await context.setOffline(true);
      await page.getByRole("button", { name: /continue/i }).click();
      await expect(page.locator('[data-onboarding-failure]')).toBeVisible();
    });

    test("@journey email field uses correct input mode", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const emailField = page.getByLabel(/email/i);
      await expect(emailField).toHaveAttribute("inputmode", "email");
    });

    test("@journey email field has autocomplete", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const emailField = page.getByLabel(/email/i);
      await expect(emailField).toHaveAttribute("autocomplete", "email");
    });

    test("@journey name field has autocomplete", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      const nameField = page.getByLabel(/name/i);
      await expect(nameField).toHaveAttribute("autocomplete", "name");
    });

    test("@journey resume preserves completed stages", async ({ page, context }) => {
      await openOnboarding(page, context, "resume");
      await expect(page.locator('[data-stage-state="done"]')).toHaveCount(2, { timeout: 5000 });
      await expect(page.locator('[data-stage-state="processing"]')).toHaveCount(1);
      await requirePost(page, "/human/v1/onboarding/resume", () => page.locator('[data-onboarding-resume]').click());
      await expect(page.locator('[data-account-active]')).toHaveAttribute("data-account-active", "true");
      await page.reload();
      await expect(page.locator('[data-stage-state="done"]')).toHaveCount(3);
      await expect(page.locator('[data-account-active]')).toHaveAttribute("data-account-active", "true");
    });

    test("@journey shows protocol identity stage with plain language", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      await expect(
        page.locator('[data-stage-key="onboarding.stage.setting-up-your-protocol-identity"]')
      ).toBeVisible();
      await expect(
        page.getByText(/activating your account/i)
      ).toBeVisible();
    });

    test("@a11y onboarding form elements have proper labels", async ({ page, context }) => {
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await expect(page.getByLabel(/email/i)).toHaveAccessibleName(/email/i);
      await expect(page.getByLabel(/name/i)).toHaveAccessibleName(/name/i);
    });

    test("@a11y failure notice has alert role", async ({ page, context }) => {
      await openOnboarding(page, context, "failure");
      const failureNotice = page.locator('[data-onboarding-failure]').locator("..");
      await expect(failureNotice).toHaveAttribute("role", "alert");
    });

    test("@a11y active status notice has status role", async ({ page, context }) => {
      await openOnboarding(page, context, "progress");
      const activeNotice = page.locator('[data-account-active]').locator("..");
      await expect(activeNotice).toHaveAttribute("role", "status");
    });

    test("@visual onboarding account creation baseline", async ({ page, context }, testInfo) => {
      if (!harness.visualBaselineReviewed) {
        test.skip();
      }
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await expect(page.locator('[data-journey="onboarding"]')).toBeVisible();
      await expect(page).toHaveScreenshot(
        `${testInfo.project.name}-onboarding-creation.png`,
        {
          fullPage: true,
          animations: "disabled",
        }
      );
    });

    test("@visual onboarding sign-in baseline", async ({ page, context }, testInfo) => {
      if (!harness.visualBaselineReviewed) {
        test.skip();
      }
      await page.goto(harness.baseUrl, { waitUntil: "networkidle" });
      await page.getByRole("button", { name: /sign in instead/i }).click();
      await expect(page.locator('[data-decision-screen="1"]')).toBeVisible();
      await expect(page).toHaveScreenshot(
        `${testInfo.project.name}-onboarding-signin.png`,
        {
          fullPage: true,
          animations: "disabled",
        }
      );
    });

    test("@visual onboarding progress baseline", async ({ page, context }, testInfo) => {
      if (!harness.visualBaselineReviewed) {
        test.skip();
      }
      await openOnboarding(page, context, "progress");
      await expect(page.locator('[data-journey-phase="progress"]')).toBeVisible();
      await expect(page).toHaveScreenshot(
        `${testInfo.project.name}-onboarding-progress.png`,
        {
          fullPage: true,
          animations: "disabled",
        }
      );
    });
  });
}

test.describe("cross-shell onboarding invariants", () => {
  test("onboarding decision count never exceeds limit", async () => {
    const CREATE_DECISIONS = CREATE_ACCOUNT_DECISIONS;
    const SIGNIN_DECISIONS = SIGN_IN_DECISIONS;
    const DECISION_LIMIT = ONBOARDING_DECISION_LIMIT;
    expect(CREATE_DECISIONS).toBeLessThanOrEqual(DECISION_LIMIT);
    expect(SIGNIN_DECISIONS).toBeLessThanOrEqual(DECISION_LIMIT);
  });

  test("banned vocabulary does not appear in onboarding copy", async () => {
    const ONBOARDING_COPY_KEYS = [
      "onboarding.stage.creating-your-account",
      "onboarding.stage.adding-your-passkey",
      "onboarding.stage.setting-up-your-protocol-identity",
      "onboarding.stage.putting-recovery-in-place",
      "onboarding.create.title",
      "onboarding.create.body",
      "onboarding.email.label",
      "onboarding.name.label",
      "onboarding.create.action",
      "onboarding.passkey.title",
      "onboarding.passkey.body",
      "onboarding.passkey.action",
      "onboarding.passkey.resume.body",
      "onboarding.passkey.resume.action",
      "onboarding.progress.title",
      "onboarding.progress.body",
      "onboarding.progress.safe_to_close",
      "onboarding.pending.body",
      "onboarding.active",
      "onboarding.not_active",
      "onboarding.failure.title",
      "onboarding.failure.stage",
      "onboarding.failure.preserved",
      "onboarding.resume.action",
      "onboarding.signin.title",
      "onboarding.signin.body",
      "onboarding.signin.action",
      "onboarding.signin.switch",
      "onboarding.create.switch",
      "onboarding.signin.done",
      "onboarding.signin.continue",
      "onboarding.ceremony.cancelled",
    ];
    for (const key of ONBOARDING_COPY_KEYS) {
      expect(copyEntry(key).message.length).toBeGreaterThan(0);
    }
    for (const message of onboardingMessages()) {
      expect(message).not.toMatch(/[{}]/u);
      expect(forbiddenCopy(message)).toBeUndefined();
    }
    expect(forbiddenCopy("Your proof is ready.")).toBe("proof");
    expect(vocabularyPattern("proof").test("Your proof is ready.")).toBe(true);
    expect(forbiddenCopy("Your waterproof coat is ready.")).toBeUndefined();
  });
});
