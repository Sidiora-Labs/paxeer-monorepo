import assert from "node:assert/strict";
import { createPublicKey, verify } from "node:crypto";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir, totalmem } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { setTimeout as delay } from "node:timers/promises";
import { JSDOM } from "jsdom";
import { helloSigningBytes, PeerIdentity } from "../src/identity.mjs";
import { PeerPage, renderStatus } from "../src/page.mjs";
import { decodeSignalFrame, signalHex } from "../src/signal.mjs";
import {
  fixtures,
  hostHelloBytes,
  RoomHost,
  roomId,
  vector,
} from "./host-signal.mjs";
import { createPeerServer } from "../scripts/serve.mjs";

const html = await readFile(new URL("../index.html", import.meta.url), "utf8");
// Exact manifest_v1() bytes from the program manifest_v2_capabilities fixture,
// also consumed by identity.test.mjs.
const manifestV1 = Buffer.from(
  [
    "00010a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
    "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0000",
    "00000000000200000000000000030000000000000005000000000000012c00000000000001ac0d0d0d0d0d0d0d0d0d0d0d0d",
    "0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d000101011414141414141414141414141414141414141414141414141414",
    "1414141414141515151515151515151515151515151515151515151515151515151515151515161616161616161616161616",
    "1616161616161616161616161616161616161616171717171717171717171717171717171717171717171717171717171717",
    "1717181818181818181818181818181818181818181818181818181818181818181800000400000008000000100000002000",
    "010000ea600004010001010000001f68747470733a2f2f776f726b65722e6578616d706c652f70617861692f76311e1e1e1e",
    "1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e01000100011f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f",
    "1f1f1f1f1f1f1f1f1f1f1f1f1f1f1f2020202020202020202020202020202020202020202020202020202020202020000000",
    "00",
  ].join(""),
  "hex",
).toString("hex");

async function eventually(predicate) {
  const deadline = Date.now() + 5000;
  while (!predicate()) {
    if (Date.now() > deadline) assert.fail("Timed out waiting for page state");
    await delay(5);
  }
}

async function environment(t) {
  const directory = await mkdtemp(join(tmpdir(), "peer-page-"));
  const path = join(directory, "identity.pkcs8");
  await writeFile(path, Buffer.from(fixtures.key.pkcs8_hex, "hex"), {
    mode: 0o600,
    flag: "wx",
  });
  const server = createPeerServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const origin = `http://127.0.0.1:${server.address().port}`;
  const host = new RoomHost();
  const received = [];
  const connections = [];
  const hostUrl = await host.listen(t, (connection, frame) => {
    connections.push(connection);
    received.push(frame);
    connection.send(vector("offer"));
  });
  const pages = [];
  t.after(async () => {
    for (const { page, dom } of pages) {
      page.dispose();
      dom.window.close();
    }
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
    await rm(directory, { recursive: true, force: true });
  });
  return {
    path,
    origin,
    received,
    connections,
    host,
    async page() {
      const dom = new JSDOM(html, { url: `${origin}/` });
      // jsdom has no WebGPU or IndexedDB. Exercise the real native identity
      // store and measured memory/probe path, without substituting those APIs.
      const page = new PeerPage(dom.window.document, {
        identityOptions: { mode: "native", path },
        mode: "native",
      });
      pages.push({ page, dom });
      await page.start();
      const document = dom.window.document;
      document.getElementById("manifest-v1").value = manifestV1;
      document.getElementById("region").value = "US";
      document.getElementById("host").value = hostUrl;
      document.getElementById("room").value = roomId;
      document.getElementById("measure").click();
      await eventually(
        () =>
          !document.getElementById("join").disabled ||
          document.getElementById("error").textContent,
      );
      assert.equal(document.getElementById("error").textContent, "");
      return { page, dom, document };
    },
  };
}

function submit(document, window) {
  document
    .getElementById("join-form")
    .dispatchEvent(
      new window.Event("submit", { bubbles: true, cancelable: true }),
    );
}

async function join_flow_sends_hello_with_the_stored_key_after_page_restart(t) {
  const env = await environment(t);
  const identity = await PeerIdentity.open({ mode: "native", path: env.path });
  const saved = await readFile(env.path);
  const first = await env.page();
  assert.equal(
    first.document.getElementById("peer-key").textContent,
    identity.peerKey,
  );
  assert.match(
    first.document.getElementById("adapter").textContent,
    /WebGPU unavailable/,
  );
  const measured = JSON.parse(
    first.document.getElementById("manifest").textContent,
  );
  assert.equal(measured.version, 2);
  assert.equal(measured.systemMemoryMiB, Math.floor(totalmem() / 1048576));
  assert.ok(measured.bandwidthClass >= 1 && measured.bandwidthClass <= 4);
  submit(first.document, first.dom.window);
  await eventually(() => env.received.some((frame) => frame.type === "hello"));
  const hello = env.received.find((frame) => frame.type === "hello");
  assert.equal(signalHex(hello.room_id), roomId);
  assert.equal(Object.hasOwn(hello, "sequence"), false);
  assert.equal(signalHex(hello.peer_key), identity.peerKey);
  assert.deepEqual(hello.nonce, vector("challenge").nonce);
  assert.equal(Buffer.from(hello.manifest).toString("hex"), measured.bytes);
  const key = createPublicKey({
    key: Buffer.concat([
      Buffer.from("302a300506032b6570032100", "hex"),
      Buffer.from(identity.peerKey, "hex"),
    ]),
    format: "der",
    type: "spki",
  });
  assert.deepEqual(
    Buffer.from(helloSigningBytes(roomId, hello.nonce)),
    hostHelloBytes(hello.room_id, hello.nonce),
  );
  assert.ok(
    verify(
      null,
      hostHelloBytes(hello.room_id, hello.nonce),
      key,
      Buffer.from(hello.signature),
    ),
  );
  const closed = once(env.connections[0].socket, "close");
  first.page.dispose();
  await closed;
  const second = await env.page();
  submit(second.document, second.dom.window);
  await eventually(
    () => env.received.filter((frame) => frame.type === "hello").length === 2,
  );
  const hellos = env.received.filter((frame) => frame.type === "hello");
  assert.equal(signalHex(hellos[1].peer_key), identity.peerKey);
  // The fixture host deliberately reuses its fixed challenge on reconnect.
  assert.deepEqual(hellos[1].nonce, vector("challenge").nonce);
  assert.ok(
    verify(
      null,
      hostHelloBytes(hellos[1].room_id, hellos[1].nonce),
      key,
      Buffer.from(hellos[1].signature),
    ),
  );
  assert.deepEqual(await readFile(env.path), saved);
}

async function status_panel_renders_host_offer_deal_and_unavailable_earnings(
  t,
) {
  const env = await environment(t);
  const { document, dom } = await env.page();
  submit(document, dom.window);
  await eventually(
    () =>
      document.getElementById("connection").textContent ===
      "Joined room — awaiting deal",
  );
  assert.deepEqual(
    JSON.parse(document.getElementById("offer").textContent),
    vector("offer").offer,
  );
  assert.equal(document.getElementById("layers").textContent, "[0, 4)");
  env.connections[0].send(vector("deal"));
  await eventually(
    () => document.getElementById("deal").textContent === "07".repeat(32),
  );
  assert.equal(document.getElementById("layers").textContent, "[0, 4)");
  assert.equal(
    document.getElementById("connection").textContent,
    "Deal received — awaiting acceptance",
  );
  assert.deepEqual(
    JSON.parse(document.getElementById("deal-details").textContent),
    vector("deal").deal,
  );
  assert.equal(
    JSON.parse(document.getElementById("deal-details").textContent).price,
    "100000000000000000000",
  );
  for (const id of ["laps", "earnings", "receipts"])
    assert.equal(
      document.getElementById(id).textContent,
      "Unavailable — host does not send this state",
    );
  assert.notEqual(
    document.getElementById("earnings").textContent,
    vector("deal").deal.price,
  );
  assert.equal(document.getElementById("error").textContent, "");
  // Rendering a received deal does not send an unvalidated acknowledgement.
  assert.deepEqual(
    env.received.map((frame) => frame.type),
    ["hello"],
  );
}

async function status_renderer_displays_deal_live_and_host_error_vectors(t) {
  const env = await environment(t);
  const { document } = await env.page();
  for (const name of ["offer", "deal", "deal_live", "error"]) {
    const fixture = fixtures.vectors.find((entry) => entry.name === name);
    renderStatus(
      document,
      decodeSignalFrame(fixture.json),
      fixtures.key.peer_key_hex,
    );
  }
  assert.equal(document.getElementById("deal").textContent, "07".repeat(32));
  assert.equal(
    document.getElementById("connection").textContent,
    "Host reports deal live",
  );
  assert.equal(document.getElementById("error").textContent, "degraded");
  assert.equal(
    document.getElementById("earnings").textContent,
    "Unavailable — host does not send this state",
  );
}

async function host_error_code_is_rendered_and_socket_is_closed(t) {
  const env = await environment(t);
  const { document, dom, page } = await env.page();
  submit(document, dom.window);
  await eventually(
    () =>
      document.getElementById("connection").textContent ===
      "Joined room — awaiting deal",
  );
  const closed = once(env.connections[0].socket, "close");
  env.connections[0].send(vector("error"));
  await closed;
  assert.equal(document.getElementById("error").textContent, "degraded");
  assert.equal(document.getElementById("leave").disabled, true);
  page.showError(new Error("<img src=x onerror=alert(1)>"));
  assert.equal(document.getElementById("error").children.length, 0);
  assert.equal(
    document.getElementById("error").textContent,
    "<img src=x onerror=alert(1)>",
  );
}

async function unload_closes_the_real_socket_once_without_inventing_bye(t) {
  const env = await environment(t);
  const { document, dom } = await env.page();
  submit(document, dom.window);
  await eventually(
    () =>
      document.getElementById("connection").textContent ===
      "Joined room — awaiting deal",
  );
  const socket = env.connections[0].socket;
  let closes = 0;
  socket.on("close", () => closes++);
  const closed = once(socket, "close");
  for (const event of ["beforeunload", "pagehide", "unload"])
    dom.window.dispatchEvent(new dom.window.Event(event));
  await closed;
  assert.equal(closes, 1);
  assert.deepEqual(
    env.received.map((frame) => frame.type),
    ["hello"],
  );
  assert.equal(document.getElementById("connection").textContent, "Left room");
  assert.equal(document.getElementById("leave").disabled, true);
  assert.equal(document.getElementById("join").disabled, false);
}

async function failed_measurement_cannot_reuse_a_stale_manifest(t) {
  const env = await environment(t);
  const { document, dom, page } = await env.page();
  document.getElementById("manifest-v1").value = "bad";
  document
    .getElementById("manifest-v1")
    .dispatchEvent(new dom.window.Event("input"));
  assert.equal(document.getElementById("join").disabled, true);
  await assert.rejects(page.measure(), /hexadecimal bytes/);
  assert.equal(
    document.getElementById("manifest").textContent,
    "Measurement failed",
  );
  await assert.rejects(page.join(), /Measure this device/);
  assert.equal(env.received.length, 0);
}

async function static_server_serves_modules_probe_and_rejects_private_paths(t) {
  const { origin } = await environment(t);
  const index = await fetch(`${origin}/`);
  assert.equal(index.status, 200);
  assert.match(index.headers.get("content-type"), /text\/html/);
  assert.equal(await index.text(), html);
  const module = await fetch(`${origin}/src/page.mjs`);
  assert.equal(module.status, 200);
  assert.match(module.headers.get("content-type"), /text\/javascript/);
  assert.equal(module.headers.get("cross-origin-opener-policy"), "same-origin");
  assert.equal(
    module.headers.get("cross-origin-embedder-policy"),
    "require-corp",
  );
  const probe = await fetch(`${origin}/probe`);
  assert.equal(probe.headers.get("content-length"), "65536");
  assert.equal((await probe.arrayBuffer()).byteLength, 65536);
  assert.equal(
    (await fetch(`${origin}/src/page.mjs`, { method: "HEAD" })).status,
    200,
  );
  assert.equal(
    (await fetch(`${origin}/probe`, { method: "HEAD" })).headers.get(
      "content-length",
    ),
    "65536",
  );
  for (const path of [
    "/package.json",
    "/test/page.test.mjs",
    "/src/%2e%2e%2fpackage.json",
    "/src/%2e%2e%2f%2e%2e%2fpackage.json",
    "/src/missing.mjs",
  ])
    assert.ok([403, 404].includes((await fetch(`${origin}${path}`)).status));
  assert.equal((await fetch(`${origin}/src/%GG`)).status, 400);
  assert.equal((await fetch(`${origin}/`, { method: "POST" })).status, 405);
}

test(join_flow_sends_hello_with_the_stored_key_after_page_restart);
test(status_panel_renders_host_offer_deal_and_unavailable_earnings);
test(status_renderer_displays_deal_live_and_host_error_vectors);
test(host_error_code_is_rendered_and_socket_is_closed);
test(failed_measurement_cannot_reuse_a_stale_manifest);
test(unload_closes_the_real_socket_once_without_inventing_bye);
test(static_server_serves_modules_probe_and_rejects_private_paths);
