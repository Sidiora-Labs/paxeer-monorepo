import { encode } from "bs58";
import { MockKeyStore, BaseSigner, type MessageFactor } from "../src";

beforeAll(() => {
  const crypto = require("crypto");
  Object.defineProperty(globalThis, "crypto", {
    value: crypto.webcrypto,
    configurable: true,
  });
});

describe("signer test", () => {
  const keystore = new MockKeyStore(
    encode(
      Buffer.from(
        "0cc31f8a253c92b5cf9461f8d7a128ef24083eb8556f5a73831558fade2c3ff4",
        "hex",
      ),
    ),
  );
  const signer = new BaseSigner(keystore);

  test("signer", async () => {
    const message: MessageFactor = {
      url: "http://localhost:8080/api/v1/transactions",
      method: "POST",
      // data: {},
    };
    const signedMessage = await signer.sign(message);
  });
});
