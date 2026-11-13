import { EndpointClient, JsonRpcTransportError } from '@paxeer/wallet';
import { expect, test } from 'vitest';
import { SCENARIOS, recordings, startAccountEndpoint } from './endpoint';
import './endpoint.test';

async function asset_height_test_preserves_numeric_wire_and_sdk_normalization() {
  const endpoint = await startAccountEndpoint();
  try {
    const url = endpoint.config('merged').endpointUrl;
    const response = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: 'assets', method: 'px_listAssets', params: [] }),
    });
    expect(response.status).toBe(200);
    const wire = await response.json();
    expect(wire.jsonrpc).toBe('2.0');
    expect(wire.id).toBe('assets');
    expect(wire.result.height).toBe(1000);
    expect(typeof wire.result.height).toBe('number');

    const client = new EndpointClient({ url, hostedUrl: url });
    const page = await client.listBankAssets();
    expect(page).toEqual({ ...wire.result, height: '1000' });
    expect(typeof page.height).toBe('string');
    expect(endpoint.methods('merged')).toEqual(['px_listAssets', 'px_listAssets']);
    expect(endpoint.seen.map((entry) => entry.path)).toEqual(['/', '/']);
    expect(endpoint.unmatched).toEqual([]);
    expect(endpoint.refusals).toEqual([]);
  } finally {
    await endpoint.close();
  }
}

test(
  asset_height_test_preserves_numeric_wire_and_sdk_normalization.name,
  asset_height_test_preserves_numeric_wire_and_sdk_normalization
);

async function asset_height_test_refuses_recorded_page_without_height() {
  const endpoint = await startAccountEndpoint();
  try {
    const url = endpoint.config('available').endpointUrl;
    const client = new EndpointClient({ url, hostedUrl: url });
    const raw = await client.batch([{ method: 'px_listAssets', params: [] }]);
    const recorded = recordings('available').find(
      (entry) => entry.request.method === 'px_listAssets'
    );
    if (!recorded) throw new Error('missing recorded asset page without height');
    expect(raw).toEqual([{ ok: true, result: recorded.response.result }]);
    expect(raw[0]).toHaveProperty('result.assets');
    expect(raw[0]).not.toHaveProperty('result.height');
    const malformedPage = client.listBankAssets();
    await expect(malformedPage).rejects.toBeInstanceOf(JsonRpcTransportError);
    await expect(malformedPage).rejects.toHaveProperty(
      'message',
      'malformed bank asset page height'
    );
    expect(endpoint.methods('available')).toEqual(['px_listAssets', 'px_listAssets']);
    expect(endpoint.seen.map((entry) => entry.path)).toEqual(['/', '/']);
    expect(endpoint.unmatched).toEqual([]);
    expect(endpoint.refusals).toEqual([]);
  } finally {
    await endpoint.close();
  }
}

test(
  asset_height_test_refuses_recorded_page_without_height.name,
  asset_height_test_refuses_recorded_page_without_height
);

async function fixture_router_path_isolates_scenarios_at_root_origins() {
  const endpoint = await startAccountEndpoint();
  try {
    const origins = SCENARIOS.map((scenario) => {
      const url = new URL(endpoint.config(scenario).endpointUrl);
      expect(url.pathname).toBe('/');
      expect(url.search).toBe('');
      expect(url.hash).toBe('');
      return url.origin;
    });
    expect(new Set(origins).size).toBe(SCENARIOS.length);
    for (const scenario of SCENARIOS) {
      const url = endpoint.config(scenario).endpointUrl;
      const network = recordings(scenario).find(
        (entry) => entry.request.method === 'px_getNetwork'
      );
      if (!network) throw new Error(`missing recorded network for ${scenario}`);
      const client = new EndpointClient({ url, hostedUrl: url });
      expect(await client.batch([network.request, network.request])).toEqual([
        { ok: true, result: network.response.result },
        { ok: true, result: network.response.result },
      ]);
      expect(endpoint.methods(scenario)).toEqual(['px_getNetwork', 'px_getNetwork']);
      expect(endpoint.seen.filter((entry) => entry.scope === scenario)).toMatchObject([
        { scope: scenario, path: '/', methods: ['px_getNetwork', 'px_getNetwork'] },
      ]);
    }
    expect(endpoint.unmatched).toEqual([]);
    expect(endpoint.refusals).toEqual([]);

    const unmatched: string[] = [];
    for (const scenario of SCENARIOS) {
      const url = endpoint.config(scenario).endpointUrl;
      for (const route of ['/rpc', `/${scenario}/rpc`, '/available/']) {
        const response = await fetch(new URL(route, url), {
          method: 'POST',
          headers: { 'content-type': 'application/json' },
          body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'px_getNetwork', params: [] }),
        });
        expect(response.status).toBe(404);
        expect(await response.json()).toEqual({ error: 'unknown route' });
        unmatched.push(`POST ${route}`);
      }
      expect(endpoint.methods(scenario)).toEqual(Array(5).fill('px_getNetwork'));
    }
    expect(endpoint.unmatched).toEqual(unmatched);
  } finally {
    await endpoint.close();
  }
}

test(
  fixture_router_path_isolates_scenarios_at_root_origins.name,
  fixture_router_path_isolates_scenarios_at_root_origins
);
