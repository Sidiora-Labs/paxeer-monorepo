import { after, before, test } from 'node:test';
import assert from 'node:assert/strict';
import { execFile, spawn } from 'node:child_process';
import { once } from 'node:events';
import { chmod, chown, copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { CandlesClient, CandlesHttpError, UdfResponseError, getHistory, httpClient, parseHistory } from '../dist/index.js';

// Task 22.31.72: audit 22.30.41 finding 1 is proved by finding1VendorPageBounds;
// finding 2 is proved by finding2ForwardPagination and finding2CountbackPagination.
// No producer wiring change is required outside this task's paths.
const run = promisify(execFile);
const root = fileURLToPath(new URL('../../../../..', import.meta.url));
const charts = join(root, 'services/launchpad/trading-charts');
const symbol = '0x1111111111111111111111111111111111111111';
const rowCount = 10003;
const request = { symbol, resolution: '1', from: 60, to: (rowCount + 1) * 60 };
const fields = ['t', 'o', 'h', 'l', 'c', 'v'];
let directory;
let pgBin;
let postgresStarted = false;
let postgresOptions;
let producer;
let baseUrl;
const client = new CandlesClient();

const goProducer = String.raw`package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dsn := os.Getenv("CANDLES_REPAIR_POSTGRES_URL")
	if err := migrate.Up(ctx, dsn); err != nil { return err }
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil { return err }
	defer pool.Close()
	const address = "0x1111111111111111111111111111111111111111"
	_, err = pool.Exec(ctx, ` + '`' + `INSERT INTO candles.candles
		(pool_address,timeframe,candle_start,open,high,low,close,volume_usdl,last_trade_ts,sequence_num)
		SELECT $1,'1m',i*60,'1000000000000000000','2000000000000000000',
		'1000000000000000000','2000000000000000000','1000001',i*60,10004-i
		FROM generate_series(1,10003) i` + '`' + `, address)
	if err != nil { return err }
	source := httpapi.PostgresHistorySource{Pool: pool}
	rows, err := source.QueryHistory(ctx, httpapi.HistoryRequest{
		Symbol: address, Timeframe: "1m", From: 60, To: 600240,
	}, 2001)
	if err != nil { return err }
	if len(rows) != 2001 || httpapi.MaxHistoryBars != 5000 || httpapi.MaxCanonicalHistoryBars != 2000 {
		return fmt.Errorf("producer history bounds changed")
	}
	if _, err := httpapi.CanonicalHistory(rows[:2000]); err != nil { return err }
	if _, err := httpapi.CanonicalHistory(rows); err == nil {
		return fmt.Errorf("canonical history admitted 2001 bars")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil { return err }
	mux := http.NewServeMux()
	mux.Handle("GET /history", httpapi.HistoryHandler{Source: source, SigningKey: key})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return err }
	server := &http.Server{Handler: mux, ReadHeaderTimeout: time.Second}
	stopping, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-stopping.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	fmt.Println("http://" + listener.Addr().String())
	err = server.Serve(listener)
	if err == http.ErrServerClosed { return nil }
	return err
}
`;

after(async function closeRealProducer() {
  if (producer && producer.exitCode === null && producer.signalCode === null) {
    const closed = once(producer, 'close');
    process.kill(-producer.pid, 'SIGTERM');
    await closed;
  }
  if (postgresStarted) {
    await run(join(pgBin, 'pg_ctl'), ['-D', join(directory, 'postgres'), '-m', 'immediate', '-w', 'stop'], postgresOptions);
  }
  if (directory) await rm(directory, { recursive: true, force: true });
});

before(async function openRealProducer() {
  directory = await mkdtemp(join(tmpdir(), 'candles-sdk-22-31-72-'));
  await chmod(directory, 0o755);
  const socket = join(directory, 'socket');
  const data = join(directory, 'postgres');
  await mkdir(socket);
  await mkdir(data);
  postgresOptions = { timeout: 60000, maxBuffer: 1024 * 1024 };
  if (process.getuid?.() === 0) {
    const { stdout: uid } = await run('id', ['-u', 'nobody']);
    const { stdout: gid } = await run('id', ['-g', 'nobody']);
    postgresOptions.uid = Number(uid.trim());
    postgresOptions.gid = Number(gid.trim());
    await chown(socket, postgresOptions.uid, postgresOptions.gid);
    await chown(data, postgresOptions.uid, postgresOptions.gid);
  }
  pgBin = process.env.PG_BINDIR ?? (await run('pg_config', ['--bindir'])).stdout.trim();
  await run(join(pgBin, 'initdb'), ['-D', data, '-A', 'trust', '--no-locale', '-U', 'postgres'], postgresOptions);
  await run(join(pgBin, 'pg_ctl'), [
    '-D', data, '-l', join(data, 'server.log'), '-o', `-F -k ${socket} -h ''`, '-w', 'start',
  ], postgresOptions);
  postgresStarted = true;

  // A temporary module under the producer's import namespace can use its real
  // internal packages without writing a helper into the Go domain's checkout.
  let goMod = await readFile(join(charts, 'go.mod'), 'utf8');
  goMod = goMod.replace(/^module (.+)$/m, 'module $1/candles-sdk-repair');
  goMod = goMod.replaceAll('=> ../../../sdk/launchpad/shared', `=> ${JSON.stringify(join(root, 'sdk/launchpad/shared'))}`);
  goMod = goMod.replaceAll('=> ../../../sdk/launchpad/protocol', `=> ${JSON.stringify(join(root, 'sdk/launchpad/protocol'))}`);
  goMod += `\nrequire github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts v0.0.0\n`;
  goMod += `replace github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts => ${JSON.stringify(charts)}\n`;
  await writeFile(join(directory, 'go.mod'), goMod);
  await copyFile(join(charts, 'go.sum'), join(directory, 'go.sum'));
  await writeFile(join(directory, 'main.go'), goProducer);
  producer = spawn('go', ['run', '-mod=mod', '.'], {
    cwd: directory, detached: true, stdio: ['ignore', 'pipe', 'pipe'],
    env: { ...process.env, GOWORK: 'off', CANDLES_REPAIR_POSTGRES_URL: `host=${socket} user=postgres dbname=postgres sslmode=disable` },
  });
  let diagnostics = '';
  producer.stderr.setEncoding('utf8').on('data', chunk => { diagnostics += chunk; });
  baseUrl = await new Promise((resolveUrl, reject) => {
    const lines = createInterface({ input: producer.stdout });
    const timer = setTimeout(() => reject(new Error(`Go producer startup timed out: ${diagnostics}`)), 540000);
    producer.once('error', reject);
    producer.once('close', code => {
      clearTimeout(timer);
      reject(new Error(`Go producer exited ${code}: ${diagnostics}`));
    });
    lines.on('line', line => {
      if (line.startsWith('http://127.0.0.1:')) {
        clearTimeout(timer);
        resolveUrl(line);
      }
    });
  });
  client.configure({ baseUrl });
  httpClient.configure({ baseUrl });
}, { timeout: 600000 });

function assertBars(page, first, count) {
  assert.equal(page.s, 'ok');
  assert.deepEqual(page.t, Array.from({ length: count }, (_, i) => (first + i) * 60));
  for (const [field, value] of Object.entries({ o: 1, h: 2, l: 1, c: 2, v: 1.000001 })) {
    assert.deepEqual(page[field], Array(count).fill(value));
  }
}

async function vendorPage(parameters) {
  const query = new URLSearchParams(Object.entries({ ...request, ...parameters }).map(([key, value]) => [key, String(value)]));
  const response = await fetch(`${baseUrl}/history?${query}`);
  assert.equal(response.status, 200);
  return response.json();
}

async function finding1VendorPageBounds() {
  for (const count of [2001, 5000]) {
    const rangePage = await client.getHistory({ ...request, to: (count + 1) * 60 });
    assertBars(rangePage, 1, count);
    assert.equal(rangePage.nextCursor, undefined);
    const countbackPage = await client.getHistory({ symbol, resolution: '1', to: (count + 1) * 60, countback: count });
    assertBars(countbackPage, 1, count);
    assert.equal(countbackPage.nextCursor, undefined);
  }
  for (const countback of [0, -1, 5001, 1.5, Number.MAX_SAFE_INTEGER]) {
    await assert.rejects(client.getHistory({ ...request, countback }), RangeError);
  }
  const response = await fetch(`${baseUrl}/history?${new URLSearchParams({ ...request, countback: '5001' })}`);
  assert.equal(response.status, 422);
  assert.equal((await response.json()).code, 'FIELD_INVALID');

  const full = await vendorPage({ to: 5001 * 60 });
  const extra = await vendorPage({ from: 5001 * 60, to: 5002 * 60 });
  const oversized = { ...full };
  for (const field of fields) oversized[field] = full[field].concat(extra[field]);
  assert.throws(() => parseHistory(oversized), /History exceeds 5000 bars/);
}
test(finding1VendorPageBounds);

async function finding2ForwardPagination() {
  const received = [];
  let cursor;
  for (const [first, count] of [[1, 5000], [5001, 5000], [10001, 3]]) {
    const page = await getHistory({ ...request, ...(cursor === undefined ? {} : { cursor }) });
    assertBars(page, first, count);
    received.push(...page.t);
    if (count === 5000) {
      assert.equal(typeof page.nextCursor, 'string');
      assert.ok(page.nextCursor.length <= 256);
      assert.notEqual(page.nextCursor, cursor);
    } else {
      assert.equal(page.nextCursor, undefined);
    }
    cursor = page.nextCursor;
  }
  assert.deepEqual(received, Array.from({ length: rowCount }, (_, i) => (i + 1) * 60));
  assert.equal(new Set(received).size, rowCount);
}
test(finding2ForwardPagination);

async function finding2CountbackPagination() {
  const received = [];
  let cursor;
  let remaining = rowCount;
  for (let i = 0; i < 5; i++) {
    const count = Math.min(2001, remaining);
    const page = await getHistory({ symbol, resolution: '1', to: request.to, countback: 2001, ...(cursor === undefined ? {} : { cursor }) });
    assertBars(page, remaining - count + 1, count);
    received.unshift(...page.t);
    remaining -= count;
    if (remaining > 0) assert.equal(typeof page.nextCursor, 'string');
    else assert.equal(page.nextCursor, undefined);
    cursor = page.nextCursor;
  }
  assert.equal(remaining, 0);
  assert.deepEqual(received, Array.from({ length: rowCount }, (_, i) => (i + 1) * 60));
  assert.equal(new Set(received).size, rowCount);
}
test(finding2CountbackPagination);

async function finding2CursorValidationAndBinding() {
  const raw = await vendorPage({});
  assert.equal(parseHistory(raw).nextCursor, raw.nextCursor);
  for (const invalid of ['', null, 42, {}, 'x'.repeat(257), 'not-a-token', 'a.b.c', 'a. b', 'a.b\n']) {
    assert.throws(() => parseHistory({ ...raw, nextCursor: invalid }), UdfResponseError);
    await assert.rejects(client.getHistory({ ...request, cursor: invalid }), UdfResponseError);
  }
  const [body, signature] = raw.nextCursor.split('.');
  const tampered = `${body}.${signature[0] === 'A' ? 'B' : 'A'}${signature.slice(1)}`;
  for (const input of [
    { ...request, cursor: tampered },
    { ...request, to: request.to + 60, cursor: raw.nextCursor },
    { ...request, countback: 5000, cursor: raw.nextCursor },
  ]) {
    await assert.rejects(client.getHistory(input), error => error instanceof CandlesHttpError && error.status === 400);
  }
  assert.deepEqual(await getHistory({ ...request, from: request.to, to: request.to + 60 }), { s: 'no_data' });
}
test(finding2CursorValidationAndBinding);
