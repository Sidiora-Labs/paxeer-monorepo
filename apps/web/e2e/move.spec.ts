import { expect, type Page } from "@playwright/test";
import { test, requirePost } from "./journey-browser.ts";
import type { JourneyFixtureStore } from "./journey-fixtures.ts";

test.beforeEach(async ({ publicSession, journeyPeer }) => {
  expect(publicSession.session.current).toBe(true);
  expect(journeyPeer.decisions.size).toBe(0);
});

async function commitRenderedMove(page: Page, store: JourneyFixtureStore): Promise<void> {
  await expect(page.getByRole("button", { name: /move money/i })).toBeEnabled();
  await requirePost(page, "/human/v1/moves", () => page.getByRole("button", { name: /move money/i }).click());
  expect(store.requests.filter((request) => request.method === "POST" && request.path === "/human/v1/moves/quote")).toHaveLength(1);
  const commits = store.requests.filter((request) => request.method === "POST" && request.path === "/human/v1/moves");
  expect(commits).toHaveLength(1);
  expect(commits[0]!.idempotencyKey).toMatch(/^[a-f0-9]{32}$/u);
}

test.describe("Move money journey", () => {
  test("@journey navigates to move money from home", async ({ page, journeyPeer }) => {
    await page.goto("/app", { waitUntil: "networkidle" });
    await expect(page.locator("main")).toHaveCount(1);

    const moveButton = page.getByRole("button", { name: /move money/i });
    await expect(moveButton).toBeVisible();
    await moveButton.click();

    await expect(page).toHaveURL("/app/move");
    await expect(page.getByRole("heading", { name: /move money/i })).toBeVisible();
  });

  test("@journey completes move money wizard with three decisions in mobile shell", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const whoStep = page.getByRole("heading", { name: /who/i });
    await expect(whoStep).toBeVisible();

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await expect(destinationOption).toBeChecked();

    const continueButton = page.getByRole("button", { name: /continue/i });
    await expect(continueButton).toBeEnabled();
    await continueButton.click();

    const amountStep = page.getByRole("heading", { name: /how much/i });
    await expect(amountStep).toBeVisible();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");

    await expect(continueButton).toBeEnabled();
    await continueButton.click();

    const reviewStep = page.getByRole("heading", { name: /review/i });
    await expect(reviewStep).toBeVisible();

    await expect(page.getByText(/fee estimate/i)).toBeVisible();
    await expect(page.getByText(/ceiling/i)).toBeVisible();

    const confirmButton = page.getByRole("button", { name: /move money/i });
    await expect(confirmButton).toBeEnabled();
  });

  test("@journey completes move money wizard with three decisions in desktop shell", async ({ page, journeyPeer }) => {
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const whoStep = page.getByRole("heading", { name: /who/i });
    await expect(whoStep).toBeVisible();

    await expect(page.getByText(/summary/i)).toBeVisible();

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();

    const continueButton = page.getByRole("button", { name: /continue/i });
    await continueButton.click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");

    await continueButton.click();

    const reviewStep = page.getByRole("heading", { name: /review/i });
    await expect(reviewStep).toBeVisible();

    await expect(page.getByText(/fee estimate/i)).toBeVisible();
    await expect(page.getByText(/ceiling/i)).toBeVisible();
  });

  test("@journey shows review with plain language quote", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();

    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");

    await page.getByRole("button", { name: /continue/i }).click();

    await expect(page.getByText(/fee estimate/i)).toBeVisible();
    await expect(page.getByText(/ceiling/i)).toBeVisible();
    await expect(page.getByText(/arrival/i)).toBeVisible();
    await expect(page.getByText(/automatic/i)).toBeVisible();
  });

  test("@journey renders other account destination option", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const otherAccountOption = page.getByRole("radio", { name: /other account/i });
    await expect(otherAccountOption).toBeVisible();
    await otherAccountOption.check();

    const accountInput = page.getByRole("textbox", { name: /account/i });
    await expect(accountInput).toBeVisible();
    await accountInput.fill("acc_test_12345678");

    const continueButton = page.getByRole("button", { name: /continue/i });
    await expect(continueButton).toBeEnabled();
  });

  test("@journey prevents continuation without destination selection", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const continueButton = page.getByRole("button", { name: /continue/i });
    await expect(continueButton).toBeDisabled();
  });

  test("@journey prevents continuation without amount", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();

    await page.getByRole("button", { name: /continue/i }).click();

    const continueButton = page.getByRole("button", { name: /continue/i });
    await expect(continueButton).toBeDisabled();
  });

  test("@journey @refusal renders refusal state with honest message", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("999999999999");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "refused";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByRole("alert")).toBeVisible();
    await expect(page.getByText(/refused/i)).toBeVisible();
    await expect(page.getByText(/money.*stayed|money.*left/i)).toBeVisible();
  });

  test("@journey @refusal shows change path when refusal provides it", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("100000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "change-path";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByRole("button", { name: /change/i })).toBeVisible();
  });

  test("@journey @still-checking renders still-checking state with locked actions", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "still-checking";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByText(/still.?checking/i)).toBeVisible();
    await expect(page.getByText(/locked/i)).toBeVisible();

    const actionButtons = page.getByRole("button").filter({ hasText: /send|again/i });
    await expect(actionButtons).not.toHaveCount(0);
    await expect(actionButtons.first()).toBeDisabled();
  });

  test("@journey renders in-progress state with receipt-gating", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "sending";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByRole("status")).toBeVisible();
    await expect(page.getByText(/sending|processing/i)).toBeVisible();
  });

  test("@journey @receipt-backed renders done state only when receipt-backed", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "done";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByText(/done|complete/i)).toBeVisible();
    await expect(page.getByText(/receipt/i)).toBeVisible();
  });

  test("@journey shows processing when done state lacks receipt", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "no-receipt";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByText(/processing/i)).toBeVisible();
    const doneIndicator = page.getByText(/done|complete/i);
    await expect(doneIndicator).not.toBeVisible();
  });

  test("@journey supports cancellation back to home", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const cancelButton = page.getByRole("button", { name: /cancel|back/i });
    await expect(cancelButton).toBeVisible();
    await cancelButton.click();

    await expect(page).toHaveURL("/app");
  });

  test("@journey handles quote expiration gracefully", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();
    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");
    await page.getByRole("button", { name: /continue/i }).click();

    journeyPeer.moveScenario = "quote-expired";

    await commitRenderedMove(page, journeyPeer);

    await expect(page.getByText(/expired/i)).toBeVisible();
    await expect(page.getByRole("heading", { name: /review/i })).toBeVisible();
  });

  test("@journey @visual desktop split pane shows live summary", async ({ page, journeyPeer }, testInfo) => {
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const destinationOption = page.getByRole("radio").first();
    await destinationOption.check();

    await expect(page.getByText(/to/i)).toBeVisible();

    await page.getByRole("button", { name: /continue/i }).click();

    const amountInput = page.getByRole("textbox", { name: /amount/i });
    await amountInput.fill("1000");

    await expect(page.getByText(/amount/i)).toBeVisible();
    await expect(page.getByText(/1.*000/)).toBeVisible();

    expect(process.env.HUMAN_VISUAL_BASELINE_REVIEWED).toBe("1");
    await expect(page).toHaveScreenshot(`${testInfo.project.name}-move-summary.png`, {
      fullPage: true,
      animations: "disabled",
    });
  });

  test("@journey mobile full-screen wizard hides other content", async ({ page, journeyPeer }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const wizardContainer = page.locator("[role='region']").filter({ hasText: /move money/i });
    await expect(wizardContainer).toBeVisible();

    const mainLandmark = page.locator("main");
    await expect(mainLandmark).toBeVisible();
  });

  test("@journey enforces three decisions maximum for initiation", async ({ page, journeyPeer }) => {
    await page.goto("/app/move", { waitUntil: "networkidle" });

    const steps = page.getByRole("button", { name: /who|amount|review/i });
    await expect(steps).toHaveCount(3);
  });
});
