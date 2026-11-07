import { address, hash, raw } from '../create/creation-domain';
import type { ChainSubmission } from './chain-adapters';
import { validateChainDraft, type ChainDraft } from './chain-domain';

const submissionKey = 'chain-submission-v1';

export class ChainDraftStorage {
  private creator: string;

  constructor(creator: string) {
    this.creator = address(creator);
  }

  private key() {
    return `chain-draft-v1:125:${this.creator}`;
  }

  private checked(draft: ChainDraft, kind: 'draft' | 'submission'): ChainDraft {
    const now = Date.now();
    const errors = validateChainDraft(draft, kind === 'draft' ? now : draft?.updatedAt);
    delete errors.name;
    if (Object.keys(errors).length || draft.version !== 1 || address(draft.creator) !== this.creator
      || !Number.isSafeInteger(now) || draft.updatedAt > now
      || typeof draft.name !== 'string' || draft.name.length > 32) {
      throw new Error('Your saved chain draft is invalid or expired.');
    }
    const { draftId, version, creator, chainId, name, tier, feeShareBps, updatedAt } = draft;
    return { draftId, version, creator, chainId, name, tier, feeShareBps, updatedAt };
  }

  saveDraft(draft: ChainDraft): void {
    localStorage.setItem(this.key(), JSON.stringify(this.checked(draft, 'draft')));
  }

  restoreDraft(): ChainDraft | null {
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
}
