import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { readFileSync } from 'node:fs';
import path from 'node:path';
import { PERPS_COLLATERAL_QUERY } from '../perpsCollateral';

// collateralTokens response from services/platform/perps-data/internal/graphql/perps.go.
export class PerpsConfiguration {
  answer: unknown = JSON.parse(
    readFileSync(path.resolve(__dirname, 'fixtures/perps-collateral.json'), 'utf8')
  );
  readonly queries: unknown[] = [];
  status = 200;
  endpoint = '';
  private readonly server = createServer(async (request, response) => {
    let body = '';
    for await (const chunk of request) body += chunk;
    const query: unknown = JSON.parse(body);
    this.queries.push(query);
    if (
      request.method !== 'POST' ||
      request.url !== '/graphql' ||
      typeof query !== 'object' ||
      query === null ||
      !('query' in query) ||
      query.query !== PERPS_COLLATERAL_QUERY
    ) {
      response.writeHead(400).end();
      return;
    }
    response.writeHead(this.status, { 'content-type': 'application/json' });
    response.end(JSON.stringify(this.answer));
  });

  async open() {
    await new Promise<void>((resolve, reject) => {
      this.server.once('error', reject);
      this.server.listen(0, '127.0.0.1', resolve);
    });
    this.endpoint = `http://127.0.0.1:${(this.server.address() as AddressInfo).port}/graphql`;
  }

  async close() {
    this.server.closeAllConnections();
    await new Promise<void>((resolve, reject) =>
      this.server.close((error) => (error ? reject(error) : resolve()))
    );
  }
}
