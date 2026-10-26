import { readFileSync } from 'node:fs';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterAll, beforeAll, describe, expect, expectTypeOf, it, test } from 'vitest';
import {
  JOURNEY_STATES,
  LADDER_RUNGS,
  decodeExplorerStatus,
  decodeJourney,
  readExplorerStatus,
  statusLadder,
  type ExplorerRung,
  type ExplorerTransactionStatus,
  type JourneyState,
  type LadderRung,
  type LadderSource,
  type SequencerRung,
} from '../src/index.js';

const fixtureDir = new URL('./fixtures/', import.meta.url);
const schemaDir = new URL('../../../services/human/schema/human-api/', import.meta.url);

const explorerAnswers = JSON.parse(readFileSync(new URL('explorer-status.json', fixtureDir), 'utf8')) as Record<
  SequencerRung,
  ExplorerTransactionStatus
>;
const explorerStatuses: Record<string, ExplorerTransactionStatus> = Object.fromEntries(
  Object.values(explorerAnswers).map((answer, index) => [`0x${`e${index + 1}`.repeat(32)}`, answer]),
);

let server: Server;
let base: string;

function handle(req: IncomingMessage, res: ServerResponse): void {
  const match = /^\/api\/v2\/transactions\/(0x[0-9a-f]{64})\/status$/.exec(req.url ?? '');
  const body = match ? explorerStatuses[match[1] as string] : undefined;
  if (req.method !== 'GET' || body === undefined) {
    res.writeHead(404, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ message: 'Not found' }));
    return;
  }
  res.writeHead(200, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

beforeAll(async () => {
  server = createServer(handle);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

afterAll(async () => {
  await new Promise<void>((resolve, reject) => server.close((err) => (err ? reject(err) : resolve())));
});

function schemaJourneyStates(): string[] {
  const text = readFileSync(new URL('journeys.kvx', schemaDir), 'utf8');
  const block = text.split('[type.JourneyState]')[1] ?? '';
  const variants = /variants = (\[[^\]]*\])/.exec(block);
  return JSON.parse(variants?.[1] ?? '[]') as string[];
}

describe('statusLadder', () => {
  function ladder_rungs_are_submitted_receipted_committed_in_order() {
    expectTypeOf<LadderRung>().toEqualTypeOf<SequencerRung>();
    expect(LADDER_RUNGS).toEqual(['submitted', 'receipted', 'committed']);
    expect(statusLadder.rungs).toBe(LADDER_RUNGS);
    expect(statusLadder.tables.explorer).toEqual({
      submitted: 'submitted',
      receipted: 'receipted',
      committed: 'committed',
    });
    for (const rung of LADDER_RUNGS) {
      expect(statusLadder.fromExplorer(rung)).toEqual({ rung, source: 'explorer', state: rung });
    }
    const steps = LADDER_RUNGS.map((rung) => statusLadder.fromExplorer(rung));
    expect(statusLadder.highest(steps)).toEqual({ rung: 'committed', source: 'explorer', state: 'committed' });
    expect(statusLadder.highest([...steps].reverse())).toEqual(steps[2]);
    expect(statusLadder.highest(steps.slice(0, 2))).toEqual(steps[1]);
  }

  test(ladder_rungs_are_submitted_receipted_committed_in_order);

  function ladder_journey_states_map_onto_the_three_rungs_without_an_anchor_source() {
    expectTypeOf<LadderSource>().toEqualTypeOf<'receipt' | 'explorer' | 'journey'>();
    const expected: Record<JourneyState, LadderRung | null> = {
      'getting-ready': null,
      sending: 'submitted',
      processing: 'receipted',
      done: 'receipted',
      'done-finalised': 'committed',
      'still-checking': null,
      refused: null,
      'waiting-for-you': null,
    };
    expect(statusLadder.tables.journey).toEqual(expected);
    expect(Object.keys(statusLadder.tables.journey)).toEqual([...JOURNEY_STATES]);
    expect(Object.keys(statusLadder.tables.journey)).toEqual(schemaJourneyStates());
    for (const state of JOURNEY_STATES) {
      expect(statusLadder.fromJourney(state)).toEqual({ rung: expected[state], source: 'journey', state });
    }
    expect(Object.keys(statusLadder.tables)).toEqual(['explorer', 'journey']);
    expect(statusLadder.tables).not.toHaveProperty('anchor');
    expect(statusLadder).not.toHaveProperty('fromAnchor');
    expect(() => statusLadder.fromJourney('pending' as JourneyState)).toThrow(RangeError);
    expect(() => statusLadder.fromExplorer('final' as SequencerRung)).toThrow(RangeError);
  }

  test(ladder_journey_states_map_onto_the_three_rungs_without_an_anchor_source);

  it('ladder_explorer_table_maps_every_explorer_rung', () => {
    expect(statusLadder.tables.explorer).toEqual({ submitted: 'submitted', receipted: 'receipted', committed: 'committed' });
    for (const [state, rung] of Object.entries(statusLadder.tables.explorer)) {
      expect(statusLadder.fromExplorer(state as SequencerRung)).toEqual({ rung, source: 'explorer', state });
    }
  });

  it('ladder_journey_table_maps_every_journey_state_in_the_schema', () => {
    expect(Object.keys(statusLadder.tables.journey)).toEqual(schemaJourneyStates());
    expect(Object.keys(statusLadder.tables.journey)).toEqual([...JOURNEY_STATES]);
    expect(statusLadder.tables.journey).toEqual({
      'getting-ready': null,
      sending: 'submitted',
      processing: 'receipted',
      done: 'receipted',
      'done-finalised': 'committed',
      'still-checking': null,
      refused: null,
      'waiting-for-you': null,
    });
    for (const [state, rung] of Object.entries(statusLadder.tables.journey)) {
      expect(statusLadder.fromJourney(state as JourneyState)).toEqual({ rung, source: 'journey', state });
    }
  });

  it('ladder_reads_recorded_explorer_statuses_onto_the_ladder', async () => {
    const rungs: Record<string, string | null> = {};
    for (const hash of Object.keys(explorerStatuses)) {
      const status = await readExplorerStatus(base, hash);
      expect(status).toEqual(explorerStatuses[hash]);
      rungs[status.rung] = statusLadder.fromExplorer(status).rung;
    }
    expect(rungs).toEqual({ submitted: 'submitted', receipted: 'receipted', committed: 'committed' });
    await expect(readExplorerStatus(base, `0x${'00'.repeat(32)}`)).rejects.toThrow('HTTP 404');
  });

  async function ladder_explorer_status_decodes_rung_block_number_and_committed_height() {
    expectTypeOf<ExplorerRung>().toEqualTypeOf<SequencerRung>();
    expectTypeOf<ExplorerTransactionStatus>().toEqualTypeOf<{
      rung: SequencerRung;
      block_number: number | null;
      committed_height: number | null;
    }>();
    expect(Object.keys(explorerAnswers)).toEqual(['submitted', 'receipted', 'committed']);
    for (const [hash, answer] of Object.entries(explorerStatuses)) {
      expect(decodeExplorerStatus(answer)).toEqual(answer);
      const status = await readExplorerStatus(base, hash);
      expect(status).toEqual(answer);
      expect(statusLadder.fromExplorer(status)).toEqual({ rung: answer.rung, source: 'explorer', state: answer.rung });
    }
    expect(decodeExplorerStatus({ ...explorerAnswers.submitted, committed_height: null })).toEqual({
      rung: 'submitted', block_number: null, committed_height: null,
    });
    const committed = explorerAnswers.committed;
    expect(decodeExplorerStatus({ ...committed, committed_height: committed.block_number })).toEqual({
      ...committed, committed_height: committed.block_number,
    });
    for (const field of ['block_number', 'committed_height']) {
      for (const value of [undefined, -1, 1.5, Number.MAX_SAFE_INTEGER + 1, NaN, Infinity, '1000']) {
        expect(() => decodeExplorerStatus({ ...committed, [field]: value })).toThrow(TypeError);
      }
    }
    for (const rung of ['pending', 'instant', 'sealed', 'final', 'unknown']) {
      expect(() => decodeExplorerStatus({ ...committed, rung })).toThrow(TypeError);
    }
  }

  test(ladder_explorer_status_decodes_rung_block_number_and_committed_height);

  function ladder_explorer_status_refuses_a_committed_answer_below_its_block() {
    const committed = explorerAnswers.committed;
    expect(committed.block_number).toBe(1000);
    expect(() => decodeExplorerStatus({ ...committed, committed_height: 999 })).toThrow(
      new TypeError('malformed explorer status'),
    );
    expect(() => decodeExplorerStatus({ ...committed, committed_height: null })).toThrow(
      new TypeError('malformed explorer status'),
    );
    expect(() => decodeExplorerStatus({ rung: committed.rung, block_number: committed.block_number })).toThrow(
      new TypeError('malformed explorer status'),
    );
  }

  test(ladder_explorer_status_refuses_a_committed_answer_below_its_block);

  it('ladder_maps_the_golden_journey_and_its_stages', () => {
    const recorded = JSON.parse(readFileSync(new URL('golden/journey.get.response.json', schemaDir), 'utf8')) as {
      body: { result: unknown };
    };
    const journey = decodeJourney(recorded.body.result);
    expect(statusLadder.fromJourney(journey.state)).toEqual({ rung: 'receipted', source: 'journey', state: 'processing' });
    const stages = journey.stages.map((stage) => statusLadder.fromJourney(stage.state));
    expect(stages.map((step) => step.rung)).toEqual(['receipted', 'receipted', 'receipted']);
    expect(statusLadder.highest(stages)).toEqual({ rung: 'receipted', source: 'journey', state: 'done' });
  });

  it('ladder_highest_picks_the_top_rung_across_sources', () => {
    const steps = [
      statusLadder.fromJourney('processing'),
      statusLadder.fromExplorer('committed'),
      statusLadder.fromJourney('still-checking'),
    ];
    expect(statusLadder.highest(steps)).toEqual({ rung: 'committed', source: 'explorer', state: 'committed' });
    expect(statusLadder.highest([statusLadder.fromJourney('refused')])).toEqual({ rung: null, source: 'journey', state: 'refused' });
    expect(statusLadder.highest([])).toBeNull();
    expect(statusLadder.rungs).toEqual(['submitted', 'receipted', 'committed']);
  });

  it('ladder_refuses_a_state_outside_its_tables', () => {
    expect(() => statusLadder.fromJourney('pending' as JourneyState)).toThrow(RangeError);
    expect(() => statusLadder.fromExplorer('confirmed' as SequencerRung)).toThrow(RangeError);
  });
});
