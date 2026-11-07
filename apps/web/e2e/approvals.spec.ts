import { expect, type Page } from "@playwright/test";
import { test, installJourneyPeer, installPasskey, requireDecision } from "./journey-browser.ts";
import { signInPublicSession } from "./public-session.ts";
import { copyEntry } from "../copy/runtime.ts";

test.beforeEach(async ({ journeyPeer }) => {
  expect(journeyPeer.details.size).toBe(3);
});

test.describe("Approvals journey", () => {
  test.beforeEach(async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
  });

  test("@journey renders approval inbox in mobile shell", async ({ page }, testInfo) => {
    if (!testInfo.project.name.includes("mobile")) {
      test.skip();
    }
    await expect(page.locator('[data-application="approvals"]')).toBeVisible();
    await expect(page.getByText("Approvals", { exact: true })).toBeVisible();
  });

  test("@journey renders approval inbox in desktop shell", async ({ page }, testInfo) => {
    if (!testInfo.project.name.includes("desktop")) {
      test.skip();
    }
    await expect(page.locator('[data-application="approvals"]')).toBeVisible();
    await expect(page.getByText("Approvals", { exact: true })).toBeVisible();
  });

  test("@journey @critical grants approval with step-up ceremony", async ({ page, journeyPeer }) => {
    const approvalId = journeyPeer.idFor("pending");
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    const approveButton = page.getByRole("button", { name: "Approve" });
    await expect(approveButton).toBeVisible();

    await approveButton.click();
    await expect(page.getByText(/will move/i)).toBeVisible();
    const confirmButton = page.getByRole("button", { name: "Approve" }).last();
    await confirmButton.click();
    await requireDecision(journeyPeer, approvalId, "approved");
    await expect(page.getByRole("status")).toBeVisible();

  });

  test("@journey @critical rejects approval with nothing-moved confirmation", async ({ page, journeyPeer }) => {
    const approvalId = journeyPeer.idFor("pending");
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    const rejectButton = page.getByRole("button", { name: "Reject" });
    await expect(rejectButton).toBeVisible();

    await rejectButton.click();
    await expect(page.getByText(/nothing/i)).toBeVisible();
    const confirmButton = page.getByRole("button", { name: "Reject" }).last();
    await confirmButton.click();
    await requireDecision(journeyPeer, approvalId, "rejected");
    await expect(page.getByRole("status")).toBeVisible();

  });

  test("@journey renders expired approval state", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const expiredBadge = page.getByText("Expired");
    await expect(expiredBadge).toBeVisible();

    const expiredItem = page.locator("li[role='button']", { has: expiredBadge }).first();
    await expiredItem.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.getByText(/expired/i)).toBeVisible();
    await expect(page.getByRole("button", { name: "Approve" })).not.toBeVisible();

  });

  test("@journey tracks released activity after approval grant", async ({ page, journeyPeer }) => {
    const approvalId = journeyPeer.idFor("pending");
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    const approveButton = page.getByRole("button", { name: "Approve" });
    await expect(approveButton).toBeVisible();

    await approveButton.click();
    const confirmButton = page.getByRole("button", { name: "Approve" }).last();
    await confirmButton.click();
    await requireDecision(journeyPeer, approvalId, "approved");
    await expect(page.getByText("Released activity")).toBeVisible({ timeout: 10000 });
    const trackLink = page.getByRole("link", { name: /track/i });
    await expect(trackLink).toBeVisible();

  });

  test("@journey @critical shows expiry countdown for pending approvals", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const firstPendingApproval = page.locator("li[role='button']").first();
    await expect(firstPendingApproval).toBeVisible();

    await firstPendingApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    const countdown = page.locator('[role="timer"]');
    await expect(countdown).toBeVisible();

    await expect(countdown).toHaveText(/minute|imminent/i);

  });

  test("@journey displays budget remaining after approval", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.getByText(/budget/i)).toBeVisible();
    await expect(page.getByText(/verification/i)).toBeVisible();

  });

  test("@journey desktop split view shows inbox and detail", async ({ page }, testInfo) => {
    if (!testInfo.project.name.includes("desktop")) {
      test.skip();
    }
    await page.goto("/app/approvals");
    await expect(page.locator("section").filter({ hasText: "Approvals" })).toBeVisible();
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.locator("section").filter({ hasText: "Approvals" })).toBeVisible();

  });

  test("@journey mobile screen navigates from inbox to detail", async ({ page }, testInfo) => {
    if (!testInfo.project.name.includes("mobile")) {
      test.skip();
    }
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.locator("section").filter({ hasText: "Approvals" })).not.toBeVisible();

  });
});

test.describe("Two-device convergence", () => {
  async function openApprovalInSecondContext(approvalId: string, secondPage: Page) {
    await secondPage.goto(`/app/approvals/${approvalId}`);
    await expect(secondPage.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(secondPage.getByRole("button", { name: "Approve", exact: true })).toBeVisible();
  }

  for (const action of ["approve", "reject"] as const) {
    test(action === "approve"
      ? "@journey @critical shows already-decided when second device acts on same approval"
      : "@journey converges to same outcome across devices", async ({ page, context, browser, baseURL, publicSession, journeyPeer }) => {
        const approvalId = journeyPeer.idFor("pending");
        const secondContext = await browser.newContext({ baseURL: baseURL! });
        try {
          expect(secondContext).not.toBe(context);
          const secondSession = await signInPublicSession(secondContext, baseURL!, publicSession);
          expect(secondSession.accountId).toBe(publicSession.accountId);
          expect(secondSession.session.session_id).not.toBe(publicSession.session.session_id);
          expect(secondSession.session.device.device_id).not.toBe(publicSession.session.device.device_id);
          await installJourneyPeer(secondContext, journeyPeer);
          const secondPage = await secondContext.newPage();
          await installPasskey(secondContext, secondSession, secondPage);
          await openApprovalInSecondContext(approvalId, secondPage);
          await page.goto(`/app/approvals/${approvalId}`);
          const label = action === "approve" ? "Approve" : "Reject";
          await expect(page.getByRole("button", { name: label, exact: true })).toBeVisible();
          await page.getByRole("button", { name: label, exact: true }).click();
          await expect(page.getByRole("dialog")).toBeVisible();
          await page.getByRole("dialog").getByRole("button", { name: label, exact: true }).click();
          await requireDecision(journeyPeer, approvalId, action === "approve" ? "approved" : "rejected");
          const identity = journeyPeer.decisionKeys.get(approvalId);
          // Act from the still-open, independently authenticated consumer before reload.
          await secondPage.getByRole("button", { name: "Reject", exact: true }).click();
          await secondPage.getByRole("dialog").getByRole("button", { name: "Reject", exact: true }).click();
          await expect(secondPage.getByText(action === "approve" ? /released activity/i : /nothing.*moved/i).first()).toBeVisible();
          await secondPage.reload();
          await expect(secondPage.getByText(/already decided/i)).toBeVisible();
          await expect(secondPage.getByRole("button", { name: "Approve", exact: true })).toHaveCount(0);
          await expect(secondPage.getByRole("button", { name: "Reject", exact: true })).toHaveCount(0);
          await requireDecision(journeyPeer, approvalId, action === "approve" ? "approved" : "rejected");
          expect(journeyPeer.decisionKeys.get(approvalId)).toBe(identity);
          expect(journeyPeer.requests.filter((request) => request.path.endsWith(`/${approvalId}/reject`)).length).toBeGreaterThanOrEqual(1);
        } finally { await secondContext.close(); }
      });
  }
});

test.describe("Approval expiry handling", () => {
  test("@journey expired approval cannot be approved", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const expiredBadge = page.getByText("Expired");
    await expect(expiredBadge).toBeVisible();

    const expiredItem = page.locator("li[role='button']", { has: expiredBadge }).first();
    await expiredItem.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.getByRole("button", { name: "Approve" })).not.toBeVisible();
    await expect(page.getByRole("button", { name: "Reject" })).not.toBeVisible();

  });

  test("@journey expired approval shows no money moved message", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const expiredBadge = page.getByText("Expired");
    await expect(expiredBadge).toBeVisible();

    const expiredItem = page.locator("li[role='button']", { has: expiredBadge }).first();
    await expiredItem.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.getByText(/expired/i)).toBeVisible();

  });
});

test.describe("Defective approval handling", () => {
  test("@journey defective approval shows error state", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const defectiveBadge = page.getByText("Defective");
    await expect(defectiveBadge).toBeVisible();

    const defectiveItem = page.locator("li[role='button']", { has: defectiveBadge }).first();
    await defectiveItem.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    await expect(page.getByText(/defective/i)).toBeVisible();
    await expect(page.getByRole("button", { name: "Approve" })).not.toBeVisible();

  });
});

test.describe("Technical details disclosure", () => {
  test("@journey renders disclosure from held activity", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    for (const key of ["approval.detail.agent", "approval.detail.counterparty", "approval.detail.amount", "approval.detail.fees"]) {
      await expect(page.getByText(copyEntry(key).message, { exact: true })).toBeVisible();
    }

  });

  test("@journey exposes technical details", async ({ page, journeyPeer }) => {
    await page.goto("/app/approvals");
    const firstApproval = page.locator("li[role='button']").first();
    await expect(firstApproval).toBeVisible();

    await firstApproval.click();
    await expect(page.locator('[data-application="approval-detail"]')).toBeVisible();
    const technicalDetails = page.getByText("Technical details");
    await expect(technicalDetails).toBeVisible();

    await technicalDetails.click();
    await expect(page.getByText(/reference|evidence/i)).toBeVisible();

  });
});
