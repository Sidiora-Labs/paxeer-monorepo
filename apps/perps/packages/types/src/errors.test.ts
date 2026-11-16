import { ApiError, SDKError } from "./errors";

test("SDK errors preserve their message and identity", () => {
  const error = new SDKError("Connect a wallet");
  expect(error).toBeInstanceOf(Error);
  expect(error.name).toBe("SDKError");
  expect(error.message).toBe("[PAXEER_NETWORK SDK ERROR]:Connect a wallet");
});

test("API errors preserve their message and identity", () => {
  const error = new ApiError("Order rejected", 400);
  expect(error).toBeInstanceOf(Error);
  expect(error.name).toBe("ApiError");
  expect(error.message).toBe("Order rejected");
});
