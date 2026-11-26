import { formatUsdl } from '@/utils/format';
import { creationFunding, type FundingRead } from './creation-adapters';
import type { LaunchState } from './creation-controller';
import type { LaunchAttempt } from './creation-storage';
import type { FundingPlan } from './fee-summary';
import type { LaunchStepStatus } from './launch-progress';

export interface LaunchStepDescription {
  id: string;
  label: string;
  status: LaunchStepStatus;
  detail?: string;
  showRecoveryForm?: boolean;
}

export interface LaunchStepsInput {
  phase: LaunchState['phase'];
  approvalStage?: LaunchState['approvalStage'];
  attempt?: LaunchAttempt;
  plan: FundingPlan | null;
  firstBuyRaw?: string;
  resolutionVerified?: boolean;
  publicationVerified?: boolean;
  imageSkipped: boolean;
  metadataSkipped: boolean;
  metadataSettled: boolean;
}

export function fundingPlanForSummary(
  attempt: LaunchAttempt | undefined,
  read: FundingRead | null,
): FundingPlan | null {
  return attempt || !read ? null : creationFunding(read);
}

export function deriveLaunchSteps({
  phase,
  approvalStage,
  attempt,
  plan,
  firstBuyRaw,
  resolutionVerified,
  publicationVerified,
  imageSkipped,
  metadataSkipped,
  metadataSettled,
}: LaunchStepsInput): LaunchStepDescription[] {
  const approvalActive = phase === 'approving' &&
    (approvalStage === 'wallet_requested' || approvalStage === 'submitted');
  const approveStatus = attempt || approvalStage === 'confirmed'
    ? 'done'
    : approvalActive
      ? 'active'
      : plan?.state === 'signable'
        ? 'done'
        : 'pending';
  const steps: LaunchStepDescription[] = [{
    id: 'approve',
    label: 'Approve the 1 USDL fee',
    status: approveStatus,
    detail: approveStatus === 'done'
      ? 'Approved'
      : approvalActive
        ? approvalStage === 'wallet_requested'
          ? 'Confirm in your wallet'
          : 'Confirming...'
        : undefined,
  }];

  if (!attempt) {
    steps.push({
      id: 'create',
      label: 'Create coin',
      status: phase === 'signing' ? 'active' : 'pending',
      detail: phase === 'signing' ? 'Confirm in your wallet' : undefined,
      showRecoveryForm: false,
    });
    steps.push({ id: 'image', label: 'Upload image', status: 'pending' });
    steps.push({ id: 'metadata', label: 'Save coin details', status: 'pending' });
    steps.push({
      id: 'first-buy',
      label: 'First buy',
      status: firstBuyRaw === '0' ? 'skipped' : 'pending',
      detail: firstBuyRaw === '0' ? 'None' : undefined,
    });
    return steps;
  }

  const resolved = attempt.state === 'resolved' && resolutionVerified;
  const walletRequested = !attempt.createTxHash && phase === 'signing';
  steps.push({
    id: 'create',
    label: 'Create coin',
    status: attempt.state === 'reverted' ? 'error' : resolved ? 'done' : 'active',
    detail: attempt.state === 'reverted'
      ? "Didn't go through"
      : resolved
        ? 'Created'
        : attempt.createTxHash
          ? attempt.state === 'resolved'
            ? 'Checking...'
            : 'Confirming...'
          : walletRequested
            ? 'Confirm in your wallet'
            : 'Not confirmed yet',
    showRecoveryForm: !attempt.createTxHash && !walletRequested,
  });

  const imageDone = Boolean(attempt.metadataAssetId);
  const imageClosed = Boolean(attempt.metadataPatch) || attempt.metadataStatus === 'published';
  const imageStatus = imageDone
    ? 'done'
    : imageClosed || imageSkipped
      ? 'skipped'
      : resolved
        ? 'active'
        : 'pending';
  steps.push({
    id: 'image',
    label: 'Upload image',
    status: imageStatus,
    detail: imageDone ? 'Uploaded' : imageStatus === 'skipped' ? 'No image' : undefined,
  });
  steps.push({
    id: 'metadata',
    label: 'Save coin details',
    status: attempt.metadataStatus === 'published'
      ? publicationVerified ? 'done' : 'active'
      : metadataSkipped
        ? 'skipped'
        : resolved && (imageDone || imageSkipped || imageClosed)
          ? 'active'
          : 'pending',
    detail: attempt.metadataStatus === 'published'
      ? 'Saved'
      : metadataSkipped
        ? 'Add them later from the coin page'
        : attempt.metadataLastError
          ? "Didn't save, try again"
          : undefined,
  });

  const buyRequested = attempt.draft.firstBuyRaw !== '0';
  steps.push({
    id: 'first-buy',
    label: 'First buy',
    status: !buyRequested
      ? 'skipped'
      : attempt.purchaseState === 'confirmed'
        ? 'done'
        : attempt.purchaseState === 'skipped'
          ? 'skipped'
          : attempt.purchaseState === 'failed'
            ? 'error'
            : resolved && (metadataSettled || ['signing_locked', 'outcome_unknown', 'submitted'].includes(attempt.purchaseState))
              ? 'active'
              : 'pending',
    detail: !buyRequested
      ? 'None'
      : attempt.purchaseState === 'confirmed'
        ? `${formatUsdl(attempt.draft.firstBuyRaw)} bought`
        : attempt.purchaseState === 'skipped'
          ? 'Skipped'
          : attempt.purchaseState === 'failed'
            ? "Didn't go through"
            : formatUsdl(attempt.draft.firstBuyRaw),
  });
  return steps;
}
