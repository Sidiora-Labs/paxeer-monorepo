(() => {
  const scenario =
    new URL(location.href).searchParams.get("fixture") || "anonymous";
  const nativeFetch = window.fetch.bind(window);
  let provisioned = false;
  const gateway = "http://localhost:3001";
  const auth = "http://localhost:54321";
  const pause = () => new Promise((resolve) => setTimeout(resolve, 30000));

  if (scenario.startsWith("embedded-") || scenario === "callback-signed-in") {
    const expires = Math.floor(Date.now() / 1000) + 3600;
    const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
    const payload = btoa(
      JSON.stringify({ sub: walletIdentity.id, exp: expires }),
    );
    localStorage.setItem(
      "sb-localhost-auth-token",
      JSON.stringify({
        access_token: `${header}.${payload}.test-signature`,
        refresh_token: "test-refresh-token",
        expires_in: 3600,
        expires_at: expires,
        token_type: "bearer",
        user: walletIdentity,
      }),
    );
  }

  window.fetch = async (input, init) => {
    const url = new URL(
      typeof input === "string" || input instanceof URL ? input : input.url,
      location.href,
    );
    if (url.origin !== gateway && url.origin !== auth)
      return nativeFetch(input, init);
    if (scenario === "embedded-loading" && url.pathname === "/v1/wallet/me")
      await pause();
    if (
      scenario === "embedded-creating" &&
      url.pathname === "/v1/wallet/provision"
    )
      await pause();
    if (scenario === "email-sending" && url.pathname === "/auth/v1/otp")
      await pause();
    let answer = "default";
    if (url.pathname === "/v1/wallet/me") {
      if (
        !provisioned &&
        ["embedded-unprovisioned", "embedded-creating"].includes(scenario)
      )
        answer = "unprovisioned";
      if (scenario === "embedded-error") answer = "expired";
    }
    if (url.pathname === "/auth/v1/otp" && scenario === "email-error")
      answer = "refused";
    url.searchParams.set("fixture", answer);
    if (url.pathname === "/")
      url.searchParams.set("method", JSON.parse(init.body).method);
    const response = await nativeFetch(url, init);
    if (url.pathname === "/v1/wallet/provision" && response.ok)
      provisioned = true;
    return response;
  };

  if (scenario.startsWith("injected")) {
    const listeners = new EventTarget();
    const methods = {
      eth_chainId: "chain-id",
      eth_requestAccounts: "accounts",
      eth_accounts: "accounts",
      eth_sendTransaction: "transaction",
      eth_signTypedData_v4: "typed-data",
    };
    const detail = Object.freeze({
      info: Object.freeze({
        uuid: "f847897a-6873-45fb-bcc4-81041318246a",
        name: "Injected Wallet",
        icon: "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxIDEiLz4=",
        rdns: "invalid.example.injected",
      }),
      provider: {
        async request({ method }) {
          if (!Object.hasOwn(methods, method))
            throw new Error(`Unsupported wallet method: ${method}`);
          if (
            scenario === "injected-pending" &&
            method === "eth_sendTransaction"
          )
            await pause();
          const response = await nativeFetch(
            `${gateway}/replay/${methods[method]}`,
          );
          if (!response.ok)
            throw new Error(`Wallet response: ${response.status}`);
          const body = await response.json();
          if (body.error) throw new Error(body.error.message);
          return body.result;
        },
        on(event, listener) {
          listeners.addEventListener(event, listener);
          return this;
        },
        removeListener(event, listener) {
          listeners.removeEventListener(event, listener);
          return this;
        },
      },
    });
    window.addEventListener("eip6963:requestProvider", () => {
      window.dispatchEvent(
        new CustomEvent("eip6963:announceProvider", { detail }),
      );
    });
  }
})();
