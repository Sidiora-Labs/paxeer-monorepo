import { expect, test } from 'vitest';
import { CORE_DEPLOYMENT } from '@/core/network/deployment';
import { creationFunding, type FundingRead } from './creation-adapters';
import { review, selectOptical, verifyCost } from './creation-domain';
import type { CreationDraftV2, LaunchAttempt } from './creation-storage';
import { deriveLaunchSteps, fundingPlanForSummary, type LaunchStepsInput } from './launch-steps';

const deployment = {
  ...CORE_DEPLOYMENT,
  revision: CORE_DEPLOYMENT.manifestSha256,
  usdlSpender: CORE_DEPLOYMENT.launchpadPrecompile,
};
const transactionHash = '0x' + CORE_DEPLOYMENT.manifestSha256;
const attemptStates: LaunchAttempt['state'][] = [
  'signing_locked', 'outcome_unknown', 'submitted', 'resolved', 'reverted',
];
const phases: LaunchStepsInput['phase'][] = [
  'draft', 'review', 'approving', 'signing', 'submitted', 'resolution_delayed',
  'resolved', 'metadata_pending', 'published', 'purchase_review',
  'purchase_submitted', 'purchase_failed', 'purchase_confirmed',
];

function launchAttempt(overrides: Partial<LaunchAttempt> = {}): LaunchAttempt {
  const draft: CreationDraftV2 = {
    draftId: 'coin', version: 2, creator: CORE_DEPLOYMENT.treasury, chainId: 125,
    name: 'Coin', symbol: 'COIN', description: '', tags: [], socialLinks: [],
    uploadRefs: [], firstBuyRaw: '0', opticalSelection: 'standard',
    updatedAt: Date.now(), feeStrategy: 0,
  };
  const cost = verifyCost(deployment, {
    revision: deployment.revision, decimals: 6, feeRaw: '1000000', gasRaw: '2500000000000000',
  });
  return {
    intentId: draft.draftId, creator: draft.creator, chainId: 125, draftId: draft.draftId,
    reviewedSnapshot: review(draft.draftId, draft, cost, selectOptical('standard', deployment.revision, []), Date.now()).snapshot,
    draft, cost, deploymentRevision: deployment.revision, createdAt: Date.now(),
    state: 'signing_locked', metadataStatus: 'metadata_pending', metadataAttemptCount: 0,
    purchaseState: 'not_requested', ...overrides,
  };
}

function fundingRead(age = 0): FundingRead {
  return {
    cost: launchAttempt().cost,
    balances: { usdl: '12000000', pax: '1000000000000000000', allowance: '0' },
    blockHeight: '1', blockHash: transactionHash, gasStage: 'approval', observedAt: Date.now() - age,
  };
}

function launchSteps(overrides: Partial<LaunchStepsInput> = {}) {
  return deriveLaunchSteps({
    phase: 'review', plan: null, firstBuyRaw: '0', imageSkipped: false,
    metadataSkipped: false, metadataSettled: false, ...overrides,
  });
}

function createStepAsksForTheWalletWhileTheWalletIsAsked() {
  const steps = launchSteps({ phase: 'signing', attempt: launchAttempt() });
  expect(steps[1]).toEqual({
    id: 'create', label: 'Create coin', status: 'active',
    detail: 'Confirm in your wallet', showRecoveryForm: false,
  });
}

function recoveryFormShowsOnlyForAnUnknownOutcome() {
  expect(launchSteps({
    phase: 'resolution_delayed', attempt: launchAttempt({ state: 'outcome_unknown' }),
  })[1]).toEqual({
    id: 'create', label: 'Create coin', status: 'active',
    detail: 'Not confirmed yet', showRecoveryForm: true,
  });
  for (const phase of phases.filter(phase => phase !== 'signing')) {
    expect(launchSteps({ phase, attempt: launchAttempt() })[1]).toMatchObject({
      status: 'active', detail: 'Not confirmed yet', showRecoveryForm: true,
    });
  }
  for (const state of attemptStates) {
    for (const phase of phases) {
      expect(launchSteps({
        phase, attempt: launchAttempt({ state, createTxHash: transactionHash }),
      })[1].showRecoveryForm).toBe(false);
    }
  }
}

function approvalStepNamesWhatTheFlowWaitsFor() {
  const plan = creationFunding(fundingRead());
  expect(plan.state).toBe('approval_required');
  for (const phase of ['draft', 'review'] as const) {
    expect(launchSteps({ phase, plan })[0]).toEqual({
      id: 'approve', label: 'Approve the 1 USDL fee', status: 'pending', detail: undefined,
    });
  }
  expect(launchSteps({ phase: 'approving', plan, approvalStage: 'wallet_requested' })[0]).toMatchObject({
    status: 'active', detail: 'Confirm in your wallet',
  });
  expect(launchSteps({ phase: 'approving', plan, approvalStage: 'submitted' })[0]).toMatchObject({
    status: 'active', detail: 'Confirming...',
  });
  for (const phase of ['approving', 'draft', 'review', 'signing'] as const) {
    expect(launchSteps({ phase, plan, approvalStage: 'confirmed' })[0]).toMatchObject({
      status: 'done', detail: 'Approved',
    });
  }
  expect(launchSteps({ phase: 'draft', plan, approvalStage: 'wallet_requested' })[0]).toMatchObject({
    status: 'pending', detail: undefined,
  });
}

function costSummaryHoldsNoFundingStateOnceAnAttemptExists() {
  const read = fundingRead(31000);
  for (const state of attemptStates) {
    expect(fundingPlanForSummary(launchAttempt({ state }), read)).toBeNull();
  }
  expect(fundingPlanForSummary(undefined, read)).toEqual({ state: 'funding_unavailable', approvalTargetRaw: undefined });
  const fresh = fundingRead();
  expect(fundingPlanForSummary(undefined, fresh)).toEqual(creationFunding(fresh));
  expect(fundingPlanForSummary(undefined, null)).toBeNull();
  expect(fundingPlanForSummary(launchAttempt(), null)).toBeNull();
}

function stepsOfAConfirmedCreationStayAsTheyWere() {
  for (const [state, detail, status] of [
    ['signing_locked', 'Confirming...', 'active'],
    ['outcome_unknown', 'Confirming...', 'active'],
    ['submitted', 'Confirming...', 'active'],
    ['resolved', 'Checking...', 'active'],
    ['reverted', "Didn't go through", 'error'],
  ] as const) {
    const attempt = launchAttempt({ state, createTxHash: transactionHash });
    const steps = launchSteps({ attempt });
    expect(steps.map(({ id, status, detail }) => ({ id, status, detail }))).toEqual([
      { id: 'approve', status: 'done', detail: 'Approved' },
      { id: 'create', status, detail },
      { id: 'image', status: 'pending', detail: undefined },
      { id: 'metadata', status: 'pending', detail: undefined },
      { id: 'first-buy', status: 'skipped', detail: 'None' },
    ]);
    expect(steps[1].showRecoveryForm).toBe(false);
  }
  const resolved = launchAttempt({ state: 'resolved', createTxHash: transactionHash });
  expect(launchSteps({ attempt: resolved, resolutionVerified: true })[1]).toMatchObject({ status: 'done', detail: 'Created' });
  expect(launchSteps({ attempt: resolved, resolutionVerified: true })[2]).toMatchObject({ status: 'active', detail: undefined });
  expect(launchSteps({ attempt: resolved, resolutionVerified: true, imageSkipped: true })[2]).toMatchObject({ status: 'skipped', detail: 'No image' });
  expect(launchSteps({ attempt: { ...resolved, metadataAssetId: 'coin-image' }, resolutionVerified: true })[2]).toMatchObject({ status: 'done', detail: 'Uploaded' });
  expect(launchSteps({ attempt: { ...resolved, metadataLastError: 'Metadata service unavailable; token remains created.' }, resolutionVerified: true, imageSkipped: true })[3]).toMatchObject({ status: 'active', detail: "Didn't save, try again" });
  expect(launchSteps({ attempt: resolved, metadataSkipped: true })[3]).toMatchObject({ status: 'skipped', detail: 'Add them later from the coin page' });
  for (const publicationVerified of [false, true]) {
    expect(launchSteps({ attempt: { ...resolved, metadataStatus: 'published' }, publicationVerified })[3]).toMatchObject({ status: publicationVerified ? 'done' : 'active', detail: 'Saved' });
  }
  for (const [purchaseState, detail, status] of [
    ['not_requested', '5.00 USDL', 'pending'],
    ['review_required', '5.00 USDL', 'pending'],
    ['signing_locked', '5.00 USDL', 'active'],
    ['outcome_unknown', '5.00 USDL', 'active'],
    ['submitted', '5.00 USDL', 'active'],
    ['confirmed', '5.00 USDL bought', 'done'],
    ['skipped', 'Skipped', 'skipped'],
    ['failed', "Didn't go through", 'error'],
  ] as const) {
    const attempt = { ...resolved, draft: { ...resolved.draft, firstBuyRaw: '5000000' }, purchaseState };
    expect(launchSteps({ attempt, resolutionVerified: true })[4]).toMatchObject({ status, detail });
    if (purchaseState === 'review_required') {
      expect(launchSteps({ attempt, resolutionVerified: true, metadataSettled: true })[4]).toMatchObject({ status: 'active', detail });
    }
  }
}

test(createStepAsksForTheWalletWhileTheWalletIsAsked.name, createStepAsksForTheWalletWhileTheWalletIsAsked);
test(recoveryFormShowsOnlyForAnUnknownOutcome.name, recoveryFormShowsOnlyForAnUnknownOutcome);
test(approvalStepNamesWhatTheFlowWaitsFor.name, approvalStepNamesWhatTheFlowWaitsFor);
test(costSummaryHoldsNoFundingStateOnceAnAttemptExists.name, costSummaryHoldsNoFundingStateOnceAnAttemptExists);
test(stepsOfAConfirmedCreationStayAsTheyWere.name, stepsOfAConfirmedCreationStayAsTheyWere);
