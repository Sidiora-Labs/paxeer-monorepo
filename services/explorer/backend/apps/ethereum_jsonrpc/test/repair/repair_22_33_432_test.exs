Code.require_file("../ethereum_jsonrpc/web_socket/lifecycle.exs", __DIR__)

defmodule EthereumJSONRPC.Repair2233432Test do
  use ExUnit.Case, async: false

  alias EthereumJSONRPC.WebSocket.Lifecycle

  @tag timeout: 60_000
  test "ws-reconnect-test-absent: retired PID fails while the original handle cancels on the recovered node" do
    Lifecycle.assert_primary_recovery()
  end

  @tag timeout: 60_000
  test "ws-reconnect-test-absent: verified TLS reconnect delivers recorded headers and cancels the new server ID" do
    Lifecycle.assert_tls_reconnect()
  end
end
