import { address } from '../create/creation-domain';
import { validateChainDraft, type ChainDraft } from './chain-domain';

export class ChainDraftStorage {
  private creator: string;

  constructor(creator: string) {
    this.creator = address(creator);
  }

  private key() {
    return `chain-draft-v1:125:${this.creator}`;
  }

  private checked(draft: ChainDraft): ChainDraft {
    const errors = validateChainDraft(draft, Date.now());
    delete errors.name;
    if (Object.keys(errors).length || draft.version !== 1 || address(draft.creator) !== this.creator
      || typeof draft.name !== 'string' || draft.name.length > 32) {
      throw new Error('Your saved chain draft is invalid or expired.');
    }
    const { draftId, version, creator, chainId, name, tier, feeShareBps, updatedAt } = draft;
    return { draftId, version, creator, chainId, name, tier, feeShareBps, updatedAt };
  }

  saveDraft(draft: ChainDraft): void {
    localStorage.setItem(this.key(), JSON.stringify(this.checked(draft)));
  }

  restoreDraft(): ChainDraft | null {
    const body = localStorage.getItem(this.key());
    if (body === null) return null;
    if (body.length > 4096) throw new Error('Your saved chain draft is too large.');
    return this.checked(JSON.parse(body));
  }

  clearDraft(): void {
    localStorage.removeItem(this.key());
  }
}
