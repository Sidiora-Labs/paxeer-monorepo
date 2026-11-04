Code.require_file("export_queue_contract.exs", __DIR__)

defmodule Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueueTest do
  use ExUnit.Case
  use Explorer.DataCase

  import ExUnit.CaptureLog, only: [capture_log: 1]

  alias Explorer.Chain.MultichainSearchDb.TokenInfoExportQueue
  alias Explorer.MicroserviceInterfaces.MultichainSearch
  alias Explorer.TestHelper

  alias Indexer.Fetcher.MultichainSearchDb.TokenInfoExportQueue,
    as: MultichainSearchDbTokenInfoExportQueue

  alias Indexer.Fetcher.MultichainSearchDb.ExportQueueContract

  @moduletag :capture_log

  setup do
    ExportQueueContract.preserve_config(MultichainSearchDbTokenInfoExportQueue.Supervisor)
    start_supervised!({Task.Supervisor, name: Indexer.TaskSupervisor})
    Application.put_env(:indexer, MultichainSearchDbTokenInfoExportQueue.Supervisor, disabled?: false)

    :ok
  end

  describe "init/3" do
    setup do
      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:1234",
        api_key: "12345",
        token_info_chunk_size: 1000
      )
    end

    test "initializes with data from the retry queue" do
      token_info_item_1 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_2 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_3 = insert(:multichain_search_db_export_token_info_queue)

      {:ok, token_info_item_1_data} = Jason.decode(Jason.encode!(token_info_item_1.data))
      {:ok, token_info_item_2_data} = Jason.decode(Jason.encode!(token_info_item_2.data))
      {:ok, token_info_item_3_data} = Jason.decode(Jason.encode!(token_info_item_3.data))

      reducer = fn data, acc -> [data | acc] end

      pid =
        []
        |> MultichainSearchDbTokenInfoExportQueue.Supervisor.child_spec()
        |> ExUnit.Callbacks.start_supervised!()

      results = MultichainSearchDbTokenInfoExportQueue.init([], reducer, nil)

      assert Enum.count(results) == 3

      assert Enum.member?(results, %{
               address_hash: token_info_item_1.address_hash,
               data_type: token_info_item_1.data_type,
               data: token_info_item_1_data
             })

      assert Enum.member?(results, %{
               address_hash: token_info_item_2.address_hash,
               data_type: token_info_item_2.data_type,
               data: token_info_item_2_data
             })

      assert Enum.member?(results, %{
               address_hash: token_info_item_3.address_hash,
               data_type: token_info_item_3.data_type,
               data: token_info_item_3_data
             })

      :timer.sleep(100)
      GenServer.stop(pid)
    end
  end

  describe "run/2" do
    setup do
      bypass = Bypass.open()

      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:#{bypass.port}",
        api_key: "12345",
        token_info_chunk_size: 1000
      )

      on_exit(fn -> Bypass.down(bypass) end)

      {:ok, bypass: bypass}
    end

    test "successfully processes multichain search db export retry queue data", %{bypass: bypass} do
      export_data = [
        insert(:multichain_search_db_export_token_info_queue),
        insert(:multichain_search_db_export_token_info_queue),
        insert(:multichain_search_db_export_token_info_queue)
      ]

      TestHelper.get_chain_id_mock()

      Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

      expected = %{
        "api_key" => "12345",
        "chain_id" => "1",
        "tokens" =>
          Enum.map(export_data, fn item ->
            key =
              case item.data_type do
                :metadata -> "metadata"
                :total_supply -> "metadata"
                :counters -> "counters"
                :market_data -> "price_data"
              end

            %{
              "address_hash" => "0x" <> Base.encode16(item.address_hash, case: :lower),
              key => Jason.decode!(Jason.encode!(item.data))
            }
          end)
      }

      ExportQueueContract.expect_import(bypass, expected)

      assert :ok = MultichainSearchDbTokenInfoExportQueue.run(export_data, nil)
    end

    test "returns {:retry, failed_data} on error where failed_data is only chunks that failed to export", %{
      bypass: _bypass
    } do
      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:1234",
        api_key: "12345",
        token_info_chunk_size: 1
      )

      token_info_item_1 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_2 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_3 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_4 = insert(:multichain_search_db_export_token_info_queue)
      token_info_item_4_address_hash_string = "0x" <> Base.encode16(token_info_item_4.address_hash, case: :lower)
      token_info_item_5 = insert(:multichain_search_db_export_token_info_queue)

      export_data = [
        token_info_item_1,
        token_info_item_2,
        token_info_item_3,
        token_info_item_4,
        token_info_item_5
      ]

      TestHelper.get_chain_id_mock()

      tesla_expectations(token_info_item_4_address_hash_string)

      log =
        capture_log(fn ->
          assert {:retry, [item_to_retry]} = MultichainSearchDbTokenInfoExportQueue.run(export_data, nil)

          assert item_to_retry.address_hash == token_info_item_4.address_hash and
                   item_to_retry.data_type == token_info_item_4.data_type and
                   item_to_retry.data == token_info_item_4.data
        end)

      assert Repo.aggregate(TokenInfoExportQueue, :count) == 1
      results = Repo.all(TokenInfoExportQueue)
      assert Enum.all?(results, &(&1.retries_number == 1))
      assert log =~ "Batch token info export attempt to the Multichain Search DB failed"

      # Check that `retries_number` is incrementing

      TestHelper.get_chain_id_mock()

      tesla_expectations(token_info_item_4_address_hash_string)

      MultichainSearchDbTokenInfoExportQueue.run(export_data, nil)

      assert Repo.aggregate(TokenInfoExportQueue, :count) == 1
      results = Repo.all(TokenInfoExportQueue)
      assert Enum.all?(results, &(&1.retries_number == 2))

      export_data_2 = [
        token_info_item_2,
        token_info_item_3,
        token_info_item_4,
        token_info_item_5
      ]

      Tesla.Test.expect_tesla_call(
        times: 4,
        returns: fn %{url: "http://localhost:1234/api/v1/import:batch", body: body}, _opts ->
          case Jason.decode(body) do
            _ ->
              {:ok, %Tesla.Env{status: 200, body: Jason.encode!(%{"status" => "ok"})}}
          end
        end
      )

      MultichainSearchDbTokenInfoExportQueue.run(export_data_2, nil)

      assert Repo.aggregate(TokenInfoExportQueue, :count) == 0
    end
  end

  defp tesla_expectations(address_4_hash_string) do
    Tesla.Test.expect_tesla_call(
      times: 5,
      returns: fn %{url: "http://localhost:1234/api/v1/import:batch", body: body}, _opts ->
        case Jason.decode(body) do
          {:ok, %{"tokens" => [%{"address_hash" => ^address_4_hash_string}]}} ->
            {:ok, %Tesla.Env{status: 500, body: Jason.encode!(%{"code" => 0, "message" => "Error"})}}

          _ ->
            {:ok, %Tesla.Env{status: 200, body: Jason.encode!(%{"status" => "ok"})}}
        end
      end
    )
  end
end
