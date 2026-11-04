package human_test

/*
Task 22.33.1402 repair record (audit 22.30.78, acceptance 22998.1).

The task's step 10 and gotcha 2984 map the originally planned Go proofs to
test_repair_22_33_1402.py. This file retains the declared audit-record path;
the Python tests exercise the production Python components in process, with
the committed TLS loopback peer, without a Go adapter or subprocess gate.

finding.owner-test-current-success
Proving test: test_TestRepairM_1402_OwnerTestCurrentSuccess.
test_owner_native.py now drives owner_native.produce through merged_owner's
real codec and HTTPS RPC client into provision.owner_registration and
provision.funding_evidence. The committed peer persists accounts, identities,
transactions and receipts. The test asserts exact source debit and owner
credit, committed recovery policy and identity, stops and reopens the peer,
then checks unchanged identity, evidence, balances and submitted transactions.
Missing funding evidence and wrong owner identity still refuse; guardian
signature, unsigned-set, substitution and rotation cases remain asserted.

finding.web-explorer-relay
Proving test: test_TestRepairM_1402_WebExplorerRelay.
web-deployment.yaml uses PAXEER_X_EXPLORER_BASE_URL through the required
layerx-human-web-explorer/base-url ConfigMap, the internal CA Secret and
paired HTTPS network policies. Retired explorer-index Secrets, bearer/naming
configuration and the obsolete explorer relay have been removed together.
The current explorer client uses public native APIs with server TLS trust;
it does not consume the retired bearer token.
Observed: apps/web/src/explorer/client.ts programRecord validates its input
and explorer origin, then throws ExplorerInterfaceUnavailableError("program-read")
(interface.8.err-core-program-read-refused). Per step 11 this is a documented
refusal, not a failed prerequisite for this repair. The gate checks the
deployment contract and documented refusal; it claims no successful deployed
program read. The external integration remains a wiring request.

Wiring request, domain 10: apps/web/src/explorer/client.ts and
apps/web/src/explorer/model.ts must wire and consume the native deployed-program
read when domain 14 publishes it, retaining explicit unavailable refusals.
Wiring request, domain 14: services/explorer/backend and services/explorer/deploy
must publish that read and provision layerx-human-web-explorer/base-url with
the current HTTPS explorer origin, a Service on port 443 and backend pod label
app=paxeer-x-explorer-backend in layerx-testnet to match the paired policies.

Baseline: continued attempt 3's applied owner/deployment repair, without
reverting it. Its reported gate failed both named proving tests: the owner
loader lacked services/platform/hosted/paxeer/evm.py and the web check required
a successful program read. Prerequisite 22.31.467 restores the production
codec; step 11 explicitly corrects the program-read expectation. No additional
pre-change verification is claimed: the wrapper-only, implementation-first
order controls this continuation. The inherited repair_22_33_1402.kvx remains
untouched outside scope; this record supersedes its outdated missing-codec
and required-program-success observations.

Qualification evidence: hf-build and hf-done record the exact candidate
revision, command, exit code and private log in /out/run/gates.jsonl.
The final hf-done evidence line identifies the verified revision; no source
record claims qualification before that gate runs. No extra gate is required.
*/
