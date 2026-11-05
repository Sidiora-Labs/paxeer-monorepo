import assert from "node:assert/strict";

import {
  decodeResponseEnvelope,
  type JsonValue,
} from "../../src/api/generated/index.ts";

interface SettingsHttpResponse {
  ok(): boolean;
  json(): Promise<unknown>;
}

export async function readSettingsResponse<Result>(
  response: SettingsHttpResponse,
  decode: (value: JsonValue | undefined, at: string) => Result,
  at: string,
): Promise<Result> {
  assert.equal(response.ok(), true, `${at}: HTTP request must succeed`);
  const envelope = decodeResponseEnvelope(await response.json() as JsonValue, at);
  assert.equal(envelope.ok, true, `${at}: service request must succeed`);
  return decode(envelope.result, `${at}.result`);
}
