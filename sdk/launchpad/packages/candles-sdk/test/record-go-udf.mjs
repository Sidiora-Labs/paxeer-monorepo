// Development-only transcript recorder. The Node test never imports this file.
import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { copyFile, mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const root = fileURLToPath(new URL('../../../../../', import.meta.url));
const charts = join(root, 'services/launchpad/trading-charts');
const run = promisify(execFile);
const producer = String.raw`package main

import (
  "context"
  "encoding/json"
  "net/http"
  "net/http/httptest"
  "net/url"
  "os"
  "sort"

  "github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/httpapi"
  "github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
  "github.com/go-chi/chi/v5"
)

const pool = "0x000000000000000000000000000000000000eeee"

type memoryHistory struct { rows []store.CandleRow }

func (s memoryHistory) QueryHistory(ctx context.Context, r httpapi.HistoryRequest, limit int) ([]store.CandleRow, error) {
  if err := ctx.Err(); err != nil { return nil, err }
  rows := make([]store.CandleRow, 0)
  for _, row := range s.rows {
    if row.PoolAddress == r.Symbol && row.Timeframe == r.Timeframe && row.CandleStart >= r.From && row.CandleStart < r.To {
      rows = append(rows, row)
    }
  }
  sort.Slice(rows, func(i, j int) bool {
    if r.Countback > 0 { return rows[i].CandleStart > rows[j].CandleStart }
    return rows[i].CandleStart < rows[j].CandleStart
  })
  if len(rows) > limit { rows = rows[:limit] }
  return rows, ctx.Err()
}

type request struct {
  Method string ` + '`json:"method"`' + `
  URL string ` + '`json:"url"`' + `
}
type response struct {
  Status int ` + '`json:"status"`' + `
  Headers http.Header ` + '`json:"headers"`' + `
  Body string ` + '`json:"body"`' + `
}
type exchange struct {
  Request request ` + '`json:"request"`' + `
  Response response ` + '`json:"response"`' + `
}

func main() {
  rows := make([]store.CandleRow, 5002)
  for i := range rows {
    rows[i] = store.CandleRow{PoolAddress: pool, Timeframe: "1m", CandleStart: int64(i+1)*60,
      Open: "1", High: "2", Low: "1", Close: "2", VolumeUsdl: "1000001", SequenceNum: int64(5002-i)}
  }
  handler := httpapi.HistoryHandler{Source: memoryHistory{rows}, SigningKey: []byte("synthetic-test-cursor-key-32-bytes-only")}
  router := func(h httpapi.HistoryHandler) http.Handler {
    r := chi.NewRouter()
    r.Route("/udf", func(r chi.Router) { httpapi.RegisterUDFWithHistory(r, h) })
    return r
  }
  exchanges := make([]exchange, 0)
  record := func(h http.Handler, path string) map[string]json.RawMessage {
    rec := httptest.NewRecorder()
    h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
    exchanges = append(exchanges, exchange{request{http.MethodGet, path}, response{rec.Code, rec.Header(), rec.Body.String()}})
    body := make(map[string]json.RawMessage)
    if path != "/udf/time" {
      if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil { panic(err) }
    }
    return body
  }
  cursor := func(body map[string]json.RawMessage) string {
    var value string
    if err := json.Unmarshal(body["nextCursor"], &value); err != nil { panic(err) }
    return value
  }
  h := router(handler)
  record(h, "/udf/config")
  record(h, "/udf/symbols?symbol="+pool)
  record(h, "/udf/time")
  history := "/udf/history?symbol="+pool+"&resolution=1&from=60&to=300180"
  first := cursor(record(h, history))
  record(h, history+"&cursor="+url.QueryEscape(first))
  countback := "/udf/history?symbol="+pool+"&resolution=1&to=300180&countback=5000"
  latest := cursor(record(h, countback))
  record(h, countback+"&cursor="+url.QueryEscape(latest))
  record(h, "/udf/history?symbol="+pool+"&resolution=1&from=300180&to=300240")
  record(h, "/udf/symbols?symbol=SID")
  record(h, "/udf/symbols?symbol="+url.QueryEscape("SID & USDL"))
  record(h, "/udf/history?symbol=SID&resolution=1&from=60&to=300180")
  parts := []byte(first)
  for i, b := range parts {
    if b == '.' {
      if parts[i+1] == 'A' { parts[i+1] = 'B' } else { parts[i+1] = 'A' }
      break
    }
  }
  record(h, history+"&cursor="+url.QueryEscape(string(parts)))
  record(h, "/udf/history?symbol="+pool+"&resolution=1&from=60&to=300240&cursor="+url.QueryEscape(first))
  unavailable := handler
  unavailable.Source = nil
  record(router(unavailable), history)
  corrupt := handler
  row := rows[0]
  row.Open = "xx"
  corrupt.Source = memoryHistory{[]store.CandleRow{row}}
  record(router(corrupt), history)
  if err := json.NewEncoder(os.Stdout).Encode(exchanges); err != nil { panic(err) }
}
`;

const directory = await mkdtemp(join(tmpdir(), 'record-go-udf-'));
try {
  let manifest = await readFile(join(charts, 'go.mod'), 'utf8');
  manifest = manifest.replace(/^module (.+)$/m, 'module $1/candles-sdk-transcript');
  manifest = manifest.replaceAll('=> ../../../sdk/launchpad/shared', `=> ${JSON.stringify(join(root, 'sdk/launchpad/shared'))}`);
  manifest = manifest.replaceAll('=> ../../../sdk/launchpad/protocol', `=> ${JSON.stringify(join(root, 'sdk/launchpad/protocol'))}`);
  manifest += '\nrequire github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts v0.0.0\n';
  manifest += `replace github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts => ${JSON.stringify(charts)}\n`;
  await writeFile(join(directory, 'go.mod'), manifest);
  await copyFile(join(charts, 'go.sum'), join(directory, 'go.sum'));
  await writeFile(join(directory, 'main.go'), producer);
  const { stdout } = await run('go', ['run', '-mod=mod', '.'], {
    cwd: directory, env: { ...process.env, GOWORK: 'off' }, timeout: 600000, maxBuffer: 4 * 1024 * 1024,
  });
  const paths = [
    'services/launchpad/trading-charts/internal/httpapi/history.go',
    'services/launchpad/trading-charts/internal/httpapi/symbols.go',
    'services/launchpad/trading-charts/internal/httpapi/udf.go',
    'sdk/launchpad/shared/http/errors.go',
  ];
  const sources = {};
  for (const path of paths) sources[path] = createHash('sha256').update(await readFile(join(root, path))).digest('hex');
  const { stdout: revision } = await run('git', ['rev-parse', 'HEAD'], { cwd: root });
  const transcript = {
    producer: { revision: revision.trim(), registration: 'RegisterUDFWithHistory', sources },
    pool: '0x000000000000000000000000000000000000eeee',
    exchanges: JSON.parse(stdout),
  };
  await mkdir(new URL('./fixtures/', import.meta.url), { recursive: true });
  await writeFile(new URL('./fixtures/go-udf-22-33-1045.json', import.meta.url), `${JSON.stringify(transcript, null, 2)}\n`);
} finally {
  await rm(directory, { recursive: true, force: true });
}
