Code.require_file("../ethereum_jsonrpc/paxeer_x_capture.exs", __DIR__)

defmodule EthereumJSONRPC.PaxeerXCaptureTest do
  @moduledoc "Replays recorded Paxeer headers, sync progress, traces and transaction counts through the JSON-RPC consumers."

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
