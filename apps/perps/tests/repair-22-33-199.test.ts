import assert from "node:assert/strict";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { test } from "node:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";

process.env.NEXT_PUBLIC_PERPS_GATEWAY_URL = "http://127.0.0.1";
process.env.NEXT_PUBLIC_PAXSCAN_BASE_URL = "http://127.0.0.1";
process.env.NEXT_PUBLIC_PAXSCAN_API_URL = "http://127.0.0.1/api/v2";
process.env.NEXT_PUBLIC_PAXSCAN_WS_URL = "ws://127.0.0.1";
process.env.SENTRY_DSN = "";
process.env.NEXT_PUBLIC_SENTRY_DSN = "";

async function modules() {
  const { FeedbackSubmission } = await import(
    "../src/widgets/errors/feedback-submission"
  );
  const { ReportFeedbackForm } = await import(
    "../src/widgets/errors/ReportFeedbackModal"
  );
  const { Dialog } = await import("../src/ui/atoms/dialog");
  const { makeError } = await import("../src/core/errors/classify");
  const render = (draft: InstanceType<typeof FeedbackSubmission>) =>
    renderToStaticMarkup(
      React.createElement(
        Dialog,
        { open: true },
        React.createElement(ReportFeedbackForm, {
          draft,
          onClose: () => assert.fail("An unaccepted report must stay open"),
        }),
      ),
    );
  return { FeedbackSubmission, makeError, render };
}

async function repair_f042_unreportable_feedback_keeps_editable_draft() {
  const { FeedbackSubmission, makeError, render } = await modules();
  const draft = new FeedbackSubmission(makeError("WALLET_USER_REJECTED"));
  assert.equal(draft.error.isReportable, false);
  draft.setComments(
    "  I cancelled the signature, but the form kept spinning.  ",
  );
  draft.setEmail(" trader@example.test ");
  const original = draft.getSnapshot();
  const states: string[] = [];
  const unsubscribe = draft.subscribe(() =>
    states.push(draft.getSnapshot().status),
  );
  const submission = draft.submit();
  assert.equal(draft.getSnapshot().status, "sending");
  assert.match(render(draft), /Sending…/);
  await submission;
  assert.deepEqual(states, ["sending", "unavailable"]);
  assert.deepEqual(draft.getSnapshot(), { ...original, status: "unavailable" });
  let markup = render(draft);
  assert.match(markup, /role="alert"/);
  assert.match(markup, /Reporting is unavailable/);
  assert.match(markup, /Your message is still here/);
  assert.match(markup, /Try again/);
  assert.match(markup, /I cancelled the signature, but the form kept spinning/);
  assert.match(markup, /value=" trader@example.test "/);
  assert.doesNotMatch(markup, /disabled=""|accepted — thanks|Sent — thanks/);
  draft.setComments("Updated description for retry");
  await draft.submit();
  assert.equal(draft.getSnapshot().comments, "Updated description for retry");
  assert.equal(draft.getSnapshot().email, original.email);
  assert.equal(draft.getSnapshot().status, "unavailable");
  markup = render(draft);
  assert.match(markup, /Updated description for retry/);
  assert.doesNotMatch(markup, /disabled=""|accepted — thanks|Sent — thanks/);
  unsubscribe();
}
test(repair_f042_unreportable_feedback_keeps_editable_draft);

async function repair_f042_missing_sentry_client_is_not_receipt() {
  const { FeedbackSubmission, makeError, render } = await modules();
  const Sentry = await import("@sentry/nextjs");
  assert.equal(Sentry.getClient(), undefined);
  const draft = new FeedbackSubmission(makeError("API_TIMEOUT"));
  assert.equal(draft.error.isReportable, true);
  draft.setComments("Please investigate the disconnected order form.");
  draft.setEmail("offline@example.test");
  await draft.submit();
  assert.equal(Sentry.getClient(), undefined);
  assert.deepEqual(draft.getSnapshot(), {
    comments: "Please investigate the disconnected order form.",
    email: "offline@example.test",
    status: "unavailable",
  });
  const markup = render(draft);
  assert.match(markup, /Reporting is unavailable/);
  assert.match(markup, /Try again/);
  assert.match(markup, /Please investigate the disconnected order form/);
  assert.doesNotMatch(markup, /disabled=""|accepted — thanks|Sent — thanks/);
}
test(repair_f042_missing_sentry_client_is_not_receipt);

async function repair_f042_real_capture_and_retry_require_feedback_acknowledgement() {
  const { FeedbackSubmission, makeError, render } = await modules();
  const Sentry = await import("@sentry/nextjs");
  const envelopes: string[] = [];
  let responseStatus = 200;
  const server = createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    envelopes.push(Buffer.concat(chunks).toString("utf8"));
    response.writeHead(responseStatus, { "Content-Type": "application/json" });
    response.end("{}");
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  try {
    Sentry.init({
      dsn: `http://fixture@127.0.0.1:${(server.address() as AddressInfo).port}/1`,
      tunnel: `${origin}/envelope`,
      defaultIntegrations: [],
      skipOpenTelemetrySetup: true,
      autoSessionTracking: false,
      sendClientReports: false,
    });
    assert.ok(Sentry.getClient()?.getDsn());
    assert.notEqual(Sentry.getClient()?.getOptions().enabled, false);
    const draft = new FeedbackSubmission(
      makeError("API_TIMEOUT", "feedback-fixture"),
    );
    assert.equal(draft.error.isReportable, true);
    draft.setComments("  The order form disconnected.  ");
    draft.setEmail(" trader@example.test ");
    const submission = draft.submit();
    const duplicate = draft.submit();
    await Promise.all([submission, duplicate]);
    assert.equal(await Sentry.flush(2000), true);
    assert.equal(
      envelopes.length,
      2,
      "One error and one feedback, despite a double submit",
    );
    const payloads = envelopes.map((envelope) =>
      JSON.parse(envelope.split("\n")[2]),
    );
    const feedback = payloads.find((payload) => payload.type === "feedback");
    const error = payloads.find((payload) => payload.type !== "feedback");
    assert.ok(feedback);
    assert.ok(error);
    assert.equal(error.tags.reported_by_user, "true");
    assert.equal(error.tags["error.context"], "feedback-fixture");
    assert.equal(
      feedback.contexts.feedback.message,
      "The order form disconnected.",
    );
    assert.equal(
      feedback.contexts.feedback.contact_email,
      "trader@example.test",
    );
    assert.equal(
      feedback.contexts.feedback.associated_event_id,
      error.event_id,
    );
    assert.equal(
      draft.getSnapshot().status,
      "unavailable",
      "The current core reporter returns no explicit acknowledgement, even after capture",
    );
    assert.equal(
      draft.getSnapshot().comments,
      "  The order form disconnected.  ",
    );
    assert.equal(draft.getSnapshot().email, " trader@example.test ");
    assert.match(render(draft), /Reporting is unavailable/);
    assert.doesNotMatch(
      render(draft),
      /disabled=""|accepted — thanks|Sent — thanks/,
    );

    responseStatus = 503;
    draft.setComments("The retry also disconnected.");
    await draft.submit();
    assert.equal(await Sentry.flush(2000), true);
    assert.equal(envelopes.length, 4);
    const retryFeedback = envelopes
      .slice(2)
      .map((envelope) => JSON.parse(envelope.split("\n")[2]))
      .find((payload) => payload.type === "feedback");
    assert.ok(retryFeedback);
    assert.equal(
      retryFeedback.contexts.feedback.message,
      "The retry also disconnected.",
    );
    assert.equal(draft.getSnapshot().status, "unavailable");
    assert.equal(draft.getSnapshot().comments, "The retry also disconnected.");
    assert.equal(draft.getSnapshot().email, " trader@example.test ");
    assert.match(render(draft), /Try again/);
    assert.doesNotMatch(
      render(draft),
      /disabled=""|accepted — thanks|Sent — thanks/,
    );
  } finally {
    await Sentry.close(2000);
    server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      server.close((error) => (error ? reject(error) : resolve())),
    );
  }
}
test(repair_f042_real_capture_and_retry_require_feedback_acknowledgement);
