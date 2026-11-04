defmodule Indexer.Fetcher.Optimism.TransactionBatchRPCFixture do
  @moduledoc false

  import ExUnit.Assertions

  alias Explorer.Chain.Hash

  @blocks_fixture Path.expand(
                    "../../../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json",
                    __DIR__
                  )

  def blocks do
    recorded = @blocks_fixture |> File.read!() |> Jason.decode!()
    Enum.map(["0x1714efd", "0x1714efe"], &Map.fetch!(recorded, &1))
  end

  def hashes do
    Enum.map(blocks(), fn block ->
      {:ok, hash} = Hash.Full.cast(block["hash"])
      hash.bytes
    end)
  end

  def unknown_hash, do: <<0::256>>

  def start_rpc(hashes) do
    owner = self()
    bypass = Bypass.open()
    results = Map.new(blocks(), &{&1["hash"], &1}) |> Map.put(encode_hash(unknown_hash()), nil)

    expected_requests =
      hashes
      |> Enum.with_index()
      |> Enum.map(fn {hash, id} ->
        %{"jsonrpc" => "2.0", "id" => id, "method" => "eth_getBlockByHash", "params" => [encode_hash(hash), false]}
      end)

    Bypass.expect_once(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      assert requests == expected_requests

      responses =
        requests
        |> Enum.reverse()
        |> Enum.map(fn %{"id" => id, "params" => [hash, false]} ->
          %{"jsonrpc" => "2.0", "id" => id, "result" => Map.fetch!(results, hash)}
        end)

      send(owner, {:block_rpc_exchange, requests, responses})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{bypass.port}/"],
        http_options: [timeout: 2_000, recv_timeout: 2_000]
      ]
    ]
  end

  defp encode_hash(bytes) do
    {:ok, hash} = Hash.Full.cast(bytes)
    to_string(hash)
  end
end
