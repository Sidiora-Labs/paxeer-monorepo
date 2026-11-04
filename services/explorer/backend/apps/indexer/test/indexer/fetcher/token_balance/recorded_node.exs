defmodule Indexer.Fetcher.TokenBalance.RecordedNode do
  @moduledoc false

  import ExUnit.Assertions

  @fixture Path.join(__DIR__, "recorded_eth_call.json")

  def transcript(name) do
    @fixture |> File.read!() |> Jason.decode!() |> Map.fetch!(name)
  end

  def start do
    previous = Application.fetch_env!(:explorer, :json_rpc_named_arguments)

    ExUnit.Callbacks.on_exit(fn ->
      Application.put_env(:explorer, :json_rpc_named_arguments, previous)
    end)

    Bypass.open()
  end

  def expect_response(node, name, address_hash, token_contract_address_hash, block_number) do
    record = transcript(name)
    [request] = record["request"]
    owner = self()
    path = "/#{name}/#{block_number}"

    expected = %{
      request
      | "params" => [
          %{
            "to" => to_string(token_contract_address_hash),
            "data" => "0x70a08231" <> String.pad_leading(String.slice(to_string(address_hash), 2..-1//1), 64, "0")
          },
          EthereumJSONRPC.integer_to_quantity(block_number)
        ]
    }

    Bypass.expect_once(node, "POST", path, fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      assert Jason.decode!(body) == [expected]
      send(owner, {:recorded_eth_call, name, expected})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(record["response"]))
    end)

    url = "http://127.0.0.1:#{node.port}#{path}"

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      variant: EthereumJSONRPC.Geth,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: [url],
        fallback_urls: [url],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ]
    ]

    Application.put_env(:explorer, :json_rpc_named_arguments, arguments)
    arguments
  end
end
