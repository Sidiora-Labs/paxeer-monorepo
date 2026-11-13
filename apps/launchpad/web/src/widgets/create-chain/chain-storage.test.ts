// @vitest-environment jsdom
import { expect, test, vi } from 'vitest';
import type { ChainSubmission } from './chain-adapters';
import type { ChainDraft } from './chain-domain';
import { ChainDraftStorage } from './chain-storage';

function chain_submissions_survive_editable_draft_expiry() {
  const time = 1700000000000;
  const maxDraftAge = 30 * 86400000;
  const creator = '0x1111111111111111111111111111111111111111';
  const otherCreator = '0x2222222222222222222222222222222222222222';
  const txHash = `0x${'ab'.repeat(32)}` as const;
  const otherHash = `0x${'cd'.repeat(32)}` as const;
  const submissionKey = 'chain-submission-v2';
  const draftKey = `chain-draft-v2:125:${creator}`;
  const draft: ChainDraft = {
    draftId: 'reviewed-chain', version: 2, creator, chainId: 125, name: 'my-chain',
    tokenName: 'My Token', tokenSymbol: 'MYT', feeStrategy: 2,
    tier: 'growth', feeShareBps: 1234, updatedAt: time,
  };
  const submission: ChainSubmission = {
    creator, chainId: 125, txHash, phase: 'confirm', bondRaw: '23000000000000000000',
  };

  vi.useFakeTimers({ toFake: ['Date'] });
  localStorage.clear();
  try {
    vi.setSystemTime(time);
    const owner = new ChainDraftStorage(creator);
    localStorage.setItem(`chain-draft-v1:125:${creator}`, JSON.stringify({ ...draft, version: 1 }));
    localStorage.setItem('chain-submission-v1', JSON.stringify({ draft: { ...draft, version: 1 }, submission }));
    expect(owner.restoreDraft()).toBeNull();
    expect(ChainDraftStorage.restoreSubmission()).toBeNull();
    expect(localStorage.getItem(`chain-draft-v1:125:${creator}`)).toBeNull();
    expect(localStorage.getItem('chain-submission-v1')).toBeNull();
    owner.saveDraft(draft);
    const draftBytes = localStorage.getItem(draftKey);
    expect(draftBytes).toBe(JSON.stringify(draft));
    vi.setSystemTime(time + maxDraftAge);
    expect(new ChainDraftStorage(creator).restoreDraft()).toEqual(draft);
    owner.saveDraft(draft);
    expect(localStorage.getItem(draftKey)).toBe(draftBytes);
    vi.setSystemTime(time + maxDraftAge + 1);
    expect(() => new ChainDraftStorage(creator).restoreDraft()).toThrow(/invalid or expired/);
    expect(() => owner.saveDraft(draft)).toThrow(/invalid or expired/);
    expect(localStorage.getItem(draftKey)).toBe(draftBytes);

    for (const phase of ['confirm', 'reverted'] as const) {
      vi.setSystemTime(time);
      const savedSubmission: ChainSubmission = { ...submission, phase };
      owner.saveSubmission(draft, savedSubmission);
      const bytes = localStorage.getItem(submissionKey);
      expect(bytes).toBe(JSON.stringify({ draft, submission: savedSubmission }));
      for (const elapsed of [maxDraftAge, maxDraftAge + 1]) {
        vi.setSystemTime(time + elapsed);
        const expected = { draft, submission: savedSubmission };
        expect(new ChainDraftStorage(creator).restoreSubmission()).toEqual(expected);
        expect(ChainDraftStorage.restoreSubmission()).toEqual(expected);
        const otherAccount = new ChainDraftStorage(otherCreator);
        expect(otherAccount.restoreSubmission()).toEqual(expected);
        otherAccount.clearSubmission(txHash);
        expect(localStorage.getItem(submissionKey)).toBe(bytes);
        new ChainDraftStorage(creator).clearSubmission(otherHash);
        expect(localStorage.getItem(submissionKey)).toBe(bytes);
      }
      new ChainDraftStorage(creator).clearSubmission(txHash);
      expect(localStorage.getItem(submissionKey)).toBeNull();
      expect(ChainDraftStorage.restoreSubmission()).toBeNull();
      expect(new ChainDraftStorage(otherCreator).restoreSubmission()).toBeNull();
    }

    vi.setSystemTime(time);
    owner.saveSubmission(draft, submission);
    const validBytes = localStorage.getItem(submissionKey);
    expect(validBytes).toBe(JSON.stringify({ draft, submission }));
    vi.setSystemTime(time + 2 * maxDraftAge + 1);
    const recovered = new ChainDraftStorage(creator).restoreSubmission()!;
    expect(recovered).toEqual({ draft, submission });
    const reverted: ChainSubmission = { ...recovered.submission, phase: 'reverted' };
    new ChainDraftStorage(creator).saveSubmission(recovered.draft, reverted);
    expect(localStorage.getItem(submissionKey)).toBe(JSON.stringify({ draft, submission: reverted }));
    expect(ChainDraftStorage.restoreSubmission()).toEqual({ draft, submission: reverted });
    expect(new ChainDraftStorage(creator).restoreSubmission()).toEqual({ draft, submission: reverted });
    new ChainDraftStorage(creator).clearSubmission(txHash);
    expect(ChainDraftStorage.restoreSubmission()).toBeNull();
    const freshDraft: ChainDraft = { ...draft, draftId: 'new-chain', name: 'fresh-chain', updatedAt: Date.now() };
    new ChainDraftStorage(creator).saveDraft(freshDraft);
    expect(new ChainDraftStorage(creator).restoreDraft()).toEqual(freshDraft);

    const malformedDraftFields = [
      ['draftId', ''], ['draftId', 'invalid identity'], ['draftId', 'a'.repeat(129)],
      ['version', 0], ['version', 1], ['version', 1.5], ['version', '1'],
      ['creator', 'invalid'], ['creator', `0x${'0'.repeat(40)}`], ['creator', otherCreator],
      ['chainId', 1], ['chainId', '125'],
      ['updatedAt', Date.now() + 1], ['updatedAt', null], ['updatedAt', '1700000000000'],
      ['updatedAt', time + 0.5], ['updatedAt', Number.MAX_SAFE_INTEGER + 1], ['updatedAt', undefined],
      ['tier', 'enterprise'], ['tier', 'toString'],
      ['feeShareBps', -1], ['feeShareBps', 5001], ['feeShareBps', 1.5], ['feeShareBps', '1234'],
      ['name', 'a'.repeat(33)], ['name', null],
      ['tokenName', ''], ['tokenSymbol', ''], ['feeStrategy', 4],
    ] as const;
    const malformedSubmissionFields = [
      ['creator', 'invalid'], ['creator', `0x${'0'.repeat(40)}`], ['creator', otherCreator],
      ['chainId', 1], ['chainId', '125'],
      ['txHash', '0x1234'], ['txHash', `0x${'gg'.repeat(32)}`], ['txHash', null],
      ['phase', 'done'], ['phase', ''], ['phase', null],
      ['bondRaw', '-1'], ['bondRaw', '01'], ['bondRaw', '1.5'], ['bondRaw', 23],
      ['bondRaw', (1n << 256n).toString()], ['bondRaw', null],
    ] as const;
    for (const [part, fields] of [
      ['draft', malformedDraftFields], ['submission', malformedSubmissionFields],
    ] as const) {
      for (const [field, value] of fields) {
        const malformed = JSON.parse(validBytes!) as { draft: ChainDraft; submission: ChainSubmission };
        const body = JSON.stringify({ ...malformed, [part]: { ...malformed[part], [field]: value } });
        localStorage.setItem(submissionKey, body);
        expect(() => ChainDraftStorage.restoreSubmission(), `${part}.${field}: ${String(value)}`).toThrow();
        expect(() => new ChainDraftStorage(creator).restoreSubmission()).toThrow();
        expect(() => new ChainDraftStorage(otherCreator).restoreSubmission()).toThrow();
        expect(() => owner.clearSubmission(txHash)).toThrow();
        const invalid = JSON.parse(body) as { draft: ChainDraft; submission: ChainSubmission };
        expect(() => owner.saveSubmission(invalid.draft, invalid.submission)).toThrow();
        expect(localStorage.getItem(submissionKey)).toBe(body);
      }
    }
    for (const body of ['{', 'null', '{}', JSON.stringify({ draft: null, submission }), `${validBytes}${' '.repeat(4096)}`]) {
      localStorage.setItem(submissionKey, body);
      expect(() => ChainDraftStorage.restoreSubmission()).toThrow();
      expect(() => new ChainDraftStorage(creator).restoreSubmission()).toThrow();
      expect(() => owner.clearSubmission(txHash)).toThrow();
      expect(localStorage.getItem(submissionKey)).toBe(body);
    }
  } finally {
    localStorage.clear();
    vi.useRealTimers();
  }
}

test(chain_submissions_survive_editable_draft_expiry.name, chain_submissions_survive_editable_draft_expiry);
