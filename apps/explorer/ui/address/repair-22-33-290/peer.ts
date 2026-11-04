import { createServer } from 'node:http';
import type { ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { WebSocketServer } from 'ws';
import type { WebSocket } from 'ws';

import type { AddressCoinBalanceHistoryItem, AddressCoinBalanceHistoryResponse } from 'types/api/address';

import * as address from 'mocks/address/address';
import * as counters from 'mocks/address/counters';
import * as tokens from 'mocks/address/tokens';
import * as contract from 'mocks/contract/info';
import { nativeUnifiedAccount } from 'mocks/paxeerX/unifiedAccount';
import * as transfers from 'mocks/tokens/tokenTransfer';
import * as internal from 'mocks/txs/internalTxs';

type Frame = [string | null, string | null, string, string, unknown];

// An in-memory history store serves both HTTP pages and live notifications. Its wire fields match
// AddressView.prepare_coin_balance_history_entry/1; both transports read the same stored entry.
export class AddressRepairPeer {
  readonly requests: Array<URL> = [];
  readonly changes = new Map<number, AddressCoinBalanceHistoryItem>();
  readonly joins = new Map<WebSocket, Frame>();
  readonly http = createServer((request, response) => this.respond(new URL(request.url!, 'http://localhost'), response));
  readonly sockets = new WebSocketServer({ server: this.http });
  stalled = false;
  historyUnavailable = false;
  chartUnavailable = false;
  balance = BigInt(0);
  block = 0;

  constructor() {
    this.sockets.on('connection', (socket) => {
      socket.on('close', () => this.joins.delete(socket));
      socket.on('message', (bytes) => {
        const frame = JSON.parse(bytes.toString()) as Frame;
        if (frame[3] === 'phx_join') {
          this.joins.set(socket, frame);
          if (!this.stalled) {
            this.reply(socket, frame);
          }
        } else if (frame[3] === 'heartbeat' || frame[3] === 'phx_leave') {
          this.reply(socket, frame);
        }
      });
    });
  }

  private reply(socket: WebSocket, frame: Frame) {
    socket.send(JSON.stringify([ frame[0], frame[1], frame[2], 'phx_reply', { status: 'ok', response: {} } ]));
  }

  async start() {
    await new Promise<void>(resolve => this.http.listen(0, '127.0.0.1', resolve));
    return (this.http.address() as AddressInfo).port;
  }

  async close() {
    this.sockets.clients.forEach(socket => socket.terminate());
    await new Promise<void>((resolve, reject) => this.sockets.close(error => error ? reject(error) : resolve()));
    await new Promise<void>((resolve, reject) => this.http.close(error => error ? reject(error) : resolve()));
  }

  disconnect() {
    this.stalled = true;
    this.joins.clear();
    this.sockets.clients.forEach(socket => socket.terminate());
  }

  rejoin() {
    this.stalled = false;
    this.joins.forEach((frame, socket) => this.reply(socket, frame));
  }

  changeBalance(delta: bigint, publish = true) {
    this.balance += delta;
    this.block++;
    const row: AddressCoinBalanceHistoryItem = {
      block_number: this.block,
      block_timestamp: new Date(Date.UTC(2025, 0, 1, 0, this.block)).toISOString(),
      transaction_hash: null,
      delta: delta.toString(),
      value: this.balance.toString(),
    };
    this.changes.set(this.block, row);
    if (publish) {
      this.deliver(row);
    }
    return row;
  }

  deliver(row: AddressCoinBalanceHistoryItem) {
    if (this.stalled) {
      return;
    }
    this.joins.forEach((frame, socket) => {
      socket.send(JSON.stringify([ frame[0], null, frame[2], 'coin_balance', { coin_balance: row } ]));
    });
  }

  history(before = Infinity): AddressCoinBalanceHistoryResponse {
    const rows = Array.from(this.changes.values()).filter(row => row.block_number < before).reverse();
    const items = rows.slice(0, 50);
    return {
      items,
      next_page_params: rows.length > items.length ? { block_number: items[items.length - 1].block_number, items_count: 50 } : null,
    };
  }

  chart() {
    return { items: [ { date: '2025-01-01', value: this.balance.toString() } ], days: 1 };
  }

  reset() {
    this.requests.length = 0;
    this.changes.clear();
    this.balance = BigInt(0);
    this.block = 0;
    this.stalled = false;
    this.historyUnavailable = false;
    this.chartUnavailable = false;
  }

  private respond(url: URL, response: ServerResponse) {
    this.requests.push(url);
    response.setHeader('Content-Type', 'application/json');
    response.setHeader('Connection', 'close');
    const path = url.pathname;
    const second = url.searchParams.has('index') || url.searchParams.has('token_contract_address_hash') || url.searchParams.has('id');
    let data: unknown;
    if (path.endsWith('/coin-balance-history-by-day')) {
      if (this.chartUnavailable) {
        response.statusCode = 503;
      }
      data = this.chart();
    } else if (path.endsWith('/coin-balance-history')) {
      if (this.historyUnavailable) {
        response.statusCode = 503;
      }
      data = this.history(Number(url.searchParams.get('block_number') ?? Infinity));
    } else if (path.endsWith('/unified')) {
      data = nativeUnifiedAccount;
    } else if (path.endsWith('/counters')) {
      data = counters.forAddress;
    } else if (path.includes('/smart-contracts/')) {
      data = contract.verified;
    } else if (path.endsWith('/internal-transactions')) {
      const selected = internal.baseResponse.items.filter(row => !url.searchParams.get('filter') ||
        (url.searchParams.get('filter') === 'from' ? row.from.hash : row.to?.hash) === address.hash);
      data = { items: selected.slice(second ? 1 : 0, second ? 2 : 1), next_page_params: second ? null : { index: 1, block_number: 29611822 } };
    } else if (path.endsWith('/token-transfers')) {
      const rows = [ transfers.erc20, { ...transfers.erc20, log_index: '2', transaction_hash: internal.typeStaticCall.transaction_hash }, transfers.erc721 ];
      const selected = rows.filter(row => (!url.searchParams.get('type') ||
        (row.token !== null && url.searchParams.get('type')!.split(',').includes(row.token.type))) &&
        (!url.searchParams.get('filter') || row.from.hash === address.hash) &&
        (!url.searchParams.get('token') || row.token?.address_hash === url.searchParams.get('token')));
      data = { items: selected.slice(second ? 1 : 0, second ? 2 : 1), next_page_params: second ? null : { index: 1, block_number: 12345 } };
    } else if (path.endsWith('/nft/collections') || path.endsWith('/nft')) {
      const collection = path.endsWith('/collections');
      const matchesType = (type: string) => !url.searchParams.get('type') || url.searchParams.get('type')!.split(',').includes(type);
      const selected = collection ?
        tokens.collections.items.filter(row => matchesType(row.token.type)) :
        tokens.nfts.items.filter(row => matchesType(row.token.type));
      data = {
        items: selected.slice(second ? 1 : 0, second ? 2 : 1),
        next_page_params: second || selected.length <= 1 ? null : { token_contract_address_hash: 'next', id: 'next' },
      };
    } else if (path === `/api/v2/addresses/${ address.hash }`) {
      data = address.validator;
    } else {
      data = { items: [], next_page_params: null };
    }
    response.end(JSON.stringify(data));
  }
}
