import { writeFileSync } from 'node:fs';
import { describe, expect, it } from '../../web/node_modules/vitest/dist/index.js';
import { PrivateAccountCache, exactAmount, mergeWatchlist, portfolioLabels, profileMutation, profileConflict, watchlistReplay } from '../../web/src/widgets/profile/account-disclosure';
import { sharePreview } from '../../web/src/widgets/pnl/share-disclosure';
import { claimableSubtotal, rewardAmount, rewardPrograms } from '../../web/src/widgets/rewards/reward-disclosure';
const a = '0x' + 'a'.repeat(40), b = '0x' + 'b'.repeat(40);
const portfolio = { wallet: a, chainId: '125' as const, asOfHeight: '99', coverageStart: '2026-01-01', accountingMethodVersion: 'proportional_floor_v1', basisQuality: 'partial' as const, pricingQuality: 'partial' as const, pricedSubtotalUsdlRaw: '1000000', completeValueUsdlRaw: null, realizedPnlRaw: null, knownRealizedSubtotalUsdlRaw: '-2000000', markTimestamp: null };
const snapshot = { snapshotId: 'd17c3fdd-38aa-4ed2-aaf5-1f8fb299f913', accountingRevision: '7', intervalStart: '2026-01-01T00:00:00.000Z', intervalEnd: '2026-02-01T00:00:00.000Z', wallet: a, realized: '-1000000', unrealized: null, total: null };
const selection = { selectedFields: ['realized'] as const, expiresAt: '2026-03-01T00:00:00.000Z' };
describe('real disclosure primitives with synthetic financial inputs, no receipt claim', () => {
  it('discloses partial inventory without fabricated complete valuation', () => {
    const labels = portfolioLabels(portfolio);
    expect(labels).toEqual({
      inventory: "Coins you've traded here, separate from your wallet balance",
      method: 'proportional_floor_v1',
      coverage: 'Partial trade history since 2026-01-01',
      value: '—',
      subtotal: '1.000000 USDL',
      subtotalLabel: 'Value (some prices missing)',
      realized: '—',
      knownRealized: '-2.000000 USDL',
      mark: 'Price time not available yet',
    });
    writeFileSync(
      'launchpad/evidence/production-system/L31/phase1/partial-account.synthetic.json',
      JSON.stringify(labels),
    );
    expect(portfolioLabels({
      ...portfolio,
      completeValueUsdlRaw: '999999999',
      realizedPnlRaw: '999999999',
    })).toEqual(labels);
  });
  it('preserves zero, negatives and very large exact amounts', () => {
    expect(exactAmount('0')).toBe('0.000000 USDL'); expect(exactAmount('-1')).toBe('-0.000001 USDL'); expect(exactAmount('9007199254740993000001')).toBe('9007199254740993.000001 USDL'); expect(() => exactAmount('NaN')).toThrow();
  });
  it('clears account data immediately and rejects late requests and old session writes', () => {
    const cache = new PrivateAccountCache<string>(); const g = cache.select({ wallet: a, chainId: '125', sessionId: 'one' });
    expect(cache.commit(g, 'private A')).toBe(true); cache.select({ wallet: b, chainId: '125', sessionId: 'two' });
    expect(cache.read()).toBeUndefined(); expect(cache.commit(g, 'late A')).toBe(false); cache.clear(); expect(cache.read()).toBeUndefined(); expect(cache.commit(g, 'back button')).toBe(false);
    expect(() => cache.select({ wallet: a, chainId: '125', sessionId: '' })).toThrow();
  });
  it('previews exactly selected financial fields with anonymous default', () => {
    const result = sharePreview(snapshot, selection); expect(result.figures).toEqual({ realized: '-1.000000 USDL' }); expect(result).not.toHaveProperty('wallet'); expect(result).not.toHaveProperty('unrealized');
    expect(result).toEqual({
      snapshotId: snapshot.snapshotId,
      accountingRevision: '7',
      intervalStart: snapshot.intervalStart,
      intervalEnd: snapshot.intervalEnd,
      quoteCurrency: 'USDL',
      privacyMode: 'anonymous',
      expiresAt: selection.expiresAt,
      publicationStatus: 'preview',
      figures: { realized: '-1.000000 USDL' },
    });
    expect(Object.keys(result.figures)).toEqual(['realized']);
    const encoded = JSON.stringify(result);
    writeFileSync('launchpad/evidence/production-system/L31/phase1/share-preview.synthetic.json', encoded);
    expect(encoded).not.toContain(a); expect(encoded).not.toContain('unrealized');
    expect(encoded).not.toMatch(/<\/?(?:a|img|script)\b/i);
    expect(sharePreview(snapshot, { ...selection, privacyMode: 'wallet' })).toHaveProperty('wallet', a);
  });
  it('rejects unknown fields and malformed publication periods', () => {
    expect(() => sharePreview({...snapshot, snapshotId:'fake'},selection)).toThrow(); expect(() => sharePreview({...snapshot, intervalStart:'2026-01-01'},selection)).toThrow(); expect(() => sharePreview(snapshot, { ...selection, selectedFields: ['secret' as never] })).toThrow(); expect(() => sharePreview({ ...snapshot, intervalEnd: 'bad' }, selection)).toThrow();
  });
  it('keeps server order, requires explicit merge, normalizes identity and bounds capacity', () => {
    expect(mergeWatchlist([b], [a, b.toUpperCase().replace('0X','0x')], true)).toEqual([b,a]); expect(mergeWatchlist([b],[a],false)).toEqual([b]); expect(() => mergeWatchlist([a],['bad'],true)).toThrow(); expect(() => mergeWatchlist(Array(1001).fill(a),[],true)).toThrow();
  });
  it('reconciles offline revisions and rejects expired/cross-account replay', () => {
    const op = { mutationId: 'id', wallet: a, poolAddress: b, baseRevision: '4', expiresAt: 200 };
    expect(watchlistReplay(op,a,'5',100)).toBe('reconcile'); expect(watchlistReplay(op,b,'4',100)).toBe('wrong-account'); expect(watchlistReplay(op,a,'4',200)).toBe('expired'); expect(watchlistReplay(op,a,'4',100)).toBe('ready');
  });
  it('allowlists profile payload, preserves draft on conflict without auto retry', () => {
    expect(profileMutation(((1n<<256n)-1n).toString(),{})).toHaveProperty('expectedRevision'); expect(() => profileMutation((1n<<256n).toString(),{})).toThrow();
    const draft = { bio: 'unsaved', secret: 'not published' }; expect(profileMutation('3',draft)).toEqual({expectedRevision:'3',bio:'unsaved'}); expect(profileConflict(draft,'4')).toMatchObject({ draft, canAutoRetry:false }); expect(() => profileMutation('3',{bio:'x'.repeat(501)})).toThrow();
  });
  it('inventory has explicit blockers; failed entitlement is unknown not zero', () => {
    expect(rewardPrograms.every(p => p.authority && p.conditions && p.blocker)).toBe(true); expect(rewardPrograms.find(p=>p.type==='referral')?.settlement).toBeNull(); expect(() => rewardAmount(-1n)).toThrow(); expect(() => claimableSubtotal([-1n])).toThrow(); expect(() => rewardAmount(1n << 256n)).toThrow(); expect(rewardAmount(null)).toBe('—'); expect(rewardAmount(0n)).toBe('0.000000 USDL'); expect(claimableSubtotal([1000000n,null])).toEqual({raw:1000000n,coverage:'partial'});
  });
});
