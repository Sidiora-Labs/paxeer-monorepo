// Vendored from https://github.com/Nehanth/pooled/blob/45b5ee275a8754a37fce4a6ce174f31e4a216ef4/room/liveness.js (MIT; see LICENSE-ENGINE).
// Who is still in the room: the ping loop's silent-link drop and the host's duplicate-name probe.
// DOM-free so it can be unit tested (tests/unit/liveness_test.js).
//
// Every device answers a ping at once, so a link that has said nothing for a while is a device that
// vanished without a clean close (a crashed or killed tab, a laptop asleep, Wi-Fi gone): no pagehide,
// and the data channel's own close can take minutes to surface (#124). The ping loop drops such a
// link, the same path as a close. How long "a while" is depends on who is on the other end, because
// the other side cannot tell a stalled tab from a gone one:
//   - a computer: SILENT_MS. Its tab answers pings from the message loop even when hidden.
//   - a phone: PHONE_SILENT_MS. Its screen locks and the browser freezes the page; a phone that
//     comes back within a minute keeps its link and its layers.
//   - the host, as a guest sees it: HOST_SILENT_MS. The host's main thread does the heaviest work
//     (its shard load, the first and last layers), and a guest that drops a live host tears the
//     room down for itself.
//   - a device mid-load (its shard still loading, on either end): never. A worker's main thread
//     can block for seconds on a shard, and dropping it mid-load throws away the start. A device
//     that really died there is still noticed when its data channel closes.
// Any sign of life counts: a message, a pong, a hidden-state frame or a keep-alive byte on the wire
// (room/transport.js stamps link.rxAt). A late tick means this tab was the one stalled (throttled,
// or a long task): its queue of pongs is not read yet, so it judges nobody on that tick.
export const PING_MS = 2500;
export const SILENT_MS = 15000;
export const PHONE_SILENT_MS = 60000;
export const HOST_SILENT_MS = 30000;

// the last time anything came in from a link (a message, a wire frame, a keep-alive byte)
export function lastHeard(e) {
  return Math.max(e?.seen ?? -Infinity, e?.link?.rxAt ?? -Infinity);
}

// how long a link may stay silent before it counts as gone; Infinity: never dropped for silence
export function silentLimit({ toHost = false, phone = false, loading = false } = {}) {
  if (loading) return Infinity;
  if (toHost) return HOST_SILENT_MS;
  return phone ? PHONE_SILENT_MS : SILENT_MS;
}

// one tick of the ping loop for one link: drop it?
//   heard: lastHeard(e); late: this tick fired more than 2 pings late (this tab stalled)
export function isSilentGone({ now, heard, late = false, toHost = false, phone = false, loading = false }) {
  if (late) return false;
  return now - heard > silentLimit({ toHost, phone, loading });
}

// the host: is this device still loading its layers for a start? (a device in the chain that has
// not reported ready while the start runs). Such a device is never dropped for silence.
export function midLoad({ starting, inChain, ready }) {
  return !!(starting && inChain && !ready);
}

// --- duplicate names ---
// Names are the room's keys (colours, layers, load progress, the plan): the host makes a taken one
// unique ("laptop 2"), and the device takes the name the roster gives it (renameTo).

// a name no other device in the room uses (another device's, or the host's own), with a number added
// if needed. roster: [[id, { name }]] of the other devices; id: the device asking.
export function uniqueName(name, id, hostName, roster) {
  const taken = new Set([hostName, ...[...roster].filter(([rid]) => rid !== id).map(([, m]) => m.name)]);
  if (!taken.has(name)) return name;
  const base = name.slice(0, 36);
  let n = 2;
  while (taken.has(`${base} ${n}`)) n++;
  return `${base} ${n}`;
}

// A device that asks for a name held by a device that has gone quiet is most likely that device back
// from a crash or a killed tab (its old link never closed, so the ping loop has not dropped it yet).
// The hello waits NAME_PROBE_MS while the quiet one is pinged; if it did not answer, it is dropped, so
// the newcomer keeps its name and gets its layers back. One that answers keeps its name and the
// newcomer gets a number (uniqueName).
export const NAME_PROBE_MS = 1500;
export const QUIET_MS = 1000;

// the id of a device holding `name` that has been quiet for QUIET_MS (worth a probe), or null.
// heardOf(id): lastHeard of that device's link (-Infinity when it has none).
export function quietNamesake(name, id, roster, heardOf, now) {
  for (const [rid, m] of roster) {
    if (rid === id || m.name !== name) continue;
    if (!(now - heardOf(rid) < QUIET_MS)) return rid;
  }
  return null;
}

// after the probe: the ids holding `name` that said nothing since the probe went out at `since`
export function staleNamesakes(name, id, roster, heardOf, since) {
  const out = [];
  for (const [rid, m] of roster) {
    if (rid === id || m.name !== name) continue;
    if (heardOf(rid) >= since) continue;   // it answered: alive
    out.push(rid);
  }
  return out;
}

// a device: the name the host's roster lists this device under, when it differs from its own
// (the one it asked for was taken); null otherwise. Only the host's roster counts.
export function renameTo(members, selfId, myName, fromHost) {
  if (!fromHost || !Array.isArray(members)) return null;
  const mine = members.find((m) => m.id === selfId);
  return mine && mine.name && mine.name !== myName ? mine.name : null;
}

// --- drop detection mid-answer (the host's heartbeat) ---
// Noticing a device in the chain that went silent mid-answer (its network died,
// its tab froze or was killed) in seconds instead of waiting for ICE to give up (~30 s) or for
// the lap timeouts (30-90 s).
//
// The host pings every device in the chain every HB_BUSY_MS while an answer runs (the plain
// `ping`/`pong` every protocol version answers, so this needs no protocol change) and counts
// anything it receives from a device as a sign of life: a pong, any control message, any slice
// on any of its wire channels. A device silent for longer than deadAfter(rtt) is held: every lap
// in flight waits for it and so does the next question. If it answers again it simply carries on
// (a freeze); one that stays silent is dropped by the ping loop (isSilentGone above), which takes
// the departure path (`ai-degraded`, re-deal).
//
// Why these numbers: the control channel is reliable and ordered, so one lost packet holds
// everything behind it until SCTP retransmits it (an RTO, ~1 s at first, doubling on each loss of
// the same packet). The silence limit has to cover a double loss at the worst latency we support
// (300 ms one way, 5% loss) without a false alarm, and stay within ~3-5 s. Measured with
// tests/e2e/room_drop.mjs on 3 devices, Qwen3 1.7B: the longest silence was 0.65 s on a LAN,
// 2.5 s at RTT 300 ms + 5% loss and 3.65 s at RTT 600 ms + 5% loss, against limits of 3.5, 3.9
// and 4.8 s (docs/protocol.md, "Drop detection").
//
// Pure: no DOM, no timers; the caller passes the clock.

export const HB_BUSY_MS = 500;     // ping period to chain devices while answering
export const DEAD_MIN_MS = 3500;   // never call a device dead sooner than this
export const DEAD_MAX_MS = 5000;   // ... nor later
export const STALL_MS = 1200;      // a tick this late means this tab stalled: its inbox is stale, judge nothing
// A device that went silent is held, not dropped: the answer waits for it (frames on a frozen link
// are late, not lost) and it keeps its place and layers in case it was only frozen (a Wi-Fi stall,
// a laptop lid, a busy phone). Dropping one that stays silent is the ping loop's job (isSilentGone).

// How long a device may stay silent before it counts as dead, given its measured round trip.
export function deadAfter(rttMs) {
  const rtt = Number.isFinite(rttMs) && rttMs > 0 ? rttMs : 0;
  return Math.round(Math.min(DEAD_MAX_MS, Math.max(DEAD_MIN_MS, 3000 + 3 * rtt)));
}

// Tracks when each device was last heard from and who went silent while armed (an answer runs).
export function makeLiveness() {
  return { heard: new Map(), since: 0, armed: false, lastTick: 0, lastPing: 0, maxSilence: new Map() };
}
export function heard(L, id, now) { L.heard.set(id, now); }
// Start judging (an answer began): silence is counted from now at the earliest, so a device that
// was idle between two pings is not held to the busy-time limit for time before the answer.
export function arm(L, now) { if (!L.armed) { L.armed = true; L.since = now; L.lastTick = now; L.maxSilence.clear(); } }
export function disarm(L) { L.armed = false; }
export function forget(L, id) { L.heard.delete(id); L.maxSilence.delete(id); }

// One check. ids: the chain; rttOf(id): its last measured round trip (ms) or null.
// Returns { ping: bool (time to send the next heartbeat), dead: [{ id, silentMs, limitMs }] }.
export function tick(L, now, ids, rttOf = () => null) {
  const out = { ping: false, dead: [] };
  if (!L.armed) { L.lastTick = now; return out; }
  // this tab did not run for a while (a long GPU submit, a background-tab throttle): messages that
  // arrived meanwhile are still queued behind this timer, so silence measured now is not theirs
  if (now - L.lastTick > STALL_MS) L.since = now;
  L.lastTick = now;
  if (now - L.lastPing >= HB_BUSY_MS) { out.ping = true; L.lastPing = now; }
  for (const id of ids) {
    const silent = now - Math.max(L.heard.get(id) ?? 0, L.since);
    if (silent > (L.maxSilence.get(id) ?? 0)) L.maxSilence.set(id, silent);
    const limit = deadAfter(rttOf(id));
    if (silent > limit) out.dead.push({ id, silentMs: Math.round(silent), limitMs: limit });
  }
  return out;
}

// A held (suspect) device is back once it answered a ping sent after it went silent: a full round
// trip, not a late packet draining from before. pongFor: the send time of the latest ping it
// answered; silentSince: when the silence began.
export function suspectBack(pongFor, silentSince) { return pongFor > silentSince; }

// Lap timeout for a decode lap (one token, or a speculative verify) from the laps measured so
// far: generous against the slowest recent lap, but seconds rather than the 30-90 s fallbacks
// used before any lap is measured (the first laps compile pipelines and warm caches). A dead
// device is caught by the silence check above; this is for a frame lost on a live chain, so it
// stays well clear of the stalls a frozen or lossy link causes (15 s between two tokens after a
// 12 s freeze, tests/e2e/room_chaos.mjs; 14 s at RTT 600 ms and
// 5% loss, with laps averaging 3 s).
// stat: { n, lap (EMA ms), max (decaying max ms) }; fallback: the old fixed timeout.
export const LAP_MIN_MS = 25000;
export function lapTimeout(stat, fallback, rttMs = 0) {
  if (!stat || stat.n < 4 || !(stat.max > 0)) return fallback;
  const rtt = Number.isFinite(rttMs) && rttMs > 0 ? rttMs : 0;
  return Math.round(Math.min(fallback, Math.max(LAP_MIN_MS, 6 * Math.max(stat.max, stat.lap) + 2 * rtt + 2000)));
}
