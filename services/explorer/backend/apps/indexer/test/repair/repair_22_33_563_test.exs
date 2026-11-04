# Load the original cases even when this gate is compiled for another chain.
# Only the compile-time chain guard is unwrapped; the module and tests are intact.
source = Path.expand("../indexer/transform/stability/validators_test.exs", __DIR__)
{:if, _, [_, [do: original_module]]} = source |> File.read!() |> Code.string_to_quoted!()

unless Code.ensure_loaded?(Indexer.Transform.Stability.ValidatorsTest) do
  Code.eval_quoted(original_module, [], file: source)
end

defmodule Indexer.Repair2233563Test do
  use ExUnit.Case, async: false

  alias Explorer.Chain.Stability.Validator
  alias Indexer.Transform.Stability.Validators

  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json", __DIR__)
  @external_resource @fixture
  @miner "0xb3ad5a835b418958a9142d8681bfb4b81d2cf270"

  setup do
    original = Application.fetch_env(:explorer, :chain_type)

    on_exit(fn ->
      case original do
        {:ok, value} -> Application.put_env(:explorer, :chain_type, value)
        :error -> Application.delete_env(:explorer, :chain_type)
      end
    end)

    :ok
  end

  test "027: the original chain configuration cases are scheduled synchronously" do
    suite = Indexer.Transform.Stability.ValidatorsTest.__ex_unit__()
    assert Indexer.Transform.Stability.ValidatorsTest.__ex_unit__(:config).async? == false
    assert length(suite.tests) == 6

    for test <- suite.tests do
      refute Map.get(test.tags, :skip, false)
    end
  end

  test "027: recorded RPC blocks reach the validator schema under serialized chain selection" do
    recorded = @fixture |> File.read!() |> Jason.decode!()
    blocks_by_number = Map.new(recorded, fn {number, block} -> {EthereumJSONRPC.quantity_to_integer(number), block} end)
    bypass = Bypass.open()
    owner = self()

    Bypass.expect_once(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)

      assert Enum.map(requests, &Map.take(&1, ["jsonrpc", "method", "params"])) == [
               %{"jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => ["0x1714EFD", true]},
               %{"jsonrpc" => "2.0", "method" => "eth_getBlockByNumber", "params" => ["0x1714EFE", true]}
             ]

      responses =
        requests
        |> Enum.reverse()
        |> Enum.map(fn %{"id" => id, "params" => [number, true]} ->
          block = Map.fetch!(blocks_by_number, EthereumJSONRPC.quantity_to_integer(number))
          %{"jsonrpc" => "2.0", "id" => id, "result" => block}
        end)

      send(owner, {:recorded_blocks, Enum.map(responses, & &1["result"]["hash"])})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://127.0.0.1:#{bypass.port}/"],
        http_options: [timeout: 2_000, recv_timeout: 2_000]
      ]
    ]

    assert {:ok, fetched} = EthereumJSONRPC.fetch_blocks_by_range(24_203_005..24_203_006, arguments)
    assert fetched.errors == []
    assert Enum.sort(Enum.map(fetched.blocks_params, & &1.number)) == [24_203_005, 24_203_006]
    assert Enum.map(fetched.blocks_params, & &1.miner_hash) == [@miner, @miner]
    assert_receive {:recorded_blocks, hashes}
    assert MapSet.new(hashes) == MapSet.new(recorded, fn {_, block} -> block["hash"] end)

    for chain_type <- [:stability, :ethereum, :stability] do
      Application.put_env(:explorer, :chain_type, chain_type)
      updates = Validators.parse(fetched.blocks_params)

      case chain_type do
        :stability ->
          assert updates == [%{address_hash: @miner, blocks_validated: 2}]
          [params] = updates
          changeset = Validator.changeset(%Validator{}, params)
          assert changeset.valid?, inspect(changeset.errors)
          validator = Ecto.Changeset.apply_changes(changeset)
          assert to_string(validator.address_hash) == @miner
          assert validator.blocks_validated == 2

        :ethereum ->
          assert updates == []
      end

      assert Application.fetch_env!(:explorer, :chain_type) == chain_type
    end
  end
end
