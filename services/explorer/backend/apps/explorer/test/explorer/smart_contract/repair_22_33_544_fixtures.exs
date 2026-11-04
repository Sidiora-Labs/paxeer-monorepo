defmodule Explorer.SmartContract.Repair2233544Fixtures do
  import ExUnit.Assertions
  import ExUnit.Callbacks, only: [on_exit: 1]

  alias Explorer.SmartContract.{Helper, Reader}

  @address "0x0000000000000000000000000000000000000001"
  @bytes_abi [
    %{
      "type" => "function",
      "stateMutability" => "view",
      "outputs" => [%{"type" => "bytes16", "name" => "data", "internalType" => "bytes16"}],
      "name" => "getData",
      "inputs" => []
    }
  ]

  def assert_binary_getter do
    bypass = Bypass.open()
    value = "0x123456789@ABCD"
    result = "0x" <> Base.encode16(ABI.TypeEncoder.encode([value], [{:bytes, 16}]), case: :lower)

    assert result == "0x3078313233343536373839404142434400000000000000000000000000000000"

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)

      assert [
               %{
                 "id" => id,
                 "jsonrpc" => "2.0",
                 "method" => "eth_call",
                 "params" => [%{"data" => "0x3bc5de30", "to" => @address}, "latest"]
               }
             ] = Jason.decode!(body)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!([%{id: id, jsonrpc: "2.0", result: result}]))
    end)

    arguments = rpc_arguments(bypass)
    previous = Application.fetch_env(:explorer, :json_rpc_named_arguments)
    on_exit(fn -> restore_env(previous) end)
    Application.put_env(:explorer, :json_rpc_named_arguments, arguments)

    assert %{"3bc5de30" => {:ok, [^value]}} = Reader.query_contract(@address, @bytes_abi, %{"3bc5de30" => []}, false)

    assert "0x30783132333435363738394041424344" ==
             Helper.get_binary_string_from_contract_getter("3bc5de30", @address, @bytes_abi)
  end

  def rpc_arguments(bypass) do
    [
      variant: EthereumJSONRPC.Geth,
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{bypass.port}"],
        fallback_urls: [],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]
  end

  defp restore_env({:ok, value}), do: Application.put_env(:explorer, :json_rpc_named_arguments, value)
  defp restore_env(:error), do: Application.delete_env(:explorer, :json_rpc_named_arguments)
end
