import { assertControlEdges } from '@/core/app-foundation/control-edges.test-support';
// @vitest-environment jsdom
import React from 'react';
import { describe, expect, it } from 'vitest';
import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { CapabilityUnavailableError } from '@kindle/sdk';
import { toUnsupported } from '@/core/market/unsupported';
import { plainCopy } from '@/core/user-facing-error';
import { shortAddress } from '@/utils/format';
import { fallbackIdentity } from '@/core/market/use-wallet-identities';
import { CupHero } from './cups/cup-hero';
import { CupRulesDisclosure, rulesParagraphs } from './cups/cup-rules-disclosure';
import { cupBadge, cupHighlights, cupOperations, ladderTotalRaw, metricTone, prizeForRank, validateCup, validateStandings, type Cup, type CupStanding } from './cups/cups-contract';
import { isLeaderboardTab, parseTab } from './leaderboard-view';
import { parsePeriod, PeriodControl, periodWindows } from './period-control';
import { profileHref, RankRow } from './rank-row';
import { RankingTable } from './ranking-table';

const BORDER = /\b(border(?!-0\b|-none\b)(-[a-z0-9/\[\]-]+)?|divide-[a-z]|ring-(?!offset)[a-z0-9]|outline-(?!none|focus|2|offset)[a-z]+)\b/;
const wallet = '0xAbCdEf0123456789abcdef0123456789ABCDEF01';
const cupId = '7f3b2a10-5c4d-4e8f-9a1b-2c3d4e5f6a7b';

/** Text a reader sees: markup with every tag (and so every attribute) removed. */
function visibleText(html: string): string {
  return html.replace(/<[^>]*>/g, ' ');
}

function render(node: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return renderToStaticMarkup(<QueryClientProvider client={client}>{node}</QueryClientProvider>);
}

const cup: Cup = {
  cupId, name: 'Autumn solo cup', mode: 'solo', scoring: 'pnl', status: 'finished',
  startsAt: '2026-10-01T00:00:00Z', endsAt: '2026-10-31T23:59:59Z', prizePoolRaw: '100000000000',
  prizeLadder: [{ rank: 1, amountRaw: '50000000000' }, { rank: 2, amountRaw: '25000000000' }, { rank: 3, amountRaw: '15000000000' }, { rank: 4, amountRaw: '6000000000' }],
  rulesMarkdown: '# Scoring\nRealised PnL only.\n\nSelf-trades are excluded.',
};

const standings: CupStanding[] = [
  { rank: 2, wallet: '0x2222222222222222222222222222222222222222', metricSigned: '262473700000' },
  { rank: 1, wallet, metricSigned: '285822310000', prizeRaw: '50000000000' },
  { rank: 3, wallet: '0x3333333333333333333333333333333333333333', metricSigned: '-1500000' },
];

describe('cups contract guards', () => {
  it('accepts a well-formed cup and rejects ladder overflow, bad schedule and duplicate ranks', () => {
    expect(validateCup(cup)).toBe(cup);
    expect(() => validateCup({ ...cup, prizeLadder: [{ rank: 1, amountRaw: '100000000001' }] })).toThrow('exceeds the prize pool');
    expect(() => validateCup({ ...cup, endsAt: cup.startsAt })).toThrow('schedule');
    expect(() => validateCup({ ...cup, prizeLadder: [{ rank: 1, amountRaw: '1' }, { rank: 1, amountRaw: '1' }] })).toThrow('ladder rank');
    expect(() => validateCup({ ...cup, cupId: 'not-a-uuid' })).toThrow('identity');
  });
  it('sorts standings by rank and rejects duplicate wallets, ranks and malformed metrics', () => {
    expect(validateStandings(standings, 100).map(row => row.rank)).toEqual([1, 2, 3]);
    expect(() => validateStandings([...standings, { rank: 4, wallet: wallet.toLowerCase(), metricSigned: '1' }], 100)).toThrow('identity');
    expect(() => validateStandings([...standings, { rank: 1, wallet: '0x4444444444444444444444444444444444444444', metricSigned: '1' }], 100)).toThrow('rank');
    expect(() => validateStandings([{ rank: 1, wallet, metricSigned: '1.5' }], 100)).toThrow('shape');
    expect(() => validateStandings(standings, 2)).toThrow('page bound');
  });
  it('derives prizes, highlights, ladder totals, tones and badges from the record', () => {
    expect(prizeForRank(cup, standings[1])).toBe('50000000000');
    expect(prizeForRank(cup, standings[0])).toBe('25000000000');
    expect(prizeForRank(cup, { rank: 9 })).toBeNull();
    expect(ladderTotalRaw(cup)).toBe(96000000000n);
    expect(cupHighlights(standings).map(row => row.rank)).toEqual([1, 2, 3]);
    expect(metricTone('285822310000')).toBe('success');
    expect(metricTone('-1500000')).toBe('danger');
    expect(metricTone('0')).toBe('neutral');
    expect(cupBadge('finished')).toEqual({ status: 'finished', label: 'Finished' });
    expect(cupBadge('live').status).toBe('live');
    expect(cupBadge('scheduled').status).toBe('neutral');
  });
  it('names the typed IF_CUP_* operations', () => {
    expect(Object.values(cupOperations).sort()).toEqual(['IF_CUP_CURRENT', 'IF_CUP_GET', 'IF_CUP_RESULTS', 'IF_CUP_STANDINGS']);
  });
});

describe('period and tab mapping', () => {
  it('maps the hero segments onto IF_LEADERBOARDS windows', () => {
    expect(periodWindows).toEqual({ '1d': '24h', '1w': '7d', '1m': '30d' });
    expect(parsePeriod('1w')).toBe('1w');
    expect(parsePeriod('cup')).toBe('cup');
    expect(parsePeriod('7d')).toBe('1d');
    expect(parsePeriod(null, 'cup')).toBe('cup');
  });
  it('keeps the callouts tab for narrow viewports only', () => {
    expect(isLeaderboardTab('rewards')).toBe(true);
    expect(isLeaderboardTab('cups')).toBe(false);
    expect(parseTab('callouts', false)).toBe('callouts');
    expect(parseTab('callouts', true)).toBe('traders');
    expect(parseTab(null, true)).toBe('traders');
  });
  it('renders the four-segment radiogroup with the trophy segment labelled', () => {
    const html = render(<PeriodControl value="1d" onChange={() => {}} />);
    expect(html).toContain('role="radiogroup"');
    expect(html).toContain('aria-label="Competition"');
    expect(html.match(/role="radio"/g)).toHaveLength(4);
    expect(assertControlEdges(html)).not.toMatch(BORDER);
    expect(html).not.toMatch(/disabled=""/);
    const off = render(<PeriodControl value="1d" onChange={() => {}} cupDisabled />);
    expect(off.match(/disabled=""/g)).toHaveLength(1);
    expect(off).toContain('No competition to rank right now');
  });
});

describe('rank row', () => {
  it('renders podium tone, handle, short wallet, signed PnL, prize figure and the profile link', () => {
    const identity = { ...fallbackIdentity(wallet, 'profile'), displayName: 'hdegroot' };
    const html = renderToStaticMarkup(<RankingTable label="Traders"><RankRow rank={1} wallet={wallet} identity={identity} metricSigned="285822310000" prizeRaw="50000000000" size="lg" actions={<button type="button">Follow</button>} /></RankingTable>);
    expect(html).toContain('text-rank-1');
    expect(html).toContain('hdegroot');
    expect(html).toContain(`href="${profileHref(wallet)}"`);
    expect(html).toContain('+$285,822.31');
    expect(html).toContain('50,000 USDL');
    expect(html).toContain('text-success');
    expect(html).toContain('Follow');
    expect(html).toContain('h-list-row-lg');
    expect(assertControlEdges(html)).not.toMatch(BORDER);
  });
  it('falls back to the short address, colours losses and marks the viewer row', () => {
    const html = renderToStaticMarkup(<RankingTable label="Traders"><RankRow rank={7} wallet={wallet} metricSigned="-1500000" self /></RankingTable>);
    expect(html).toContain(shortAddress(wallet.toLowerCase(), 6, 4));
    expect(html).toContain('text-danger');
    expect(html).toContain('-$1.50');
    expect(html).toContain('>You<');
    // Names use the row-name step; figures use the numeric table step.
    expect(html).toContain('h-12');
    const doc = new DOMParser().parseFromString(html, 'text/html');
    const link = doc.querySelector('a')!;
    expect(link.className.split(/\s+/)).toEqual(expect.arrayContaining(['text-md', 'font-semibold']));
    expect(link.querySelector('span')!.className.split(/\s+/)).toContain('truncate');
    expect(html).toMatch(/class="text-num-md font-semibold[^"]*tabular-nums text-danger"/);
    expect(html).not.toContain('text-rank-');
  });
  it('shows an em dash when the metric is unavailable', () => {
    const html = renderToStaticMarkup(<RankingTable label="Traders"><RankRow rank={4} wallet={wallet} metricSigned={null} qualityNote="Not tracked yet" /></RankingTable>);
    expect(html).toContain('aria-label="no data yet"');
    expect(html).toContain('Not tracked yet');
  });
});

describe('cup hero and rules', () => {
  it('renders "No competition scheduled" from the real 404 without showing the operation or status', () => {
    const marker = toUnsupported(cupOperations.current, new CapabilityUnavailableError('Unavailable endpoint /v2/cups/current', 404));
    expect(marker).not.toBeNull();
    const html = render(<CupHero cup={marker!} onRetry={() => {}} />);
    expect(html).toContain('No competition scheduled');
    expect(html).not.toContain('IF_CUP_CURRENT');
    expect(html).not.toContain('404');
    expect(html).toContain('Prizes and results show up here once cups start');
    expect(html).not.toContain('Try again');
    const doc = new DOMParser().parseFromString(html, 'text/html');
    expect(doc.querySelectorAll('h2')).toHaveLength(1);
    expect(doc.querySelector('[data-state="unavailable"], [role="alert"]')).toBeNull();
    expect(doc.querySelector('svg')?.classList.contains('text-text-faint')).toBe(true);
    expect(assertControlEdges(html)).not.toMatch(BORDER);
  });
  it('renders a served null cup as a genuine empty answer without a retry', () => {
    const html = render(<CupHero cup={{ state: 'ready', data: null }} onRetry={() => {}} />);
    expect(html).toContain('No competition scheduled');
    expect(html).toContain('Prizes and results show up here once cups start');
    expect(html).not.toContain('IF_CUP_CURRENT');
    expect(html).not.toContain('Try again');
  });
  it('renders a genuine failure with neutral wording and a retry', () => {
    const html = render(<CupHero cup={{ state: 'error', error: new Error('socket hang up'), retry: () => {} }} onRetry={() => {}} />);
    expect(html).toContain("Couldn&#x27;t load the cup");
    expect(html).toContain(plainCopy.network);
    // The raw transport text may only survive as the primitive's developer-detail attributes (non-production), never as visible copy.
    expect(visibleText(html)).not.toContain('socket hang up');
    expect(html).toContain('role="alert"');
    expect(html).toMatch(/Try again|Retry/);
    expect(html).not.toMatch(/did not load|could not load|failed/i);
  });
  it('renders the finished cup name, badge and prize pool', () => {
    const html = render(<CupHero cup={{ state: 'ready', data: cup }} onRetry={() => {}} />);
    expect(html).toContain('Autumn solo cup');
    expect(html).toContain('Finished');
    expect(html).toContain('100,000 USDL');
    expect(html).toContain('Solo cup');
    expect(assertControlEdges(html)).not.toMatch(BORDER);
  });
  it('keeps the period control above one compact state without an invented prize or rules', () => {
    const control = <PeriodControl value="1d" onChange={() => {}} cupDisabled />;
    for (const state of [
      { state: 'error' as const, error: new Error('x'), retry: () => {} },
      { state: 'ready' as const, data: null },
      { state: 'loading' as const },
    ]) {
      const html = render(<CupHero cup={state} onRetry={() => {}} control={control} />);
      const doc = new DOMParser().parseFromString(html, 'text/html');
      expect(html).toContain('aria-label="Ranking period"');
      expect(html).not.toContain('USDL');
      expect(html).not.toContain('before:aspect-');
      expect(doc.querySelector('details, [data-cup-art]')).toBeNull();
      expect(doc.querySelector('section')?.className).toBe('flex min-w-0 flex-col gap-3');
      if (state.state === 'loading') {
        expect(doc.querySelector('[aria-busy="true"]')).not.toBeNull();
        expect(doc.querySelectorAll('.sl-skeleton')).toHaveLength(3);
      } else {
        expect(doc.querySelectorAll('h2')).toHaveLength(1);
        expect(doc.querySelector('[role="alert"]') !== null).toBe(state.state === 'error');
        expect(doc.querySelector('button:not([role="radio"])') !== null).toBe(state.state === 'error');
      }
      expect(assertControlEdges(html)).not.toMatch(BORDER);
      expect(html).not.toMatch(/\bshadow/);
    }
  });
  it('shows one neutral first-period form and the served prize for a scheduled cup', () => {
    const html = render(<CupHero cup={{ state: 'ready', data: { ...cup, status: 'scheduled' } }} onRetry={() => {}} />);
    const doc = new DOMParser().parseFromString(html, 'text/html');
    expect(html).toContain('Standings open when the cup starts');
    expect(html).toContain('100,000 USDL');
    expect(doc.querySelectorAll('[class*="@container/state"]')).toHaveLength(1);
    expect(doc.querySelector('[role="alert"]')).toBeNull();
    expect(doc.querySelector('button')).toBeNull();
  });
  it('places the rules row and the control inside a ready hero', () => {
    const html = render(<CupHero cup={{ state: 'ready', data: cup }} onRetry={() => {}} control={<PeriodControl value="cup" onChange={() => {}} />} />);
    expect(html.indexOf('aria-label="Ranking period"')).toBeLessThan(html.indexOf('Autumn solo cup'));
    expect(html.indexOf('100,000 USDL')).toBeLessThan(html.indexOf('Schedule, scoring, eligibility and rules'));
    expect(html.lastIndexOf('</section>')).toBeGreaterThan(html.indexOf('Schedule, scoring, eligibility and rules'));
  });
  it('turns rules markdown into plain paragraphs and renders the prize ladder table', () => {
    expect(rulesParagraphs(cup.rulesMarkdown)).toEqual(['Scoring Realised PnL only.', 'Self-trades are excluded.']);
    const html = render(<CupRulesDisclosure cup={cup} />);
    expect(html).toContain('Schedule, scoring, eligibility and rules');
    expect(html).toContain('aria-expanded="false"');
    expect(assertControlEdges(html)).not.toMatch(BORDER);
  });
});

describe('leaderboard tabs', () => {
  it('opens the traders board for a squads address', () => {
    expect(isLeaderboardTab('squads')).toBe(false);
    expect(parseTab('squads', false)).toBe('traders');
    expect(parseTab('squads', true)).toBe('traders');
  });
});
