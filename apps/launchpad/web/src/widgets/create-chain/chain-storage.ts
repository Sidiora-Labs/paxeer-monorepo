import { address, hash, raw } from '../create/creation-domain';
import type { ChainSubmission } from './chain-adapters';
import { validateChainDraft, type ChainDraft } from './chain-domain';
import type { ChainFeeApproval } from './chain-fees';

const submissionKey = 'chain-submission-v2';

export class ChainDraftStorage {
  private creator: string;

  constructor(creator: string) {
    this.creator = address(creator);
  }

  private key() {
    return `chain-draft-v2:125:${this.creator}`;
  }

  private checked(draft: ChainDraft, kind: 'draft' | 'submission'): ChainDraft {
    const now = Date.now();
    const errors = validateChainDraft(draft, kind === 'draft' ? now : draft?.updatedAt);
    if (kind === 'draft') {
      delete errors.name;
      if (draft.tokenName === '') delete errors.tokenName;
      if (draft.tokenSymbol === '') delete errors.tokenSymbol;
    }
    if (Object.keys(errors).length || draft.version !== 2 || address(draft.creator) !== this.creator
      || !Number.isSafeInteger(now) || draft.updatedAt > now
      || typeof draft.name !== 'string' || draft.name.length > 32
      || typeof draft.tokenName !== 'string' || new TextEncoder().encode(draft.tokenName).length > 50
      || typeof draft.tokenSymbol !== 'string' || new TextEncoder().encode(draft.tokenSymbol).length > 10) {
      throw new Error('Your saved chain draft is invalid or expired.');
    }
    const { draftId, version, creator, chainId, name, tokenName, tokenSymbol, feeStrategy, tier, feeShareBps, updatedAt } = draft;
    return { draftId, version, creator, chainId, name, tokenName, tokenSymbol, feeStrategy, tier, feeShareBps, updatedAt };
  }

  saveDraft(draft: ChainDraft): void {
    localStorage.setItem(this.key(), JSON.stringify(this.checked(draft, 'draft')));
  }

  restoreDraft(): ChainDraft | null {
    localStorage.removeItem(`chain-draft-v1:125:${this.creator}`);
    const body = localStorage.getItem(this.key());
    if (body === null) return null;
    if (body.length > 4096) throw new Error('Your saved chain draft is too large.');
    return this.checked(JSON.parse(body), 'draft');
  }

  saveSubmission(draft: ChainDraft, submission: ChainSubmission): void {
    const checked = this.checked(draft, 'submission');
    if (address(submission.creator) !== this.creator || submission.chainId !== checked.chainId
      || !['confirm', 'reverted'].includes(submission.phase)) throw new Error('Invalid saved submission.');
    const saved: ChainSubmission = {
      creator: this.creator, chainId: 125, txHash: hash(submission.txHash) as ChainSubmission['txHash'],
      phase: submission.phase, bondRaw: raw(submission.bondRaw).toString(),
    };
    localStorage.setItem(submissionKey, JSON.stringify({ draft: checked, submission: saved }));
  }

  restoreSubmission(): { draft: ChainDraft; submission: ChainSubmission } | null {
    return ChainDraftStorage.restoreSubmission();
  }

  static restoreSubmission(): { draft: ChainDraft; submission: ChainSubmission } | null {
    localStorage.removeItem('chain-submission-v1');
    const body = localStorage.getItem(submissionKey);
    if (body === null) return null;
    if (body.length > 4096) throw new Error('Your saved submission is too large.');
    const saved = JSON.parse(body) as { draft: ChainDraft; submission: ChainSubmission };
    const owner = new ChainDraftStorage(saved.submission.creator);
    const draft = owner.checked(saved.draft, 'submission');
    const submission = saved.submission;
    if (address(submission.creator) !== address(draft.creator) || submission.chainId !== draft.chainId
      || !['confirm', 'reverted'].includes(submission.phase)) throw new Error('Invalid saved submission.');
    return { draft, submission: {
      creator: address(submission.creator), chainId: 125, txHash: hash(submission.txHash) as ChainSubmission['txHash'],
      phase: submission.phase, bondRaw: raw(submission.bondRaw).toString(),
    } };
  }

  clearSubmission(txHash: string): void {
    const saved = this.restoreSubmission();
    if (saved && saved.submission.txHash === hash(txHash) && saved.submission.creator === this.creator) {
      localStorage.removeItem(submissionKey);
    }
  }

  clearDraft(): void {
    localStorage.removeItem(this.key());
  }

  saveFeeApproval(approval: ChainFeeApproval): void {
    localStorage.setItem(`chain-fee-approval-v1:125:${this.creator}`, JSON.stringify(this.checkedFeeApproval(approval)));
  }

  restoreFeeApproval(): ChainFeeApproval | null {
    const body = localStorage.getItem(`chain-fee-approval-v1:125:${this.creator}`);
    if (body === null) return null;
    if (body.length > 4096) throw new Error('Your saved market fee approval is too large.');
    return this.checkedFeeApproval(JSON.parse(body));
  }

  clearFeeApproval(txHash: string): void {
    const saved = this.restoreFeeApproval();
    if (saved?.txHash === hash(txHash)) localStorage.removeItem(`chain-fee-approval-v1:125:${this.creator}`);
  }

  private checkedFeeApproval(approval: ChainFeeApproval): ChainFeeApproval {
    if (approval.chainId !== 125 || address(approval.creator) !== this.creator) throw new Error('Invalid saved market fee approval.');
    return {
      creator: this.creator, chainId: 125, token: address(approval.token) as ChainFeeApproval['token'],
      txHash: hash(approval.txHash) as ChainFeeApproval['txHash'], feeRaw: raw(approval.feeRaw).toString(),
    };
  }
}
