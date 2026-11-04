import { once } from 'node:events';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import React from 'react';

import config from 'configs/app';
import { MultichainContext } from 'lib/contexts/multichain';
import { render } from 'ui/shared/layout/testWrapper';

import { counters } from './numberWidgetsFixtures';
import NumberWidgetsList from './NumberWidgetsList';

export async function startCountersServer() {
  const requests: Array<string> = [];
  const server = createServer((request, response) => {
    requests.push(`${ request.method } ${ request.url }`);
    if (request.method !== 'GET' || request.url !== '/api/v1/counters') {
      response.writeHead(404).end();
      return;
    }

    response.writeHead(200, { 'Content-Type': 'application/json' });
    response.end(JSON.stringify(counters));
  });

  server.listen(0, '127.0.0.1');
  await once(server, 'listening');

  const endpoint = `http://127.0.0.1:${ (server.address() as AddressInfo).port }`;

  return {
    endpoint,
    requests,
    close: () => new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    }),
  };
}

export function renderNumberWidgets(endpoint: string, props: React.ComponentProps<typeof NumberWidgetsList> = {}) {
  return render(React.createElement(MultichainContext.Provider, {
    value: {
      chain: {
        id: '1',
        name: 'Recorded stats',
        slug: 'recorded-stats',
        app_config: { ...config, apis: { ...config.apis, stats: { endpoint, basePath: '' } } },
      },
    },
  }, React.createElement(NumberWidgetsList, props)));
}
