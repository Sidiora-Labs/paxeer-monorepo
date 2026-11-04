import "server-only";

import { randomUUID } from "node:crypto";
import { constants, type Stats } from "node:fs";
import {
  link,
  lstat,
  mkdir,
  open,
  readFile,
  readdir,
  realpath,
  rename,
  unlink,
} from "node:fs/promises";
import path from "node:path";
import { threadId } from "node:worker_threads";

import {
  normalizedBudget,
  percentile75,
  RUM_SERIES_CAPACITY,
  type PerformanceRoute,
  type WebVitalName,
} from "../perf/budgets";
import {
  parseRedactedWebVital,
  recordWebVital,
  rumSnapshot,
  type RedactedWebVital,
  type RumAggregate,
} from "../perf/rum-store";

const RECORD_LIMIT_BYTES = 512;
const RECORD_PATTERN =
  /^v1-(root|explorer|app|bridge|exchange|launchpad)-(lcp|inp|cls)-(\d{13})-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.json$/u;
const TEMPORARY_PATTERN = /^(\.v1-.+\.json)(?:\.([1-9][0-9]*)\.([0-9]+))?\.tmp$/u;
const activeRuntime = globalThis as typeof globalThis & { __layerxActivePerformanceWrites?: Set<string> };
const activeWrites = activeRuntime.__layerxActivePerformanceWrites ?? new Set<string>();
activeRuntime.__layerxActivePerformanceWrites = activeWrites;

type SinkMode = "durable-file" | "development-fallback" | "unconfigured";
type SinkDelivery = "unknown" | "healthy" | "degraded";
type SinkReason =
  | "awaiting-delivery"
  | "delivery-confirmed"
  | "delivery-failed"
  | "development-only"
  | "invalid-configuration"
  | "missing-configuration";

export interface PerformanceSinkHealth {
  readonly mode: SinkMode;
  readonly configured: boolean;
  readonly durable: boolean;
  readonly delivery: SinkDelivery;
  readonly reason: SinkReason;
}

export interface PerformanceSinkSnapshot {
  readonly health: PerformanceSinkHealth;
  readonly aggregates: readonly RumAggregate[];
}

export interface PerformanceSinkResult {
  readonly accepted: boolean;
  readonly durable: boolean;
  readonly health: PerformanceSinkHealth;
}

interface DurableConfiguration {
  readonly mode: "durable-file";
  readonly directory: string;
}

interface DevelopmentConfiguration {
  readonly mode: "development-fallback";
}

interface MissingConfiguration {
  readonly mode: "unconfigured";
  readonly reason: "invalid-configuration" | "missing-configuration";
}

type SinkConfiguration = DurableConfiguration | DevelopmentConfiguration | MissingConfiguration;

interface SinkRuntime {
  delivery: SinkDelivery;
}

type SinkGlobal = typeof globalThis & { __layerxPerformanceSink?: SinkRuntime };

const runtime = globalThis as SinkGlobal;
const sinkRuntime = runtime.__layerxPerformanceSink ?? { delivery: "unknown" as const };
runtime.__layerxPerformanceSink = sinkRuntime;

function storageDirectory(value: string): string | undefined {
  if (
    value.length === 0 ||
    value.includes("\0") ||
    !path.isAbsolute(value) ||
    path.resolve(value) !== value ||
    path.parse(value).root === value
  ) {
    return undefined;
  }
  return value;
}

function configuration(): SinkConfiguration {
  const configuredDirectory = process.env.LAYERX_RUM_STORAGE_DIRECTORY;
  if (configuredDirectory !== undefined) {
    const directory = storageDirectory(configuredDirectory);
    return directory === undefined
      ? { mode: "unconfigured", reason: "invalid-configuration" }
      : { mode: "durable-file", directory };
  }
  if (
    process.env.NODE_ENV === "development" &&
    process.env.LAYERX_RUM_DEVELOPMENT_FALLBACK === "1"
  ) {
    return { mode: "development-fallback" };
  }
  return { mode: "unconfigured", reason: "missing-configuration" };
}

function health(config: SinkConfiguration): PerformanceSinkHealth {
  if (config.mode === "durable-file") {
    const reason =
      sinkRuntime.delivery === "healthy"
        ? "delivery-confirmed"
        : sinkRuntime.delivery === "degraded"
          ? "delivery-failed"
          : "awaiting-delivery";
    return Object.freeze({
      mode: config.mode,
      configured: true,
      durable: true,
      delivery: sinkRuntime.delivery,
      reason,
    });
  }
  if (config.mode === "development-fallback") {
    return Object.freeze({
      mode: config.mode,
      configured: true,
      durable: false,
      delivery: "healthy",
      reason: "development-only",
    });
  }
  return Object.freeze({
    mode: config.mode,
    configured: false,
    durable: false,
    delivery: "degraded",
    reason: config.reason,
  });
}

async function ensureDirectory(directory: string): Promise<void> {
  await mkdir(directory, { recursive: true, mode: 0o700 });
  const [resolved, metadata] = await Promise.all([realpath(directory), lstat(directory)]);
  if (
    resolved !== directory ||
    !metadata.isDirectory() ||
    metadata.isSymbolicLink() ||
    (metadata.mode & 0o077) !== 0 ||
    (typeof process.getuid === "function" && metadata.uid !== process.getuid())
  ) {
    throw new Error("unsafe performance storage directory");
  }
}

async function syncDirectory(directory: string): Promise<void> {
  const handle = await open(directory, constants.O_RDONLY);
  try {
    await handle.sync();
  } finally {
    await handle.close();
  }
}

function seriesPrefix(observation: RedactedWebVital): string {
  return `v1-${observation.route}-${observation.metric.toLowerCase()}-`;
}

async function pruneSeries(directory: string, prefix: string): Promise<void> {
  const records = (await readdir(directory))
    .filter((name) => name.startsWith(prefix) && RECORD_PATTERN.test(name))
    .sort();
  const expired = records.slice(0, Math.max(0, records.length - RUM_SERIES_CAPACITY));
  if (expired.length === 0) {
    return;
  }
  for (const name of expired) {
    try {
      await unlink(path.join(directory, name));
    } catch (error) {
      if (
        typeof error !== "object" ||
        error === null ||
        !("code" in error) ||
        error.code !== "ENOENT"
      ) {
        throw error;
      }
    }
  }
  await syncDirectory(directory);
}

/** A private durable write can be closed before commit; reopening recovers its abandoned record. */
export class DurablePerformanceWrite {
  private closed = false;
  private written = false;

  private constructor(
    private readonly directory: string,
    private readonly temporary: string,
    private readonly destination: string,
    private readonly observation: RedactedWebVital,
    private readonly handle: Awaited<ReturnType<typeof open>>,
  ) {}

  static async begin(directory: string, observation: RedactedWebVital): Promise<DurablePerformanceWrite> {
    if (parseRedactedWebVital(observation) === undefined) throw new Error("invalid performance observation");
    await ensureDirectory(directory);
    const timestamp = String(Date.now()).padStart(13, "0");
    const name = `${seriesPrefix(observation)}${timestamp}-${randomUUID()}.json`;
    const temporary = path.join(directory, `.${name}.${process.pid}.${threadId}.tmp`);
    // Register before creating the file so a concurrent snapshot cannot recover an active write.
    activeWrites.add(temporary);
    try {
      const handle = await open(temporary,
        constants.O_WRONLY | constants.O_CREAT | constants.O_EXCL | constants.O_NOFOLLOW, 0o600);
      return new DurablePerformanceWrite(directory, temporary, path.join(directory, name), observation, handle);
    } catch (error) {
      activeWrites.delete(temporary);
      throw error;
    }
  }

  async write(): Promise<void> {
    if (this.closed || this.written) throw new Error("performance write is not writable");
    const bytes = Buffer.from(`${JSON.stringify(this.observation)}\n`, "utf8");
    if (bytes.length > RECORD_LIMIT_BYTES) throw new Error("performance observation exceeds its storage bound");
    await this.handle.writeFile(bytes);
    await this.handle.sync();
    this.written = true;
  }

  async commit(): Promise<void> {
    if (this.closed || !this.written) throw new Error("performance write is not prepared");
    await this.handle.close();
    // Keep the write registered until rename finishes, including its await boundary.
    try {
      await rename(this.temporary, this.destination);
      await syncDirectory(this.directory);
      await pruneSeries(this.directory, seriesPrefix(this.observation));
    } finally {
      this.closed = true;
      activeWrites.delete(this.temporary);
    }
  }

  async close(): Promise<void> {
    if (this.closed) return;
    await this.handle.close();
    this.closed = true;
    activeWrites.delete(this.temporary);
  }
}

async function persistDurably(directory: string, observation: RedactedWebVital): Promise<void> {
  const write = await DurablePerformanceWrite.begin(directory, observation);
  try {
    await write.write();
    await write.commit();
  } finally {
    await write.close();
  }
}

function fileIdentity(name: string):
  | Readonly<{ route: PerformanceRoute; metric: WebVitalName }>
  | undefined {
  const matched = RECORD_PATTERN.exec(name);
  if (matched === null) {
    return undefined;
  }
  const route = matched[1] as PerformanceRoute;
  const metric = matched[2]?.toUpperCase() as WebVitalName;
  return { route, metric };
}

async function readObservation(
  directory: string,
  name: string,
): Promise<RedactedWebVital | undefined> {
  const identity = fileIdentity(name);
  if (identity === undefined) {
    throw new Error("foreign durable performance store entry");
  }
  const location = path.join(directory, name);
  let metadata: Stats;
  try {
    metadata = await lstat(location);
  } catch (error) {
    if (
      typeof error === "object" &&
      error !== null &&
      "code" in error &&
      error.code === "ENOENT"
    ) {
      return undefined;
    }
    throw error;
  }
  if (
    !metadata.isFile() ||
    metadata.isSymbolicLink() ||
    metadata.nlink !== 1 ||
    (metadata.mode & 0o077) !== 0 ||
    (typeof process.getuid === "function" && metadata.uid !== process.getuid()) ||
    metadata.size > RECORD_LIMIT_BYTES
  ) {
    throw new Error("invalid durable performance record");
  }
  let source: string;
  try {
    source = await readFile(location, "utf8");
  } catch (error) {
    if (
      typeof error === "object" &&
      error !== null &&
      "code" in error &&
      error.code === "ENOENT"
    ) {
      return undefined;
    }
    throw error;
  }
  const observation = parseRedactedWebVital(JSON.parse(source) as unknown);
  if (
    observation === undefined ||
    observation.route !== identity.route ||
    observation.metric !== identity.metric
  ) {
    throw new Error("durable performance record failed validation");
  }
  return observation;
}

function aggregates(observations: readonly RedactedWebVital[]): readonly RumAggregate[] {
  const series = new Map<string, { route: PerformanceRoute; metric: WebVitalName; values: number[] }>();
  for (const observation of observations) {
    const key = `${observation.route}:${observation.metric}`;
    const current = series.get(key) ?? {
      route: observation.route,
      metric: observation.metric,
      values: [],
    };
    current.values.push(observation.observed);
    series.set(key, current);
  }
  return [...series.values()]
    .map(({ route, metric, values }): RumAggregate => {
      const observed = percentile75(values);
      return Object.freeze({
        route,
        metric,
        samples: values.length,
        percentile75: observed,
        withinBudget: observed <= normalizedBudget(metric),
      });
    })
    .sort((left, right) =>
      `${left.route}:${left.metric}`.localeCompare(`${right.route}:${right.metric}`),
    );
}

function missingFile(error: unknown): boolean {
  return typeof error === "object" && error !== null && "code" in error && error.code === "ENOENT";
}

function abandonedOwner(pid: number, ownerThread: number, location: string): boolean {
  if (pid === process.pid) return ownerThread === threadId && !activeWrites.has(location);
  try {
    process.kill(pid, 0);
    return false;
  } catch (error) {
    return typeof error === "object" && error !== null && "code" in error && error.code === "ESRCH";
  }
}

async function recoverTemporary(directory: string, name: string, matched: RegExpExecArray): Promise<void> {
  const finalName = matched[1]?.slice(1);
  const identity = finalName === undefined ? undefined : fileIdentity(finalName);
  if (identity === undefined || finalName === undefined) throw new Error("foreign performance temporary");
  const location = path.join(directory, name);
  let handle: Awaited<ReturnType<typeof open>>;
  try {
    handle = await open(location, constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK);
  } catch (error) {
    if (missingFile(error)) return;
    throw error;
  }
  try {
    const metadata = await handle.stat();
    if (!metadata.isFile() || (metadata.nlink !== 1 && metadata.nlink !== 2) || (metadata.mode & 0o077) !== 0 ||
        (typeof process.getuid === "function" && metadata.uid !== process.getuid()) || metadata.size > RECORD_LIMIT_BYTES) {
      throw new Error("unsafe performance temporary");
    }
    if (metadata.nlink === 2) {
      // Recovery itself may have stopped between exclusive publication and unlink.
      const destination = await lstat(path.join(directory, finalName));
      if (destination.ino !== metadata.ino || destination.dev !== metadata.dev) {
        throw new Error("foreign performance temporary hard link");
      }
    }
    // Legacy files have no owner lease: leave them untouched, without hiding committed records.
    const pid = Number(matched[2]);
    const ownerThread = Number(matched[3]);
    if (matched[2] === undefined || !Number.isSafeInteger(pid) || pid > 2_147_483_647 ||
        !Number.isSafeInteger(ownerThread) || !abandonedOwner(pid, ownerThread, location)) return;
    let observation: RedactedWebVital | undefined;
    try {
      observation = parseRedactedWebVital(JSON.parse(await handle.readFile("utf8")) as unknown);
    } catch (error) {
      if (!(error instanceof SyntaxError)) throw error;
    }
    if (observation !== undefined && (observation.route !== identity.route || observation.metric !== identity.metric)) {
      throw new Error("performance temporary identity mismatch");
    }
    const current = await lstat(location);
    if (current.ino !== metadata.ino || current.dev !== metadata.dev) throw new Error("performance temporary changed");
    if (observation !== undefined) {
      // Exclusive link publishes complete records without replacing any committed observation.
      try {
        await link(location, path.join(directory, finalName));
      } catch (error) {
        if (typeof error !== "object" || error === null || !("code" in error) || error.code !== "EEXIST") throw error;
        const destination = await lstat(path.join(directory, finalName));
        if (destination.ino !== metadata.ino || destination.dev !== metadata.dev) {
          const committed = await readObservation(directory, finalName);
          if (committed === undefined || JSON.stringify(committed) !== JSON.stringify(observation)) throw error;
        }
      }
    }
    await unlink(location);
    await syncDirectory(directory);
    if (observation !== undefined) await pruneSeries(directory, seriesPrefix(observation));
  } catch (error) {
    if (!missingFile(error)) throw error;
  } finally {
    await handle.close();
  }
}

async function durableSnapshot(directory: string): Promise<readonly RumAggregate[]> {
  await ensureDirectory(directory);
  for (const name of (await readdir(directory)).sort()) {
    const temporary = TEMPORARY_PATTERN.exec(name);
    if (temporary !== null) await recoverTemporary(directory, name, temporary);
  }
  const observations: RedactedWebVital[] = [];
  for (const name of (await readdir(directory)).sort()) {
    if (TEMPORARY_PATTERN.test(name)) continue;
    const observation = await readObservation(directory, name);
    if (observation !== undefined) observations.push(observation);
  }
  return aggregates(observations);
}

export async function persistPerformanceObservation(
  observation: RedactedWebVital,
): Promise<PerformanceSinkResult> {
  const config = configuration();
  if (config.mode === "durable-file") {
    try {
      await persistDurably(config.directory, observation);
      sinkRuntime.delivery = "healthy";
      return Object.freeze({ accepted: true, durable: true, health: health(config) });
    } catch {
      sinkRuntime.delivery = "degraded";
      return Object.freeze({ accepted: false, durable: false, health: health(config) });
    }
  }
  if (config.mode === "development-fallback") {
    recordWebVital(observation);
    return Object.freeze({ accepted: true, durable: false, health: health(config) });
  }
  return Object.freeze({ accepted: false, durable: false, health: health(config) });
}

export async function performanceSinkSnapshot(): Promise<PerformanceSinkSnapshot> {
  const config = configuration();
  if (config.mode === "durable-file") {
    try {
      const snapshot = await durableSnapshot(config.directory);
      if (snapshot.length > 0) sinkRuntime.delivery = "healthy";
      return Object.freeze({ health: health(config), aggregates: snapshot });
    } catch {
      sinkRuntime.delivery = "degraded";
      return Object.freeze({ health: health(config), aggregates: [] });
    }
  }
  return Object.freeze({
    health: health(config),
    aggregates: config.mode === "development-fallback" ? rumSnapshot() : [],
  });
}
