import assert from "node:assert/strict";
import { once } from "node:events";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { joinCommand, parseCLI } from "../src/cli.mjs";
import { PeerIdentity } from "../src/identity.mjs";
import { RoomHost, vector as signalVector } from "./host-signal.mjs";

const vector = JSON.parse(
  await readFile(
    new URL("fixtures/host-enrol-route.json", import.meta.url),
    "utf8",
  ),
);

async function directory(t) {
  const path = await mkdtemp(join(tmpdir(), "inference-enrol-route-"));
  t.after(() => rm(path, { recursive: true, force: true }));
  return path;
}

async function body(request) {
  const parts = [];
  for await (const chunk of request) parts.push(chunk);
  return Buffer.concat(parts).toString();
}

async function listener(t, handler) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    server.closeAllConnections();
    await new Promise((resolve) => server.close(resolve));
  });
  return `http://127.0.0.1:${server.address().port}`;
}

async function enrolment_file_option_sends_exact_document_before_hello_and_absence_sends_no_request(
  t,
) {
  const dir = await directory(t);
  const identity = join(dir, "identity.pkcs8");
  const manifest = join(dir, "manifest.bin");
  const enrolment = join(dir, "enrolment.json");
  const document = JSON.stringify(vector.request, null, 2) + "\n";
  await writeFile(manifest, Buffer.from(vector.request.manifest_v2, "hex"));
  await writeFile(enrolment, document);
  for (const supplied of [true, false]) {
    const events = [];
    const host = new RoomHost();
    host.server.on("request", async (request, response) => {
      events.push(request.url);
      assert.equal(request.method, "POST");
      assert.equal(request.url, vector.route);
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(await body(request), document);
      response.writeHead(vector.accepted.status, {
        "content-type": "application/json",
        connection: "close",
      });
      response.end(JSON.stringify(vector.accepted.body));
    });
    const room = signalVector("hello").room_id;
    const url = await host.listen(t, (connection) => {
      events.push("hello");
      connection.send(signalVector("offer"));
    });
    const args = [
      "join",
      url.replace("ws:", "http:"),
      Buffer.from(room).toString("hex"),
      "--identity",
      identity,
      "--manifest",
      manifest,
      "--status-port",
      "0",
    ];
    if (supplied) args.push("--enrolment", enrolment);
    const options = parseCLI(args);
    assert.equal(options.enrolment, supplied ? enrolment : undefined);
    const session = await joinCommand(options);
    await session.close();
    assert.deepEqual(events, supplied ? [vector.route, "hello"] : ["hello"]);
  }
  assert.throws(
    () =>
      parseCLI([
        "join",
        "http://127.0.0.1",
        "45".repeat(32),
        "--manifest",
        manifest,
        "--quote",
        enrolment,
      ]),
    /Unknown option/,
  );
}

async function enrol_peer_forwards_the_shared_document_and_preserves_every_host_answer(
  t,
) {
  const identity = await PeerIdentity.open({
    path: join(await directory(t), "key"),
  });
  for (const answer of [vector.accepted, ...vector.refusals]) {
    let received;
    const url = await listener(t, async (request, response) => {
      assert.equal(request.method, "POST");
      assert.equal(request.url, vector.route);
      received = JSON.parse(await body(request));
      response.writeHead(answer.status, { "content-type": "application/json" });
      response.end(JSON.stringify(answer.body));
    });
    const operation = identity.enrolPeer(`${url}${vector.route}`, {
      document: vector.request,
    });
    if (answer.status === vector.accepted.status) {
      assert.deepEqual(await operation, answer.body);
    } else {
      await assert.rejects(operation, (error) => {
        assert.ok(error.message.includes(`(${answer.status})`));
        assert.ok(error.message.includes(JSON.stringify(answer.body)));
        return true;
      });
    }
    assert.deepEqual(received, vector.request);
  }
}

async function enrol_peer_refuses_invalid_input_and_malformed_redirected_or_timed_out_answers(
  t,
) {
  const identity = await PeerIdentity.open({
    path: join(await directory(t), "key"),
  });
  let requests = 0;
  const url = await listener(t, async (request, response) => {
    requests++;
    await body(request);
    const mode = request.headers["x-mode"];
    if (mode === "timeout") return;
    if (mode === "redirect")
      response.writeHead(302, { location: vector.route });
    else response.writeHead(200, { "content-type": "application/json" });
    response.end(mode === "json" ? "{" : "{}");
  });
  for (const document of [
    "{",
    "null",
    "[]",
    new Uint8Array([255]),
    "x".repeat(vector.limit + 1),
  ])
    await assert.rejects(
      identity.enrolPeer(`${url}${vector.route}`, { document }),
    );
  assert.equal(requests, 0);
  for (const mode of ["json", "outcome", "redirect", "timeout"])
    await assert.rejects(
      identity.enrolPeer(`${url}${vector.route}`, {
        document: vector.request,
        headers: { "x-mode": mode },
        timeoutMs: mode === "timeout" ? 50 : 2000,
      }),
    );
  assert.equal(requests, 4);
}

test(
  enrolment_file_option_sends_exact_document_before_hello_and_absence_sends_no_request,
);
test(enrol_peer_forwards_the_shared_document_and_preserves_every_host_answer);
test(
  enrol_peer_refuses_invalid_input_and_malformed_redirected_or_timed_out_answers,
);
