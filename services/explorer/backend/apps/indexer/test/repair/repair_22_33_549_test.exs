Code.require_file(
  Path.expand("../../../explorer/priv/arbitrum/migrations/20240201125730_create_arbitrum_tables.exs", __DIR__)
)

defmodule Indexer.Repair2233549Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.Arbitrum.Message
  alias Explorer.Chain.Arbitrum.Reader.Indexer.Messages, as: Reader
  alias Explorer.Repo.Arbitrum.Migrations.CreateArbitrumTables
  alias Indexer.Fetcher.Arbitrum
  alias Indexer.Fetcher.Arbitrum.Utils.Db.Messages, as: DbMessages
  alias Indexer.Fetcher.Arbitrum.Utils.Rpc

  @moduletag :capture_log

  setup do
    config = Application.get_env(:indexer, Arbitrum)
    Application.put_env(:indexer, Arbitrum, Keyword.put(config, :l1_finalization_threshold, 10))
    on_exit(fn -> Application.put_env(:indexer, Arbitrum, config) end)

    # Run the production migration in the sandbox transaction so the proof also
    # runs when the default chain's migrations do not include Arbitrum tables.
    Repo.query!("CREATE SCHEMA repair_22_33_549")
    Repo.query!("SET LOCAL search_path TO repair_22_33_549, public")

    Ecto.Migration.Runner.run(
      Repo,
      Repo.config(),
      20_240_201_125_730,
      CreateArbitrumTables,
      :forward,
      :change,
      :up,
      log: false
    )

    :ok
  end

  test "negative-safe-height: young chains retain explicit no-safe-message history" do
    put_message(1, 1, :relayed)

    for latest <- [0, 1, 8] do
      args = serve_blocks(latest)
      {safe, ^latest} = Rpc.get_safe_and_latest_l1_blocks(args, 10)

      assert safe == 0
      assert Reader.highest_safe_fully_indexed_message_to_l2(safe) == nil
      assert DbMessages.highest_safe_fully_indexed_message_to_l2(safe, nil) == nil
      assert DbMessages.highest_safe_fully_indexed_message_to_l2(safe, 7) == 7
      assert_requested_tags()
    end
  end

  test "negative-safe-height: genesis messages remain eligible without treating message zero as missing" do
    put_message(0, 0, :relayed)
    put_message(1, 1, :relayed)
    put_message(99, 0, :initiated)
    put_message(100, 0, :relayed, :from_l2)

    {safe, latest} = Rpc.get_safe_and_latest_l1_blocks(serve_blocks(1), 10)

    assert {safe, latest} == {0, 1}
    assert Reader.highest_safe_fully_indexed_message_to_l2(safe) == 0
    assert DbMessages.highest_safe_fully_indexed_message_to_l2(safe, nil) == 0
    assert_requested_tags()
  end

  test "negative-safe-height: threshold boundaries and the hard limit retain their message cutoff" do
    for height <- 0..12, do: put_message(height, height, :relayed)

    for {latest, hard_limit, expected_safe} <- [{9, 10, 0}, {10, 10, 1}, {12, 10, 3}, {5, 3, 3}, {5, 20, 0}] do
      {safe, ^latest} = Rpc.get_safe_and_latest_l1_blocks(serve_blocks(latest), hard_limit)

      assert safe == expected_safe
      assert Reader.highest_safe_fully_indexed_message_to_l2(safe) == expected_safe
      assert DbMessages.highest_safe_fully_indexed_message_to_l2(safe, nil) == expected_safe
      assert_requested_tags()
    end
  end

  test "negative-safe-height: supported safe tags and stale safe-tag fallback retain their cutoffs" do
    for height <- 0..12, do: put_message(height, height, :relayed)

    for {safe_tag, expected_safe} <- [{0, 3}, {2, 3}, {3, 3}, {7, 7}] do
      {safe, latest} = Rpc.get_safe_and_latest_l1_blocks(serve_blocks(12, safe_tag), 10)

      assert {safe, latest} == {expected_safe, 12}
      assert Reader.highest_safe_fully_indexed_message_to_l2(safe) == expected_safe
      assert DbMessages.highest_safe_fully_indexed_message_to_l2(safe, nil) == expected_safe
      assert_requested_tags()
    end
  end

  defp put_message(id, height, status, direction \\ :to_l2) do
    %Message{}
    |> Message.changeset(%{
      direction: direction,
      message_id: id,
      originating_transaction_block_number: height,
      status: status
    })
    |> Repo.insert!()
  end

  defp serve_blocks(latest, safe \\ :unsupported) do
    bypass = Bypass.open()
    owner = self()

    fixture =
      Path.expand("../../../explorer/test/support/fixture/vcr_cassettes/block_importer_import_1_pending.json", __DIR__)

    [record] = fixture |> File.read!() |> Jason.decode!()
    block = record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)

      responses =
        body
        |> Jason.decode!()
        |> Enum.map(fn request ->
          assert %{
                   "jsonrpc" => "2.0",
                   "id" => id,
                   "method" => "eth_getBlockByNumber",
                   "params" => [tag, false]
                 } = request

          assert tag in ["safe", "latest"]
          send(owner, {:block_tag, tag})

          if tag == "safe" and safe == :unsupported do
            %{"jsonrpc" => "2.0", "id" => id, "error" => %{"code" => -32602, "message" => "unsupported block tag"}}
          else
            height = if tag == "safe", do: safe, else: latest
            result = Map.put(block, "number", EthereumJSONRPC.integer_to_quantity(height))
            %{"jsonrpc" => "2.0", "id" => id, "result" => result}
          end
        end)

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{bypass.port}"],
        http_options: []
      ]
    ]
  end

  defp assert_requested_tags do
    assert_receive {:block_tag, "safe"}
    assert_receive {:block_tag, "latest"}
  end
end
