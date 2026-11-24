export async function request(url: string, options: RequestInit = {}) {
  const method = (options.method ?? "GET").toUpperCase();
  const headers = new Headers(options.headers);
  if (method !== "GET" && method !== "HEAD" && !headers.has("Content-Type")) {
    headers.set(
      "Content-Type",
      method === "DELETE"
        ? "application/x-www-form-urlencoded"
        : "application/json;charset=utf-8",
    );
  }
  const init = { ...options, method, headers };
  const { nativeRequest } =
    await import("@paxeer-network/paxeer-backend/nativeClient");
  return nativeRequest(url, init);
}
export async function get<R>(
  url: string,
  options: RequestInit = {},
  formatter?: (data: any) => R,
): Promise<R> {
  const result = await request(url, { ...options, method: "GET" });
  if (!result.success)
    throw new Error(result.message ?? "Paxeer request failed.");
  return formatter
    ? formatter(result.data)
    : Array.isArray(result.data?.rows)
      ? result.data.rows
      : result.data;
}
export const post = (url: string, data: any, options: RequestInit = {}) =>
  request(url, { ...options, method: "POST", body: JSON.stringify(data) });
export const put = (url: string, data: any, options: RequestInit = {}) =>
  request(url, { ...options, method: "PUT", body: JSON.stringify(data) });
export const del = (url: string, options: RequestInit = {}) =>
  request(url, { ...options, method: "DELETE" });
export const mutate = (url: string, options: RequestInit) =>
  request(url, options);
