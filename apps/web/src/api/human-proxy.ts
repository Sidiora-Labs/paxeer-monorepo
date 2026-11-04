import { humanServiceOrigin } from "./human-origin.ts";

function transportHeaders(source: Headers): Headers {
  const headers = new Headers(source);
  for (const name of (headers.get("connection") ?? "").split(",")) {
    if (name.trim() !== "") headers.delete(name.trim());
  }
  for (const name of [
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailer",
    "transfer-encoding",
    "upgrade",
    "host",
  ]) {
    headers.delete(name);
  }
  return headers;
}

export async function proxyHumanRequest(
  request: Request,
  path: readonly string[],
): Promise<Response> {
  if (
    !(path[0] === "v1" || (path.length === 1 && path[0] === "readyz")) ||
    path.some((segment) =>
      segment === "." || segment === ".." || segment.includes("/") || segment.includes("\\"),
    )
  ) {
    return new Response(null, { status: 404 });
  }
  const destination = new URL(humanServiceOrigin());
  destination.pathname = "/" + path.map(encodeURIComponent).join("/");
  destination.search = new URL(request.url).search;
  const headers = transportHeaders(request.headers);
  // Fetch decodes compressed responses; request an uncompressed representation.
  headers.set("Accept-Encoding", "identity");
  const init = {
    method: request.method,
    headers,
    body: request.body,
    duplex: "half",
    cache: "no-store",
    redirect: "manual",
    signal: request.signal,
  } satisfies RequestInit & { duplex: "half" };
  const upstream = await fetch(destination, init);
  const responseHeaders = transportHeaders(upstream.headers);
  responseHeaders.delete("content-encoding");
  responseHeaders.delete("content-length");
  responseHeaders.set("Cache-Control", "no-store");
  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: responseHeaders,
  });
}
