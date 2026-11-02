export interface Eip1193Provider {
  request(args: Readonly<{ method: string; params?: readonly unknown[] }>): Promise<unknown>;
}

export function windowWalletProvider(): Eip1193Provider | undefined {
  if (typeof window === "undefined") {
    return undefined;
  }
  const candidate = (window as Readonly<{ ethereum?: unknown }>).ethereum;
  if (typeof candidate !== "object" || candidate === null) {
    return undefined;
  }
  const request = (candidate as Readonly<{ request?: unknown }>).request;
  return typeof request === "function" ? (candidate as Eip1193Provider) : undefined;
}
