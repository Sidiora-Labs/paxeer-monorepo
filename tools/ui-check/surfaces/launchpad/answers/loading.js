(() => {
  const fetch = globalThis.fetch.bind(globalThis);
  globalThis.fetch = (input, init) => {
    const request = input instanceof Request ? input : null;
    const url = new URL(request ? request.url : input, location.href);
    const method = (init?.method ?? request?.method ?? 'GET').toUpperCase();
    if (method === 'GET' && url.origin === location.origin
      && /^\/(v[123]\/|users\/|stats\/|metadata\/|explorer\/api\/)/.test(url.pathname)
      && !url.pathname.startsWith('/v2/network/') && url.pathname !== '/v2/snapshots') {
      return new Promise(() => {});
    }
    return fetch(input, init);
  };
})();
