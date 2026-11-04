queue_tests = Path.expand("../indexer/fetcher/multichain_search_db", __DIR__)

for file <- ~w(balances_export_queue_test.exs main_export_queue_test.exs token_info_export_queue_test.exs) do
  Code.require_file(file, queue_tests)
end

defmodule Indexer.Repair2233557Test do
  use ExUnit.Case, async: false
  use Explorer.DataCase

  alias Explorer.Chain
  alias Explorer.Chain.Block.Range
  alias Explorer.Chain.Cache.ChainId
  alias Explorer.Chain.MultichainSearchDb.{BalancesExportQueue, MainExportQueue, TokenInfoExportQueue}
  alias Explorer.MicroserviceInterfaces.MultichainSearch
  alias Indexer.Fetcher.MultichainSearchDb.ExportQueueContract, as: Contract

  @moduletag :capture_log
  @fixture Path.expand("../../../ethereum_jsonrpc/test/support/fixture/paxeer_x", __DIR__)

  setup do
    bypass = Bypass.open()
    Contract.preserve_env([{:explorer, MultichainSearch}, {:tesla, :adapter}])
    cached_id = ConCache.get(ChainId.cache_name(), :id)
    on_exit(fn -> ChainId.set_id(cached_id) end)
    ChainId.set_id(125)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    Application.put_env(:explorer, MultichainSearch,
      service_url: "http://localhost:#{bypass.port}",
      api_key: "recorded-import-key",
      addresses_chunk_size: 7000,
      token_info_chunk_size: 1000
    )

    transcript = @fixture |> Path.join("cosmos_originated_transaction.json") |> File.read!() |> Jason.decode!()
    %{bypass: bypass, transcript: transcript}
  end

  test "013: native, ERC-20 and NFT balances retain precision through durable replay", %{bypass: bypass} do
    fetcher = Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue
    enable(fetcher)
    {:ok, owner} = Chain.string_to_address_hash("0x66a9b160f6a06f53f23785f069882ee7337180e8")
    {:ok, erc20} = Chain.string_to_address_hash("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")
    {:ok, nft} = Chain.string_to_address_hash("0xb01b6a7ef8560017aa59a990e39f26b8df29f80f")
    value = "123456789012345678901234567890123456789"
    token_id = "900719925474099312345"

    for attrs <- [
          %{token_contract_address_hash_or_native: "native", value: value},
          %{token_contract_address_hash_or_native: erc20.bytes, value: value},
          %{token_contract_address_hash_or_native: nft.bytes, value: nil, token_id: token_id}
        ] do
      %BalancesExportQueue{}
      |> BalancesExportQueue.changeset(Map.put(attrs, :address_hash, owner))
      |> Repo.insert!()
    end

    expected =
      Map.merge(Contract.empty_batch("125", "recorded-import-key"), %{
        "address_coin_balances" => [
          %{"address_hash" => to_string(owner), "token_contract_address_hash_or_native" => "native", "value" => value}
        ],
        "address_token_balances" => [
          %{
            "address_hash" => to_string(owner),
            "token_address_hash" => to_string(erc20),
            "value" => value,
            "token_id" => nil
          },
          %{
            "address_hash" => to_string(owner),
            "token_address_hash" => to_string(nft),
            "value" => nil,
            "token_id" => token_id
          }
        ]
      })

    Contract.replay_after_restart(fetcher, BalancesExportQueue, bypass, expected)
  end

  test "034: address, block range and typed hashes survive durable replay", %{bypass: bypass, transcript: transcript} do
    fetcher = Indexer.Fetcher.MultichainSearchDb.MainExportQueue
    enable(fetcher)
    transaction = transcript["transaction"]
    {:ok, address} = Chain.string_to_address_hash(transaction["from"])
    {:ok, block_hash} = Chain.string_to_full_hash(transaction["blockHash"])
    {:ok, transaction_hash} = Chain.string_to_full_hash(transaction["hash"])
    number = EthereumJSONRPC.quantity_to_integer(transaction["blockNumber"])
    insert(:address, hash: address, fetched_coin_balance: nil)

    for {hash, type} <- [{address.bytes, :address}, {block_hash.bytes, :block}, {transaction_hash.bytes, :transaction}] do
      %MainExportQueue{}
      |> MainExportQueue.changeset(%{hash: hash, hash_type: type, block_range: %Range{from: number, to: number}})
      |> Repo.insert!()
    end

    expected =
      Map.merge(Contract.empty_batch("125", "recorded-import-key"), %{
        "addresses" => [
          %{
            "hash" => to_string(address),
            "is_contract" => false,
            "is_verified_contract" => false,
            "contract_name" => nil
          }
        ],
        "block_ranges" => [%{"min_block_number" => to_string(number), "max_block_number" => to_string(number)}],
        "hashes" => [
          %{"hash" => to_string(block_hash), "hash_type" => "BLOCK"},
          %{"hash" => to_string(transaction_hash), "hash_type" => "TRANSACTION"}
        ]
      })

    Contract.replay_after_restart(fetcher, MainExportQueue, bypass, expected)
  end

  test "035: all token-info variants reach the receiver after interruption", %{bypass: bypass} do
    fetcher = Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue
    enable(fetcher)
    {:ok, address} = Chain.string_to_address_hash("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48")

    records = [
      {:metadata, "metadata", %{"token_type" => "ERC-20", "name" => "USD Coin", "symbol" => "USDC", "decimals" => 6}},
      {:total_supply, "metadata", %{"total_supply" => "123456789012345678901234567890"}},
      {:counters, "counters", %{"transfers_count" => "456", "holders_count" => "123"}},
      {:market_data, "price_data", %{"fiat_value" => "1.000001", "circulating_market_cap" => "1000.0001"}}
    ]

    for {type, _field, data} <- records do
      %TokenInfoExportQueue{}
      |> TokenInfoExportQueue.changeset(%{address_hash: address.bytes, data_type: type, data: data})
      |> Repo.insert!()
    end

    expected = %{
      "api_key" => "recorded-import-key",
      "chain_id" => "125",
      "tokens" =>
        Enum.map(records, fn {_type, field, data} ->
          %{"address_hash" => to_string(address), field => data}
        end)
    }

    Contract.replay_after_restart(fetcher, TokenInfoExportQueue, bypass, expected)
  end

  for {finding, fetcher} <- [
        {"036", Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue},
        {"037", Indexer.Fetcher.MultichainSearchDb.MainExportQueue},
        {"038", Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue}
      ] do
    test "#{finding}: exact configuration is restored after normal and interrupted exits" do
      supervisor = Module.concat(unquote(fetcher), Supervisor)
      Contract.preserve_config(supervisor)
      keys = [{:explorer, MultichainSearch}, {:tesla, :adapter}, {:indexer, supervisor}]

      for previous <- [
            :absent,
            nil,
            [
              service_url: "saved",
              api_key: "saved",
              addresses_chunk_size: 17,
              token_info_chunk_size: 19,
              disabled?: false,
              independent: :retained
            ]
          ],
          reason <- [:normal, :interrupted] do
        Enum.each(keys, fn {app, key} ->
          if previous == :absent, do: Application.delete_env(app, key), else: Application.put_env(app, key, previous)
        end)

        saved = Enum.map(keys, fn {app, key} -> Application.fetch_env(app, key) end)

        {pid, ref} =
          spawn_monitor(fn ->
            ExUnit.OnExitHandler.register(self())
            Contract.preserve_config(supervisor)
            Enum.each(keys, fn {app, key} -> Application.put_env(app, key, changed: true) end)
            exit(reason)
          end)

        assert_receive {:DOWN, ^ref, :process, ^pid, ^reason}, 5_000
        assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)
        assert Enum.map(keys, fn {app, key} -> Application.fetch_env(app, key) end) == saved
      end
    end
  end

  defp enable(fetcher) do
    supervisor = Module.concat(fetcher, Supervisor)
    Contract.preserve_config(supervisor)
    Application.put_env(:indexer, supervisor, disabled?: false)
  end
end
