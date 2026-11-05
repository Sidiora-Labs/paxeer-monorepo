import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import { createServer as createSecureServer } from "node:https";
import { registerHooks } from "node:module";
import test from "node:test";
import { getCACertificates, setDefaultCACertificates } from "node:tls";
import { getPathMatch } from "next/dist/shared/lib/router/utils/path-match.js";
import { prepareDestination } from "next/dist/shared/lib/router/utils/prepare-destination.js";

// Resolve the actual server-only marker using Next's server condition.
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "server-only") {
      return nextResolve("next/dist/compiled/server-only/empty.js", context);
    }
    return nextResolve(specifier, context);
  },
});

const { humanApi } = await import("../src/api/index.ts");
const { humanApiForServer } = await import("../src/api/server.ts");
const { humanServiceOrigin } = await import("../src/api/human-origin.ts");
const route = await import("../src/app/api/human/[...path]/route.ts");

const goldenRoot = new URL("../../../services/human/schema/human-api/golden/", import.meta.url);
const golden = (name) => JSON.parse(readFileSync(new URL(name + ".json", goldenRoot), "utf8"));
const version = golden("version.response");
const profileRequest = golden("profile.update.request");
const profile = golden("profile.update.response");
const profileFailure = golden("profile.update.failure");
const cookie = "__Host-layerx-session=opaque-session; __Host-layerx_csrf=csrf-token";
const webOrigin = "https://app.example.com";
const csrfToken = "csrf-token";
const trace = "trc_01j2h6h8jq1z2a3b4c5d6e7f8g";

function restoreEnvironment(context) {
  for (const name of ["LAYERX_HUMAN_SERVICE_URL", "LAYERX_HUMAN_WEB_ORIGIN"]) {
    const previous = process.env[name];
    context.after(() => {
      if (previous === undefined) delete process.env[name];
      else process.env[name] = previous;
    });
  }
}

async function listen(server, context, scheme) {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  context.after(async () => {
    const closed = new Promise((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
    });
    server.closeAllConnections();
    await closed;
  });
  return `${scheme}://127.0.0.1:${server.address().port}`;
}

async function humanPeer(context) {
  const cert = readFileSync(new URL("../e2e/fixtures/localhost-cert.pem", import.meta.url), "utf8");
  const key = readFileSync(new URL("../e2e/fixtures/localhost-key.pem", import.meta.url), "utf8");
  const state = { requests: [], rejectProfile: false, redirect: undefined };
  const server = createSecureServer({ cert, key }, async (request, response) => {
    let body = "";
    for await (const chunk of request) body += chunk;
    state.requests.push({ method: request.method, url: request.url, headers: request.headers, body });
    const path = new URL(request.url, "https://localhost").pathname;
    if (path === "/v1/redirect") {
      response.writeHead(307, { Location: state.redirect });
      response.end();
      return;
    }
    if (path === "/readyz") {
      response.writeHead(200, { "Content-Type": "application/json" });
      response.end(JSON.stringify({ ready: true }));
      return;
    }
    const fixture = path === profileRequest.path
      ? state.rejectProfile ? profileFailure : profile
      : path === "/v1/version" ? version : undefined;
    if (fixture === undefined) {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(fixture.status, {
      "Content-Type": "application/json",
      "Cache-Control": "public, max-age=600",
      "Set-Cookie": [
        "__Host-layerx-session=renewed; Path=/; Secure; HttpOnly; SameSite=Strict",
        "__Host-layerx_csrf=renewed-csrf; Path=/; Secure; SameSite=Strict",
      ],
    });
    response.end(JSON.stringify(fixture.body));
  });
  return { state, origin: await listen(server, context, "https") };
}

async function webBoundary(context, rewrites) {
  const failures = [];
  const server = createServer((incoming, outgoing) => {
    void (async () => {
      const original = new URL(incoming.url, "http://localhost");
      let rewritten = original;
      for (const rewrite of rewrites) {
        const params = getPathMatch(rewrite.source)(original.pathname);
        if (params === false) continue;
        const destination = prepareDestination({
          destination: rewrite.destination,
          params,
          query: Object.fromEntries(original.searchParams),
          appendParamsToQuery: false,
        });
        rewritten = new URL(destination.newUrl, original);
        // Keep the original query bytes, including repeated parameters.
        rewritten.search = original.search;
        break;
      }
      const params = getPathMatch("/api/human/:path*")(rewritten.pathname);
      if (params === false) {
        outgoing.writeHead(404).end();
        return;
      }
      const chunks = [];
      for await (const chunk of incoming) chunks.push(chunk);
      const body = Buffer.concat(chunks);
      const headers = new Headers();
      for (const [name, value] of Object.entries(incoming.headers)) {
        if (Array.isArray(value)) value.forEach((entry) => headers.append(name, entry));
        else if (value !== undefined) headers.set(name, value);
      }
      const request = new Request(rewritten, {
        method: incoming.method,
        headers,
        ...(body.length > 0 ? { body } : {}),
      });
      const response = await route[incoming.method](request, {
        params: Promise.resolve({ path: params.path }),
      });
      outgoing.writeHead(response.status, {
        ...Object.fromEntries(response.headers),
        "set-cookie": response.headers.getSetCookie(),
      });
      outgoing.end(Buffer.from(await response.arrayBuffer()));
    })().catch((error) => {
      failures.push(error);
      outgoing.writeHead(500).end();
    });
  });
  const origin = await listen(server, context, "http");
  return { origin, failures };
}

async function human_build_runtime_origin(context) {
  restoreEnvironment(context);
  const certificates = getCACertificates("default");
  context.after(() => setDefaultCACertificates(certificates));
  setDefaultCACertificates([
    ...certificates,
    readFileSync(new URL("../e2e/fixtures/localhost-cert.pem", import.meta.url), "utf8"),
  ]);
  const built = await humanPeer(context);
  const deployed = await humanPeer(context);
  process.env.LAYERX_HUMAN_SERVICE_URL = built.origin + "/human";
  process.env.LAYERX_HUMAN_WEB_ORIGIN = webOrigin;
  const { default: config } = await import("../next.config.mjs");
  const rewrites = await config.rewrites();
  assert.deepEqual(rewrites, [
    { source: "/human/v1/:path*", destination: "/api/human/v1/:path*" },
    { source: "/v1/:path*", destination: "/api/human/v1/:path*" },
    { source: "/readyz", destination: "/api/human/readyz" },
  ]);
  const web = await webBoundary(context, rewrites);
  process.env.LAYERX_HUMAN_SERVICE_URL = deployed.origin + "/human/";
  assert.deepEqual(await config.rewrites(), rewrites);

  const serverClient = humanApiForServer({ cookie, csrfToken, trace });
  const browserClient = humanApi({
    baseUrl: web.origin + "/human",
    headers: { Cookie: cookie, Origin: webOrigin },
    csrfToken: () => csrfToken,
    trace: () => trace,
  });
  assert.deepEqual(await serverClient.version(), version.body.result);
  assert.deepEqual(await browserClient.version(), version.body.result);
  assert.deepEqual(await serverClient.profileUpdate(profileRequest.body), profile.body.result);
  assert.deepEqual(await browserClient.profileUpdate(profileRequest.body), profile.body.result);
  for (const request of deployed.state.requests) {
    assert.equal(request.headers.cookie, cookie);
    assert.equal(request.headers.origin, webOrigin);
    assert.equal(request.headers["x-layerx-trace"], trace);
    assert.equal(request.headers.host, new URL(deployed.origin).host);
    if (request.method === "PATCH") {
      assert.equal(request.url, profileRequest.path);
      assert.equal(request.headers["content-type"], "application/json");
      assert.equal(request.headers["x-layerx-csrf"], csrfToken);
      assert.deepEqual(JSON.parse(request.body), profileRequest.body);
    } else {
      assert.equal(request.method, "GET");
      assert.equal(request.url, "/v1/version");
      assert.equal(request.body, "");
    }
  }
  assert.equal(deployed.state.requests.length, 4);
  assert.equal(built.state.requests.length, 0);

  for (const prefix of ["/human/v1", "/v1"]) {
    const response = await fetch(web.origin + prefix + "/version?cursor=a%2Fb&cursor=c");
    assert.equal(response.status, version.status);
    assert.deepEqual(await response.json(), version.body);
    assert.equal(response.headers.get("cache-control"), "no-store");
    assert.deepEqual(response.headers.getSetCookie(), [
      "__Host-layerx-session=renewed; Path=/; Secure; HttpOnly; SameSite=Strict",
      "__Host-layerx_csrf=renewed-csrf; Path=/; Secure; SameSite=Strict",
    ]);
    assert.equal(deployed.state.requests.at(-1).url, "/v1/version?cursor=a%2Fb&cursor=c");
  }
  const ready = await fetch(web.origin + "/readyz");
  assert.deepEqual(await ready.json(), { ready: true });
  assert.equal(ready.headers.get("cache-control"), "no-store");
  assert.equal(deployed.state.requests.at(-1).url, "/readyz");

  deployed.state.rejectProfile = true;
  for (const client of [serverClient, browserClient]) {
    await assert.rejects(client.profileUpdate(profileRequest.body), (error) => {
      assert.equal(error.status, profileFailure.status);
      assert.equal(error.trace, profileFailure.body.trace);
      assert.equal(error.detail.code, profileFailure.body.error.code);
      return true;
    });
  }
  deployed.state.redirect = built.origin + "/v1/version";
  const redirect = await fetch(web.origin + "/human/v1/redirect", { redirect: "manual" });
  assert.equal(redirect.status, 307);
  assert.equal(redirect.headers.get("location"), deployed.state.redirect);
  assert.equal(built.state.requests.length, 0, "proxy must not follow an upstream redirect");

  // The same built routes must also work when deploying back to another origin.
  process.env.LAYERX_HUMAN_SERVICE_URL = built.origin;
  assert.deepEqual(await humanApiForServer({ cookie }).version(), version.body.result);
  assert.deepEqual(await browserClient.version(), version.body.result);
  const switchedReady = await fetch(web.origin + "/readyz");
  assert.equal(switchedReady.status, 200);
  assert.deepEqual(await switchedReady.json(), { ready: true });
  assert.deepEqual(built.state.requests.map(({ url }) => url), ["/v1/version", "/v1/version", "/readyz"]);
  assert.deepEqual(web.failures, []);
}

async function human_origin_validation(context) {
  restoreEnvironment(context);
  process.env.LAYERX_HUMAN_WEB_ORIGIN = webOrigin;
  for (const service of [
    undefined, "", "invalid", "http://127.0.0.1:9443", "ftp://human.example.com",
    "https://user:password@human.example.com", "https://human.example.com/other",
    "https://human.example.com?route=other", "https://human.example.com#other",
  ]) {
    if (service === undefined) delete process.env.LAYERX_HUMAN_SERVICE_URL;
    else process.env.LAYERX_HUMAN_SERVICE_URL = service;
    assert.throws(() => humanServiceOrigin());
    assert.throws(() => humanApiForServer({ cookie }));
    for (const path of [["v1", "version"], ["readyz"]]) {
      await assert.rejects(route.GET(new Request("https://app.example.com/readyz"), {
        params: Promise.resolve({ path }),
      }));
    }
  }
  for (const suffix of ["", "/", "/human", "/human/"]) {
    process.env.LAYERX_HUMAN_SERVICE_URL = "https://human.example.com:9443" + suffix;
    assert.equal(humanServiceOrigin(), "https://human.example.com:9443");
  }
}

async function human_proxy_path_boundary(context) {
  restoreEnvironment(context);
  // An invalid origin would throw if any rejected path attempted a transport.
  process.env.LAYERX_HUMAN_SERVICE_URL = "invalid";
  for (const path of [[], ["admin"], ["readyz", "other"], ["v1", ".."], ["v1", "x/y"], ["v1", "x\\y"]]) {
    const response = await route.GET(new Request("https://app.example.com/api/human/other"), {
      params: Promise.resolve({ path }),
    });
    assert.equal(response.status, 404);
  }
  for (const method of ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"]) {
    assert.equal(typeof route[method], "function");
  }
}

test(human_build_runtime_origin);
test(human_origin_validation);
test(human_proxy_path_boundary);
