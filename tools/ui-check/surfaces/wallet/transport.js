(() => {
  const original = window.fetch.bind(window);
  const origin = "http://127.0.0.1:4174";
  window.fetch = async (input, init) => {
    const request = new Request(
      new URL(
        typeof input === "string"
          ? input
          : input instanceof URL
            ? input.href
            : input.url,
        location.href,
      ),
      input instanceof Request ? input : init,
    );
    const url = new URL(request.url);
    if (url.href === `${origin}/rpc` && request.method === "POST") {
      const calls = await request.json();
      let status = 200;
      const results = await Promise.all(
        (Array.isArray(calls) ? calls : [calls]).map(async (call) => {
          const suffix =
            call.method === "eth_call"
              ? `/${call.params[0].to.toLowerCase()}/${call.params[0].data.slice(0, 10).toLowerCase()}`
              : "";
          const answer = await original(
            `${origin}/node/${call.method}${suffix}`,
            {
              method: "POST",
              headers: { "content-type": "application/json" },
              body: JSON.stringify(call),
              signal: request.signal,
            },
          );
          if (!answer.ok) status = answer.status;
          return { ...(await answer.json()), jsonrpc: "2.0", id: call.id };
        }),
      );
      return new Response(
        JSON.stringify(Array.isArray(calls) ? results : results[0]),
        {
          status,
          headers: { "content-type": "application/json" },
        },
      );
    }
    if (
      url.origin === location.origin &&
      url.pathname.startsWith("/wallet/api/")
    ) {
      return original(
        `${origin}/reads/${url.pathname.slice("/wallet/api/".length)}${url.search}`,
        {
          method: request.method,
          headers: request.headers,
          body:
            request.method === "GET" || request.method === "HEAD"
              ? undefined
              : await request.text(),
          signal: request.signal,
        },
      );
    }
    return original(request);
  };
})();
