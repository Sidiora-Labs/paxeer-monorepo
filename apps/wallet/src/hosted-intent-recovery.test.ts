// @vitest-environment jsdom
import { expect, test } from 'vitest';
import { proveHostedIntentReopen } from './account/test/hostedIntentRecording';

async function wallet_app_hosted_intent_plan_survives_producer_store_reopen() {
  const recoveredPlan = await proveHostedIntentReopen();
  expect(recoveredPlan.plan_digest).toMatch(/^[0-9a-f]{64}$/);
  expect(recoveredPlan.legs.length).toBeGreaterThan(0);
}
test(
  wallet_app_hosted_intent_plan_survives_producer_store_reopen.name,
  wallet_app_hosted_intent_plan_survives_producer_store_reopen
);
