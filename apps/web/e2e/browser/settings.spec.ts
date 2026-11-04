import { expect, test } from "@playwright/test";

import { copyEntry } from "../../copy/catalog.ts";
import { formatCopy } from "../../copy/format.ts";
import { establishPublicSession } from "../public-session.ts";
import { decodeNotificationPreferences, decodeProfile } from "../../src/api/generated/index.ts";
import { readSettingsResponse } from "./settings-http.ts";

function requiredEnvironment(name: string): string {
  const value = process.env[name]?.trim();
  if (value === undefined || value.length === 0) {
    throw new Error(`${name} is required for authenticated settings qualification`);
  }
  return value;
}

test.beforeEach(async ({ context }) => {
  const baseUrl = new URL(requiredEnvironment("HUMAN_E2E_BASE_URL"));
  await establishPublicSession(context, baseUrl.origin);
});

test("@settings settings preferences persist and privacy masks every figure", async ({ page }) => {
  await page.goto("/app/settings", { waitUntil: "networkidle" });

  for (const section of [
    "settings.section.profile",
    "settings.section.security",
    "settings.section.wallet",
    "settings.section.notifications",
    "settings.section.advanced",
    "settings.section.help",
  ]) {
    await expect(page.getByText(copyEntry(section).message, { exact: true })).toBeVisible();
  }

  const pushLabel = copyEntry("settings.notifications.channel.push").message;
  const pushToggle = page.getByRole("switch", {
    name: formatCopy("settings.notifications.channel.toggle", { channel: pushLabel }),
  });
  const pushWasEnabled = await pushToggle.isChecked();
  if (!pushWasEnabled) {
    const channelSaved = page.waitForResponse((response) =>
      response.request().method() === "POST"
        && new URL(response.url()).pathname === "/human/v1/notifications/preferences"
    );
    await pushToggle.click();
    await readSettingsResponse(await channelSaved, decodeNotificationPreferences, "saved channel");
  }
  const approvalLabel = copyEntry("settings.notifications.class.approval_waiting").message;
  const approvalToggle = page.getByRole("switch", {
    name: formatCopy("settings.notifications.class.toggle", {
      notification: approvalLabel,
      channel: pushLabel,
    }),
  });
  const approvalWasEnabled = await approvalToggle.isChecked();
  const preferenceSaved = page.waitForResponse((response) =>
    response.request().method() === "POST"
      && new URL(response.url()).pathname === "/human/v1/notifications/preferences"
  );
  await approvalToggle.click();
  await readSettingsResponse(await preferenceSaved, decodeNotificationPreferences, "saved preference");
  await expect(approvalToggle).toBeChecked({ checked: !approvalWasEnabled });
  await page.reload({ waitUntil: "networkidle" });
  await expect(page.getByRole("switch", {
    name: formatCopy("settings.notifications.class.toggle", {
      notification: approvalLabel,
      channel: pushLabel,
    }),
  })).toBeChecked({ checked: !approvalWasEnabled });

  const privacyToggle = page.getByRole("switch", {
    name: copyEntry("settings.privacy.toggle").message,
  });
  const privacyWasEnabled = await privacyToggle.isChecked();
  if (!privacyWasEnabled) {
    await privacyToggle.click();
  }
  await expect(privacyToggle).toBeChecked();

  await page.goto("/app", { waitUntil: "networkidle" });
  const privateFigures = page.locator("[data-private-figure]");
  await expect(privateFigures.first()).toBeVisible();
  expect(await privateFigures.count()).toBeGreaterThan(0);
  expect(await privateFigures.evaluateAll((figures) => figures.every(
    (figure) => figure.getAttribute("data-private-figure") === "masked",
  ))).toBe(true);

  await page.reload({ waitUntil: "networkidle" });
  await expect(page.locator("[data-private-figure]").first()).toHaveAttribute(
    "data-private-figure",
    "masked",
  );

  await page.goto("/app/settings", { waitUntil: "networkidle" });
  if (!privacyWasEnabled) {
    await page.getByRole("switch", { name: copyEntry("settings.privacy.toggle").message }).click();
  }
  const restoredApproval = page.getByRole("switch", {
    name: formatCopy("settings.notifications.class.toggle", {
      notification: approvalLabel,
      channel: pushLabel,
    }),
  });
  if ((await restoredApproval.isChecked()) !== approvalWasEnabled) {
    await restoredApproval.click();
  }
  if (!pushWasEnabled) {
    await page.getByRole("switch", {
      name: formatCopy("settings.notifications.channel.toggle", { channel: pushLabel }),
    }).click();
  }
});


test("@settings profile and notification detail changes apply and persist", async ({ page }) => {
  await page.goto("/app/settings", { waitUntil: "networkidle" });
  await page.getByText(copyEntry("settings.profile.display_name").message, { exact: true }).click();
  const name = "Settings profile qualification";
  await page.getByRole("textbox", { name: copyEntry("settings.profile.display_name").message }).fill(name);
  const profileSaved = page.waitForResponse((response) =>
    response.request().method() === "PATCH"
      && new URL(response.url()).pathname === "/human/v1/profile"
  );
  await page.getByRole("button", { name: copyEntry("settings.action.save").message }).click();
  const savedProfile = await readSettingsResponse(await profileSaved, decodeProfile, "saved profile");
  expect(savedProfile.display_name).toBe(name);
  await expect(page.getByText(name, { exact: true })).toBeVisible();
  await page.reload({ waitUntil: "networkidle" });
  await expect(page.getByText(name, { exact: true })).toBeVisible();

  for (const detail of ["full", "minimal", "summary"] as const) {
    const choice = page.getByRole("tab", {
      name: copyEntry(`settings.notifications.detail.${detail}`).message,
      exact: true,
    });
    const detailSaved = page.waitForResponse((response) =>
      response.request().method() === "POST"
        && new URL(response.url()).pathname === "/human/v1/notifications/preferences"
    );
    await choice.click();
    const response = await detailSaved;
    const saved = await readSettingsResponse(response, decodeNotificationPreferences, "saved detail");
    expect(saved.detail).toBe(detail);
    await expect(choice).toHaveAttribute("aria-selected", "true");
    await page.reload({ waitUntil: "networkidle" });
    await expect(choice).toHaveAttribute("aria-selected", "true");
  }
});

test("@settings declared-local avatar can be added and cleared through the real profile API", async ({ page }) => {
  await page.goto("/app/settings", { waitUntil: "networkidle" });
  const initialResponse = await page.request.get("/human/v1/profile");
  const original = await readSettingsResponse(initialResponse, decodeProfile, "authenticated profile");
  const avatarUrl = new URL(requiredEnvironment("HUMAN_E2E_BASE_URL")).href;
  const openEditor = async () => {
    await page.getByText(copyEntry("settings.profile.display_name").message, { exact: true }).click();
  };
  const avatar = page.getByRole("textbox", { name: copyEntry("settings.profile.avatar").message });
  const save = async () => {
    const completed = page.waitForResponse((response) => response.request().method() === "PATCH"
      && new URL(response.url()).pathname === "/human/v1/profile");
    await page.getByRole("button", { name: copyEntry("settings.action.save").message }).click();
    const response = await completed;
    return readSettingsResponse(response, decodeProfile, "saved local profile");
  };
  await openEditor();
  await avatar.fill(avatarUrl);
  const added = await save();
  expect(added.display_name).toBe(original.display_name);
  expect(added.avatar_url).toBe(avatarUrl);
  await page.reload({ waitUntil: "networkidle" });
  await openEditor();
  await expect(avatar).toHaveValue(avatarUrl);
  await avatar.fill("");
  const cleared = await save();
  expect(cleared.display_name).toBe(original.display_name);
  expect(cleared.avatar_url).toBeUndefined();
  await page.reload({ waitUntil: "networkidle" });
  await openEditor();
  await expect(avatar).toHaveValue("");
  const persistedResponse = await page.request.get("/human/v1/profile");
  const persisted = await readSettingsResponse(persistedResponse, decodeProfile, "persisted local profile");
  expect(persisted).toEqual(cleared);
});

test("@settings privacy synchronizes tabs and remains scoped to the authenticated user", async ({ page, context }) => {
  await page.goto("/app/settings", { waitUntil: "networkidle" });
  const privacyName = copyEntry("settings.privacy.toggle").message;
  const privacyToggle = page.getByRole("switch", { name: privacyName });
  await expect(privacyToggle).not.toBeChecked();
  await privacyToggle.click();
  await expect(privacyToggle).toBeChecked();
  await page.reload({ waitUntil: "networkidle" });
  await expect(privacyToggle).toBeChecked();

  const otherTab = await context.newPage();
  await otherTab.goto("/app/settings", { waitUntil: "networkidle" });
  const otherToggle = otherTab.getByRole("switch", { name: privacyName });
  await expect(otherToggle).toBeChecked();
  await privacyToggle.click();
  await expect(privacyToggle).not.toBeChecked();
  await expect(otherToggle).not.toBeChecked();
  await otherToggle.click();
  await expect(privacyToggle).toBeChecked();
  await expect(otherToggle).toBeChecked();
  await otherTab.close();

  await page.goto("about:blank");
  await context.clearCookies();
  const baseUrl = new URL(requiredEnvironment("HUMAN_E2E_BASE_URL"));
  await establishPublicSession(context, baseUrl.origin);
  await page.goto("/app/settings", { waitUntil: "networkidle" });
  await expect(privacyToggle).not.toBeChecked();
});

test("@settings mandatory recovery and wallet rebinding keep an active delivery channel", async ({ page }) => {
  await page.goto("/app/settings", { waitUntil: "networkidle" });
  const channelToggle = (channel: "push" | "email" | "in_app") => page.getByRole("switch", {
    name: formatCopy("settings.notifications.channel.toggle", {
      channel: copyEntry(`settings.notifications.channel.${channel}`).message,
    }),
  });
  const criticalToggle = (event: "security_recovery" | "security_wallet_rebinding") => page.getByRole("switch", {
    name: formatCopy("settings.notifications.class.toggle", {
      notification: copyEntry(`settings.notifications.class.${event}`).message,
      channel: copyEntry("settings.notifications.channel.push").message,
    }),
  });
  const saveResponse = () => page.waitForResponse((response) =>
    response.request().method() === "POST"
      && new URL(response.url()).pathname === "/human/v1/notifications/preferences"
  );
  const push = channelToggle("push");
  if (!(await push.isChecked())) {
    const saved = saveResponse();
    await push.click();
    await readSettingsResponse(await saved, decodeNotificationPreferences, "enabled push channel");
    await expect(push).toBeChecked();
  }
  for (const event of ["security_recovery", "security_wallet_rebinding"] as const) {
    const critical = criticalToggle(event);
    if (!(await critical.isChecked())) {
      const saved = saveResponse();
      await critical.click();
      await readSettingsResponse(await saved, decodeNotificationPreferences, "enabled security preference");
      await expect(critical).toBeChecked();
    }
  }
  for (const channel of ["email", "in_app"] as const) {
    const toggle = channelToggle(channel);
    if (await toggle.isChecked()) {
      const saved = saveResponse();
      await toggle.click();
      await readSettingsResponse(await saved, decodeNotificationPreferences, "disabled delivery channel");
      await expect(toggle).not.toBeChecked();
    }
  }
  await push.click();
  await expect(push).toBeChecked();
  await expect(page.getByText(copyEntry("settings.notifications.non_suppressible").message, { exact: true })).toBeVisible();
  for (const event of ["security_recovery", "security_wallet_rebinding"] as const) {
    await criticalToggle(event).click();
    await expect(criticalToggle(event)).toBeChecked();
    await expect(page.getByText(copyEntry("settings.notifications.non_suppressible").message, { exact: true })).toBeVisible();
  }
  await page.reload({ waitUntil: "networkidle" });
  await expect(push).toBeChecked();
  await expect(channelToggle("email")).not.toBeChecked();
  await expect(channelToggle("in_app")).not.toBeChecked();
  for (const event of ["security_recovery", "security_wallet_rebinding"] as const) {
    await expect(criticalToggle(event)).toBeChecked();
  }
});
