import { nativeRequest } from "@paxeer-network/paxeer-backend/nativeClient";
import { del, get, mutate, post, put } from "../src";
import { request } from "../src/fetch";

jest.mock("@paxeer-network/paxeer-backend/nativeClient", () => ({
  nativeRequest: jest.fn(),
}));

const url = "https://api.example.com/v1/test";
const nativeRequestMock = jest.mocked(nativeRequest);

const getRequestHeaders = () => {
  const init = nativeRequestMock.mock.calls.at(-1)?.[1] as RequestInit;
  return new Headers(init.headers);
};

describe("fetch request headers", () => {
  beforeEach(() => {
    nativeRequestMock.mockReset();
    nativeRequestMock.mockResolvedValue({
      success: true,
      data: { rows: [] },
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
  });

  it("does not add Content-Type to GET requests", async () => {
    await get(url);

    expect(getRequestHeaders().has("Content-Type")).toBe(false);
  });

  it.each([undefined, "get", "HEAD", "head"])(
    "does not add Content-Type when method is %s",
    async (method) => {
      await request(url, method ? { method } : {});

      expect(getRequestHeaders().has("Content-Type")).toBe(false);
    },
  );

  it("preserves an explicit GET Content-Type", async () => {
    await get(url, { headers: { "Content-Type": "application/custom" } });

    expect(getRequestHeaders().get("Content-Type")).toBe("application/custom");
  });

  it.each([
    ["POST", () => post(url, { value: 1 }), "application/json;charset=utf-8"],
    ["PUT", () => put(url, { value: 1 }), "application/json;charset=utf-8"],
    ["DELETE", () => del(url), "application/x-www-form-urlencoded"],
    [
      "lowercase DELETE",
      () => request(url, { method: "delete" }),
      "application/x-www-form-urlencoded",
    ],
  ])("keeps the existing %s Content-Type default", async (_name, run, type) => {
    await run();

    expect(getRequestHeaders().get("Content-Type")).toBe(type);
  });

  it.each(["HEAD", "POST", "PUT", "PATCH", "DELETE"])(
    "preserves caller headers and body for %s",
    async (method) => {
      const body = method === "HEAD" ? undefined : JSON.stringify({ value: 1 });
      const headers = new Headers({
        "content-type": "application/custom",
        "X-Request-Id": "public-info",
      });

      await request(url, { method: method.toLowerCase(), headers, body });

      expect(nativeRequestMock).toHaveBeenCalledTimes(1);
      expect(nativeRequestMock).toHaveBeenCalledWith(url, {
        method,
        headers,
        body,
      });
      expect(getRequestHeaders().get("Content-Type")).toBe(
        "application/custom",
      );
      expect(getRequestHeaders().get("X-Request-Id")).toBe("public-info");
    },
  );

  it.each(["https://api.example.com", ""])(
    "hands the public info address with api base %s to the client",
    async (apiBaseUrl) => {
      const fetchSpy = jest.spyOn(globalThis, "fetch");
      const fullUrl = `${apiBaseUrl}/v1/public/info`;
      const rows = [{ symbol: "PERP_ETH_USDC" }];
      nativeRequestMock.mockResolvedValueOnce({
        success: true,
        data: { rows },
      });

      await expect(get(fullUrl)).resolves.toBe(rows);

      expect(nativeRequestMock).toHaveBeenCalledTimes(1);
      expect(nativeRequestMock).toHaveBeenCalledWith(fullUrl, {
        method: "GET",
        headers: new Headers(),
      });
      expect(fetchSpy).not.toHaveBeenCalled();
    },
  );

  it("passes a client error through get unchanged", async () => {
    const error = new Error("Public info is unavailable");
    nativeRequestMock.mockRejectedValueOnce(error);

    await expect(get(url)).rejects.toBe(error);
  });

  it.each([
    ["POST", () => post(url, { value: 1 }), JSON.stringify({ value: 1 })],
    ["PUT", () => put(url, { value: 1 }), JSON.stringify({ value: 1 })],
    ["DELETE", () => del(url), undefined],
    [
      "PATCH",
      () =>
        mutate(url, { method: "PATCH", body: JSON.stringify({ value: 1 }) }),
      JSON.stringify({ value: 1 }),
    ],
  ])("returns the client answer for %s", async (method, run, body) => {
    const answer = { success: true, data: { value: 1 } };
    nativeRequestMock.mockResolvedValueOnce(answer);

    await expect(run()).resolves.toBe(answer);

    expect(nativeRequestMock).toHaveBeenCalledWith(
      url,
      expect.objectContaining({
        method,
        ...(body === undefined ? {} : { body }),
      }),
    );
  });

  it("returns client data through get", async () => {
    const data = { value: 1 };
    nativeRequestMock.mockResolvedValueOnce({ success: true, data });

    await expect(get(url)).resolves.toBe(data);
  });

  it("formats client data through get", async () => {
    nativeRequestMock.mockResolvedValueOnce({
      success: true,
      data: { value: 1 },
    });

    await expect(get(url, {}, (data) => data.value)).resolves.toBe(1);
  });

  it("rejects an unsuccessful client answer through get", async () => {
    nativeRequestMock.mockResolvedValueOnce({
      success: false,
      message: "Unavailable",
    });

    await expect(get(url)).rejects.toThrow("Unavailable");
  });
});
