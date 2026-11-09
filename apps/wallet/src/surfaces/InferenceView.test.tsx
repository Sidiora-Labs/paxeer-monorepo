// @vitest-environment jsdom
import { act, type ReactElement } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, expect, test } from 'vitest';
import { decodeFunctionData, parseAbi, parseUnits } from 'viem';
import {
  decodeInferenceRoom,
  decodeInferencePeer,
  fundAsk,
  type InferenceRoom,
} from '../../../../services/agent/sdk/typescript/src/inference_market';
import {
  buildInferenceFunding,
  InferenceAskForm,
  InferenceAskList,
  InferenceEarnings,
} from './InferenceView';
import { decodeInferenceCredit } from './useSurface';
import { SURFACE_ROUTES, type SurfaceId } from './routes';

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const id = (byte: string) => byte.repeat(32);
const settlementAddress = `0x${'11'.repeat(20)}`;
const roomWire = {
  room_id: id('01'),
  host_key: id('02'),
  model_digest: id('03'),
  runtime_digest: id('04'),
  profile: {
    minimum_bond: '100',
    canary_ratio_bps: 100,
    lap_size_tokens: '32',
    challenge_window_blocks: '10',
    slash_fraction_bps: 500,
    host_share_bps: 1000,
    protocol_share_bps: 100,
    settlement_account: id('05'),
  },
  state: 0,
  opened_at_epoch: '6',
  open_asks: [id('07')],
  open_bids: [id('08')],
  ask_streams: [{ ask_id: id('07'), stream_id: id('00'), stream: null }],
};
const room = decodeInferenceRoom(roomWire);
const peer = decodeInferencePeer({
  peer_key: id('0c'),
  worker: id('0d'),
  delegate: id('0e'),
  metadata: id('0f'),
  generation: '1',
  key_version: '2',
  metadata_revision: '3',
  valid_from: '4',
  expiry: '100',
  state: 2,
  enrolled: true,
  capabilities_status: 4,
  capabilities: null,
  bond_status: 2,
  fault_count: 9,
  attestation_binding: {
    family: 1,
    model_digest: id('03'),
    runtime_digest: id('04'),
    block_height: '6',
  },
});

let container: HTMLDivElement | null = null;
let root: Root | null = null;

async function mount(element: ReactElement): Promise<HTMLDivElement> {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root!.render(element));
  return container;
}

afterEach(async () => {
  if (root) await act(async () => root!.unmount());
  container?.remove();
  container = null;
  root = null;
});

async function enterField(container: HTMLElement, name: string, value: string): Promise<void> {
  const field = container.querySelector<HTMLInputElement | HTMLSelectElement>(`[name="${name}"]`);
  if (!field) throw new Error(`Missing field ${name}`);
  const prototype =
    field instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLSelectElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, 'value')?.set;
  if (!setter) throw new Error(`Missing native value setter for ${name}`);
  await act(async () => {
    setter.call(field, value);
    field.dispatchEvent(new Event('input', { bubbles: true }));
    field.dispatchEvent(new Event('change', { bubbles: true }));
  });
}

async function inference_form_encodes_stream_open_with_the_entered_caps() {
  const container = await mount(
    <InferenceAskForm rooms={[room]}>
      {(transaction) => <output data-role="encoded">{transaction?.data}</output>}
    </InferenceAskForm>
  );
  expect(container.querySelector('[data-role="preview"]')).toBeNull();
  await enterField(container, 'model', room.roomId);
  await enterField(container, 'askRef', id('07'));
  await enterField(container, 'settlementAddress', settlementAddress);
  await enterField(container, 'priceCap', '0.003');
  await enterField(container, 'totalCap', '12.5');

  const expected = fundAsk({
    askRef: id('07'),
    settlementAccount: settlementAddress,
    rate: parseUnits('0.000003', 18),
    rateUnit: 1,
    totalCap: parseUnits('12.5', 18),
    hostAuthorityKey: room.hostKey,
  });
  const encoded = container.querySelector('[data-role="encoded"]')?.textContent;
  expect(encoded).toBe(expected.data);
  expect(container.querySelector('[data-role="preview"]')?.textContent).toContain(expected.to);
  expect(container.querySelector('[data-role="build-error"]')).toBeNull();
  expect(
    decodeFunctionData({
      abi: parseAbi([
        'function streamOpenForAsk(bytes32 askRef, address settlementAccount, uint256 rate, uint8 rateUnit, uint256 totalCap, bytes32 hostAuthorityKey) returns (bytes32)',
      ]),
      data: expected.data as `0x${string}`,
    })
  ).toEqual({
    functionName: 'streamOpenForAsk',
    args: [
      `0x${id('07')}`,
      settlementAddress,
      3_000_000_000_000n,
      1,
      12_500_000_000_000_000_000n,
      `0x${id('02')}`,
    ],
  });

  await enterField(container, 'priceCap', '0.009');
  await enterField(container, 'totalCap', '25');
  expect(container.querySelector('[data-role="encoded"]')?.textContent).toBe(
    fundAsk({
      askRef: id('07'),
      settlementAccount: settlementAddress,
      rate: 9_000_000_000_000n,
      rateUnit: 1,
      totalCap: parseUnits('25', 18),
      hostAuthorityKey: room.hostKey,
    }).data
  );
}
test(
  inference_form_encodes_stream_open_with_the_entered_caps.name,
  inference_form_encodes_stream_open_with_the_entered_caps
);

async function inference_form_refuses_unrepresentable_and_zero_caps() {
  const container = await mount(
    <InferenceAskForm rooms={[room]}>
      {(transaction) => <output data-role="encoded">{transaction?.data}</output>}
    </InferenceAskForm>
  );
  await enterField(container, 'model', room.roomId);
  await enterField(container, 'askRef', id('07'));
  await enterField(container, 'settlementAddress', settlementAddress);
  await enterField(container, 'priceCap', '0.000000000000000001');
  await enterField(container, 'totalCap', '1');
  expect(container.querySelector('[data-role="build-error"]')?.textContent).toBe(
    'Enter a positive price per thousand with at most 15 decimal places.'
  );
  expect(container.querySelector('[data-role="encoded"]')?.textContent).toBe('');
  expect(container.querySelector('[data-role="preview"]')).toBeNull();
  await enterField(container, 'priceCap', '0.003');
  await enterField(container, 'totalCap', '0');
  expect(container.querySelector('[data-role="build-error"]')?.textContent).toBe(
    'Invalid inference market quantity'
  );
  expect(container.querySelector('[data-role="encoded"]')?.textContent).toBe('');
  expect(() =>
    buildInferenceFunding({ ...room, state: 1 }, id('07'), settlementAddress, '0.003', '1')
  ).toThrow('Choose an open room.');
  expect(() => buildInferenceFunding(room, id('08'), settlementAddress, '0.003', '1')).toThrow(
    'Choose an open ask in this room.'
  );
}
test(
  inference_form_refuses_unrepresentable_and_zero_caps.name,
  inference_form_refuses_unrepresentable_and_zero_caps
);

function fundedRoom(): InferenceRoom {
  return decodeInferenceRoom({
    ...roomWire,
    ask_streams: [
      {
        ask_id: id('07'),
        stream_id: id('09'),
        stream: {
          payer: id('0a'),
          stream_account: id('0b'),
          recipient: id('05'),
          asset_id: id('0d'),
          mode: 2,
          rate: '3000000000000',
          rate_unit: '1',
          start_ms: '1000',
          last_accrual_ms: '2000',
          end_ms: '0',
          total_cap: '50000000000000000000',
          accrued_total: '12000000000000000000',
          settled_total: '10000000000000000000',
          remainder_carry: '0',
          cumulative_meter: '9007199254740993',
          meter_authorities: [id('02')],
          underfunded: false,
          paused: false,
          closed: false,
        },
      },
    ],
  });
}

async function inference_ask_list_renders_fixture_reading_and_remaining_cap() {
  const fixture = fundedRoom();
  const container = await mount(<InferenceAskList rooms={[fixture]} />);
  const ask = container.querySelector(`[data-ask="${id('07')}"]`);
  expect(ask?.textContent).toContain(room.modelDigest);
  expect(ask?.textContent).toContain(id('09'));
  expect(ask?.querySelector('[data-field="reading"]')?.textContent).toBe('9007199254740993');
  expect(ask?.querySelector('[data-field="remaining-cap"]')?.textContent).toBe('38');
  expect(ask?.querySelector('[data-field="settled"]')?.textContent).toBe('10');
  expect(ask?.textContent).toContain('Open');
  expect(() => buildInferenceFunding(fixture, id('07'), settlementAddress, '0.003', '1')).toThrow(
    'This ask already has a funded stream.'
  );
}
test(
  inference_ask_list_renders_fixture_reading_and_remaining_cap.name,
  inference_ask_list_renders_fixture_reading_and_remaining_cap
);

async function inference_earnings_panel_renders_fixture_credit_for_the_peer() {
  const credit = decodeInferenceCredit({
    status: 'available',
    asset: `0x${id('a1')}`,
    earned: '9007199254741993',
    claimed: '1000',
  });
  const container = await mount(
    <InferenceEarnings
      earnings={{
        state: 'ready',
        value: { peer, credit: { state: 'ready', value: credit } },
      }}
    />
  );
  expect(container.querySelector('[aria-label="Peer earnings"]')?.textContent).toContain(
    peer.peerKey
  );
  expect(container.textContent).toContain('Enrolled: Yes');
  expect(container.textContent).toContain('Faults: 9');
  expect(container.textContent).toContain(id('a1'));
  expect(container.querySelector('[data-field="credit"]')?.textContent).toBe('9007199254741993');
  expect(container.querySelector('[data-field="claimed"]')?.textContent).toBe('1000');
  expect(container.querySelector('[data-field="claimable"]')?.textContent).toBe('9007199254740993');
}
test(
  inference_earnings_panel_renders_fixture_credit_for_the_peer.name,
  inference_earnings_panel_renders_fixture_credit_for_the_peer
);

async function inference_earnings_refuses_unavailable_or_invalid_credits() {
  const container = await mount(
    <InferenceEarnings
      earnings={{
        state: 'ready',
        value: {
          peer,
          credit: { state: 'unavailable', reason: 'Reward ledger credits are unavailable.' },
        },
      }}
    />
  );
  expect(container.querySelector('[role="status"]')?.textContent).toBe(
    'Reward ledger credits are unavailable.'
  );
  expect(container.querySelector('[data-field="credit"]')).toBeNull();
  expect(() =>
    decodeInferenceCredit({ status: 'not-yet-produced', asset: null, earned: null, claimed: null })
  ).toThrow('Reward ledger credits are unavailable.');
  expect(() =>
    decodeInferenceCredit({
      status: 'available',
      asset: `0x${id('a1')}`,
      earned: '10',
      claimed: '11',
    })
  ).toThrow('Claimed rewards exceed earned rewards');
  expect(() =>
    decodeInferenceCredit({
      status: 'available',
      asset: `0x${id('a1')}`,
      earned: '01',
      claimed: '0',
    })
  ).toThrow('Invalid reward quantity');
}
test(
  inference_earnings_refuses_unavailable_or_invalid_credits.name,
  inference_earnings_refuses_unavailable_or_invalid_credits
);

function inference_route_is_registered_with_a_surface_id() {
  const surface: SurfaceId = 'inference';
  expect(SURFACE_ROUTES.find((route) => route.id === surface)).toEqual({
    id: 'inference',
    href: '/surfaces/inference',
    label: 'Inference',
    description: 'Fund asks and view peer earnings',
  });
}
test(
  inference_route_is_registered_with_a_surface_id.name,
  inference_route_is_registered_with_a_surface_id
);
