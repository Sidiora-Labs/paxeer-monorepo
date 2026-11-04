// @vitest-environment jsdom
import { test } from 'vitest';
import { proveHostedIntentReopen } from './account/test/hostedIntentRecording';

async function wallet_app_hosted_intent_plan_survives_producer_store_reopen() {
  await proveHostedIntentReopen();
}
test(
  wallet_app_hosted_intent_plan_survives_producer_store_reopen.name,
  wallet_app_hosted_intent_plan_survives_producer_store_reopen
);
