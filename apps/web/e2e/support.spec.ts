import { expect } from "@playwright/test";
import { test as base } from "./production-fixture.ts";
import { decodeSupportConversation, decodeSupportConversationPage } from "../src/api/generated/index.ts";
import { openSupportFixture } from "../src/support/service.fixture.ts";

const test = base.extend<{ supportPeer: Awaited<ReturnType<typeof openSupportFixture>> }>({
  supportPeer: [async ({ page, authenticatedSession }, use) => {
    const peer = await openSupportFixture();
    expect(authenticatedSession.session.current).toBe(true);
    peer.service.principal = authenticatedSession.accountId;
    await page.route(/\/human\/v1\/(support\/conversations|account\/balance)/u, async (route) => {
      const incoming = route.request();
      const path = new URL(incoming.url()).pathname;
      const response = await fetch(`${peer.baseUrl.replace(/\/human$/u, "")}${path}`, {
        method: incoming.method(), headers: incoming.headers(),
        ...(incoming.postData() === null ? {} : { body: incoming.postData()! }),
      });
      await route.fulfill({ status: response.status, headers: Object.fromEntries(response.headers), body: await response.text() });
    });
    try { await use(peer); } finally { await peer.close(); }
  }, { auto: true }],
});

const CONVERSATIONS = "/human/v1/support/conversations";

import { copyEntry } from "../copy/catalog.ts";
import { SUPPORT_TOPICS } from "../src/support/topics.ts";

test.describe("@support support chat states and flows", () => {
  test("renders empty state with topic suggestion chips on mobile", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    await expect(page.getByRole("heading", { name: copyEntry("support.title").message })).toBeVisible();
    await expect(page.getByText(copyEntry("support.empty").message)).toBeVisible();
    await expect(page.getByText(copyEntry("support.empty.body").message)).toBeVisible();

    const suggestionGroup = page.getByRole("group", { name: copyEntry("support.suggestions").message });
    await expect(suggestionGroup).toBeVisible();

    for (const topic of SUPPORT_TOPICS) {
      await expect(suggestionGroup.getByRole("button", { name: copyEntry(topic.labelKey).message })).toBeVisible();
    }
  });

  test("renders empty state with topic suggestion chips on desktop", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    await expect(page.getByRole("heading", { name: copyEntry("support.title").message })).toBeVisible();
    await expect(page.getByText(copyEntry("support.empty").message)).toBeVisible();

    const suggestionGroup = page.getByRole("group", { name: copyEntry("support.suggestions").message });
    await expect(suggestionGroup).toBeVisible();

    for (const topic of SUPPORT_TOPICS) {
      await expect(suggestionGroup.getByRole("button", { name: copyEntry(topic.labelKey).message })).toBeVisible();
    }
  });

  test("seeds draft when topic chip is clicked", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const firstTopic = SUPPORT_TOPICS[0];
    if (firstTopic === undefined) {
      throw new Error("SUPPORT_TOPICS must have at least one topic");
    }

    const topicButton = page.getByRole("button", { name: copyEntry(firstTopic.labelKey).message });
    await topicButton.click();

    const textField = page.getByRole("textbox", { name: copyEntry("support.compose.label").message });
    await expect(textField).toHaveValue(copyEntry(firstTopic.seedKey).message);
  });

  test("disables send button when draft is empty", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const sendButton = page.getByRole("button", { name: copyEntry("support.send").message });
    await expect(sendButton).toBeDisabled();
  });

  test("enables send button when draft has content", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const textField = page.getByRole("textbox", { name: copyEntry("support.compose.label").message });
    await textField.fill("I need help with a deposit");

    const sendButton = page.getByRole("button", { name: copyEntry("support.send").message });
    await expect(sendButton).toBeEnabled();
  });

  test("attaches trace identifier from query parameter", async ({ page }) => {
    const traceId = "trc_0123456789abcdef0123456789abcdef";
    await page.goto(`/app/support?trace=${traceId}`, { waitUntil: "networkidle" });

    await expect(page.getByText(copyEntry("support.trace.attached").message)).toBeVisible();
    await expect(page.getByText(traceId, { exact: false })).toBeVisible();
  });

  test("renders offline state with retry button", async ({ page }) => {
    await page.route(CONVERSATIONS, route => route.abort("failed"));
    const observed = page.waitForRequest(request => new URL(request.url()).pathname === CONVERSATIONS && request.method() === "GET");
    await page.goto("/app/support", { waitUntil: "networkidle" });
    await observed;

    await expect(page.getByText(copyEntry("state.offline.banner").message)).toBeVisible();
    await expect(page.getByRole("button", { name: copyEntry("action.retry").message })).toBeVisible();
    await page.unroute(CONVERSATIONS);
    const retried = page.waitForResponse(response => new URL(response.url()).pathname === CONVERSATIONS && response.request().method() === "GET");
    await page.getByRole("button", { name: copyEntry("action.retry").message }).click();
    expect((await retried).ok()).toBe(true);
    await expect(page.getByText(copyEntry("support.empty").message)).toBeVisible();
  });

  test("shows sending state when message is being sent", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const textField = page.getByRole("textbox", { name: copyEntry("support.compose.label").message });
    await textField.fill("Test message");

    const sendButton = page.getByRole("button", { name: copyEntry("support.send").message });

    let release: () => void = () => { throw new Error("Delivery barrier is not initialized"); };
    const barrier = new Promise<void>(resolve => { release = resolve; });
    await page.route(CONVERSATIONS, async route => {
      if (route.request().method() === "POST") await barrier;
      await route.continue();
    });
    const observed = page.waitForRequest(request => new URL(request.url()).pathname === CONVERSATIONS && request.method() === "POST");
    try {
      await sendButton.click();
      await observed;
      await expect(sendButton).toBeDisabled();
    } finally {
      release();
    }

  });

  test("desktop shell renders as docked panel", async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 960 });
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const supportContainer = page.locator("[data-support-shell='desktop']");
    await expect(supportContainer).toBeVisible();

    const box = await supportContainer.boundingBox();
    if (box === null) {
      throw new Error("Support container must have a bounding box");
    }

    const viewport = page.viewportSize();
    if (viewport === null) {
      throw new Error("Viewport size must be set");
    }

    expect(box.x + box.width).toBeGreaterThan(viewport.width - 100);
    expect(box.y + box.height).toBeGreaterThan(viewport.height - 100);
  });

  test("mobile shell renders as full screen", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const supportContainer = page.locator("[data-support-shell='mobile']");
    await expect(supportContainer).toBeVisible();

    const box = await supportContainer.boundingBox();
    if (box === null) {
      throw new Error("Support container must have a bounding box");
    }

    expect(box.x).toBeLessThanOrEqual(10);
    expect(box.y).toBeLessThanOrEqual(10);
  });
});

test.describe("@support report-to-support from error surfaces", () => {
  test("error surface includes support button", async ({ page }) => {
    await page.route("/human/v1/**", route => route.abort("failed"));

    const observed = page.waitForRequest(request => new URL(request.url()).pathname === "/human/v1/home");
    await page.goto("/app", { waitUntil: "networkidle" });
    await observed;

    await expect(page.getByRole("button", { name: copyEntry("support.open").message })).toBeVisible();
  });

  test("error surface support button navigates with trace", async ({ page }) => {
    await page.route("/human/v1/**", route => route.abort("failed"));

    const observed = page.waitForRequest(request => new URL(request.url()).pathname === "/human/v1/home");
    await page.goto("/app", { waitUntil: "networkidle" });
    await observed;

    const traceText = await page.locator("text=/trc_[0-9a-f]{32}/").textContent();
    const traceMatch = traceText?.match(/trc_[0-9a-f]{32}/);

    if (traceMatch === null || traceMatch === undefined) {
      throw new Error("Error surface must display a trace identifier");
    }

    const traceId = traceMatch[0];

    const supportButton = page.getByRole("button", { name: copyEntry("support.open").message });
    await supportButton.click();

    await expect(page).toHaveURL(new RegExp(`/app/support\\?trace=${traceId}`));
    await expect(page.getByText(copyEntry("support.trace.attached").message)).toBeVisible();
    await expect(page.getByText(traceId, { exact: false })).toBeVisible();
  });

  test("report action submits trace identifier", async ({ page }) => {
    await page.route("/human/v1/**", route => route.abort("failed"));

    const observed = page.waitForRequest(request => new URL(request.url()).pathname === "/human/v1/home");
    await page.goto("/app", { waitUntil: "networkidle" });
    await observed;

    const reportButton = page.getByRole("button", { name: copyEntry("action.report").message });
    await reportButton.click();

    await expect(page.getByRole("heading", { name: copyEntry("report.consent.title").message })).toBeVisible();
    await expect(page.getByText(copyEntry("report.consent.body").message)).toBeVisible();

    const confirmButton = page.getByRole("button", { name: copyEntry("report.submit").message });
    await expect(confirmButton).toBeVisible();
  });
});

test.describe("@support message delivery states", () => {
  test("shows retry button for failed messages", async ({ page }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });
    const textField = page.getByRole("textbox", { name: copyEntry("support.compose.label").message });
    const body = "Test message that will fail";
    await textField.fill(body);
    await page.route(CONVERSATIONS, async route => {
      if (route.request().method() === "POST") await route.abort("failed");
      else await route.continue();
    });
    const failedRequest = page.waitForRequest(request => new URL(request.url()).pathname === CONVERSATIONS && request.method() === "POST");
    await page.getByRole("button", { name: copyEntry("support.send").message }).click();
    const failed = await failedRequest;
    expect(failed.postDataJSON().body).toBe(body);
    await expect(page.getByText(copyEntry("support.message.failed").message)).toBeVisible();
    await expect(page.getByRole("button", { name: copyEntry("action.retry").message })).toBeVisible();
    await expect(page.getByText(body, { exact: true })).toBeVisible();
    await textField.fill("Retain this unsent draft during retry");
    await page.unroute(CONVERSATIONS);
    const retryResponse = page.waitForResponse(response => new URL(response.url()).pathname === CONVERSATIONS && response.request().method() === "POST");
    await page.getByRole("alert").getByRole("button", { name: copyEntry("action.retry").message }).click();
    const retried = await retryResponse;
    expect(retried.request().headers()["idempotency-key"]).toBe(failed.headers()["idempotency-key"]);
    expect(retried.request().postDataJSON().body).toBe(body);
    const envelope = await retried.json();
    expect(envelope.ok).toBe(true);
    const conversation = decodeSupportConversation(envelope.result, "support response");
    expect(conversation.messages.filter(message => message.body === body)).toHaveLength(1);
    await expect(textField).toHaveValue("Retain this unsent draft during retry");
    await expect(page.getByText(copyEntry("support.message.failed").message)).toHaveCount(0);
  });

  test("feedback loop appears after support replies", async ({ page, supportPeer }) => {
    await page.goto("/app/support", { waitUntil: "networkidle" });

    const composer = page.getByRole("textbox", { name: copyEntry("support.compose.label").message });
    await composer.fill("I need help");
    const created = page.waitForResponse(response => new URL(response.url()).pathname === CONVERSATIONS && response.request().method() === "POST");
    await page.getByRole("button", { name: copyEntry("support.send").message }).click();
    const createEnvelope = await (await created).json();
    expect(createEnvelope.ok).toBe(true);
    const createdConversation = decodeSupportConversation(createEnvelope.result, "support response");
    await expect.poll(() => supportPeer.service.conversations().length).toBe(1);
    const conversation = supportPeer.service.conversations()[0]!;
    expect(conversation.conversation_id).toBe(createdConversation.conversation_id);
    const listed = page.waitForResponse(response => new URL(response.url()).pathname === CONVERSATIONS && response.request().method() === "GET");
    supportPeer.service.recordSupportReply(conversation.conversation_id, "operator-feedback", "How can I help you?");
    await expect(page.getByText("How can I help you?", { exact: true })).toBeVisible();
    const listEnvelope = await (await listed).json();
    expect(listEnvelope.ok).toBe(true);
    const pageValue = decodeSupportConversationPage(listEnvelope.result, "support list response");
    const current = pageValue.conversations.find(value => value.conversation_id === conversation.conversation_id);
    expect(current).toBeDefined();
    const reply = current?.messages.findLast(message => message.author === "support");
    expect(reply).toBeDefined();

    await expect(page.getByText(copyEntry("support.feedback.question").message)).toBeVisible();
    await expect(page.getByRole("button", { name: copyEntry("support.feedback.yes").message })).toBeVisible();
    await expect(page.getByRole("button", { name: copyEntry("support.feedback.no").message })).toBeVisible();
    const feedbackPath = `${CONVERSATIONS}/${encodeURIComponent(conversation.conversation_id)}/feedback`;
    const feedback = page.waitForResponse(response => new URL(response.url()).pathname === feedbackPath && response.request().method() === "POST");
    await page.getByRole("button", { name: copyEntry("support.feedback.yes").message }).click();
    const feedbackResponse = await feedback;
    expect(feedbackResponse.request().postDataJSON()).toEqual({ message_id: reply!.message_id, helpful: true });
    const feedbackEnvelope = await feedbackResponse.json();
    expect(feedbackEnvelope.ok).toBe(true);
    const saved = decodeSupportConversation(feedbackEnvelope.result, "support response");
    expect(saved.feedback.some(value => value.message_id === reply!.message_id && value.helpful)).toBe(true);
  });
});
