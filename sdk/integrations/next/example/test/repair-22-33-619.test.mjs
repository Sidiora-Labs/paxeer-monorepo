// Run the mounted-route proof and all retained concurrency, migration and crash
// recovery regressions through the task's single declared verification command.
// Finding 023 is proved by finding023_mountedPaidRouteReplaysCurrentEvidence:
// recorded native receipt -> verifyRpcPayment -> payment signature header ->
// loopback HTTP -> shipped Next GET -> mounted seller -> FileFulfillmentRepository
// -> resource and settlement response, including replay after closing and reopening.
// Baseline: earlier attempts reached no tests until dependency wiring was repaired;
// all six tests passed in the carried-forward attempt. This continuation runs only
// the ordered hf-build/hf-done gates; their private ledger records the revision,
// exact commands, exit codes and logs. No extra baseline verification is claimed.
// Wiring resolved by 22.31.457 and 22.33.1477: seller commitment and buyer resolver
// interfaces are provided by their owning middleware domains on the current base.
// source-runtime.mjs loads committed production sources for both the mounted
// handler and the crash process without invoking a subprocess toolchain.
// The scoped task record maps finding 023 to its proof and names prerequisite
// wiring owned by the Next integration and middleware domains.
await import("./fulfillment.test.mjs");
