Code.require_file("../ethereum_jsonrpc/paxeer_x_capture.exs", __DIR__)

defmodule EthereumJSONRPC.Repair2233435Test do
  @moduledoc """
  Repair record 22.33.435, audit 22.30.147, acceptance 22998.1.

  Each case below carries its finding id. The original public captures, with
  their original source revision and values, are stored in the capture JSON files unchanged.
  Prior qualification ran 10 cases: eight passed, then syncing error propagation
  and strict head parity failed; attempt 2 left only producer head parity failing.
  No new pre-change gate is claimed: the ordered hf-build/hf-done gates supersede
  the proposal's baseline invocation.

  Attempt 3 records a committed Cosmos bank block through the production node's
  HTTP and WebSocket servers at base aa5e0820b3652890f3812a220f4b91f4f8a45387,
  including RPC repair 22.31.460 (4af5959eb28ba0c7269a509c5dfb2ea13b022b1c).
  The implementation-time Go recorder uses rpc/tests.SetupCommittedHeaderFixture,
  executes and commits the bank transaction, publishes the stashed commit through
  BlockHeaderNotifier and records both wire responses without rewriting them.
  Its source and native build outputs live outside the checkout during recording;
  the gate only reads the capture files and starts loopback Cowboy peers.

  newhead-fields-real-path: WebSocket delivery, HTTP refetch and Blocks conversion;
  mutation coverage retains number refetch and invalid-height refusal assertions.
  newhead-parity-real-path: all fields from the repaired producer's same-block
  header and body agree, including transactionsRoot, bloom and imported gas.
  syncing-real-path and sync-legacy-test: recorded false/progress and operational
  transport errors reach the production adapter; legacy v1 stays separately named.
  trace-real-path: recorded empty-block callTracer result and historical refusal
  cross HTTP and the production Geth trace importer; original call/create tests stay.
  mixed-count-test-capture: recorded eth block/count and Cosmos shell logs retain
  namespace separation; each existing synthetic index assertion remains intact.

  Wiring requests retained from the previous attempt:
  - Node RPC domain, rpc/info.go: disposable consensus Status-error capture.
  - Node RPC domain, rpc/tracers.go: nonempty current-node internal-call recording.
  - Node RPC domain, rpc/block.go: mixed EVM/Cosmos block with count and every index;
    public-node pax methods refuse with legacy_pax_deprecated.
  Head formatter wiring is satisfied by task 22.31.460; the old public mismatch
  remains explicit evidence. No out-of-scope producer changes are made here.

  Exact qualified revision, command, exit and private evidence location come from
  the runner's hf-done ledger. No completion or unrun gate is claimed here.
  """

  use ExUnit.Case, async: false

  alias EthereumJSONRPC.PaxeerXCapture

  test "newhead-fields-real-path: recorded WebSocket head drives HTTP block import" do
    PaxeerXCapture.assert_recorded_head_fields()
  end

  test "newhead-fields-real-path: mutated roots refetch by number and invalid heights are refused" do
    PaxeerXCapture.assert_recorded_head_mutations()
  end

  test "newhead-parity-real-path: recorded WebSocket and HTTP fields agree in the imported block" do
    PaxeerXCapture.assert_recorded_head_parity()
  end

  test "syncing-real-path: recorded consensus catch-up progress reaches the adapter over HTTP" do
    PaxeerXCapture.assert_recorded_syncing("syncing")
  end

  test "sync-legacy-test: recorded current false reaches the adapter over HTTP" do
    PaxeerXCapture.assert_recorded_syncing("synced")
  end

  test "sync-legacy-test: a real HTTP transport error must remain an adapter error" do
    PaxeerXCapture.assert_syncing_transport_error()
  end

  test "trace-real-path: recorded empty block crosses HTTP and the real trace importer" do
    PaxeerXCapture.assert_recorded_trace()
  end

  test "trace-real-path: recorded historical-block refusal survives the trace importer" do
    PaxeerXCapture.assert_recorded_trace_refusal()
  end

  test "mixed-count-test-capture: recorded eth block/count exclude recorded Cosmos shell activity" do
    PaxeerXCapture.assert_recorded_counts()
  end

  test "mixed-count-test-capture: recorded pax refusals do not become eth visibility" do
    PaxeerXCapture.assert_recorded_pax_refusals()
  end
end
