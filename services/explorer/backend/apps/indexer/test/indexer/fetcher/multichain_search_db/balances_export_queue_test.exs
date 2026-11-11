Code.require_file("export_queue_contract.exs", __DIR__)

defmodule Indexer.Fetcher.MultichainSearchDb.BalancesExportQueueTest do
  use ExUnit.Case
  use Explorer.DataCase

  import ExUnit.CaptureLog, only: [capture_log: 1]

  alias Explorer.Chain
  alias Explorer.Chain.Wei
  alias Explorer.Chain.MultichainSearchDb.BalancesExportQueue
  alias Explorer.MicroserviceInterfaces.MultichainSearch
  alias Explorer.TestHelper

  alias Indexer.Fetcher.MultichainSearchDb.BalancesExportQueue,
    as: MultichainSearchDbExportBalancesExportQueue

  alias Indexer.Fetcher.MultichainSearchDb.ExportQueueContract

  @moduletag :capture_log

  setup do
    ExportQueueContract.preserve_config(MultichainSearchDbExportBalancesExportQueue.Supervisor)
    start_supervised!({Task.Supervisor, name: Indexer.TaskSupervisor})
    Application.put_env(:indexer, MultichainSearchDbExportBalancesExportQueue.Supervisor, disabled?: false)

    :ok
  end

  describe "init/3" do
    setup do
      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:1234",
        api_key: "12345",
        addresses_chunk_size: 7000
      )
    end

    test "initializes with data from the retry queue" do
      {:ok, address_hash} =
        Chain.string_to_address_hash("0x66A9B160F6a06f53f23785F069882Ee7337180E8")

      erc_20_contract_address_hash_bytes = "A0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48" |> Base.decode16!(case: :mixed)
      nft_contract_address_hash_bytes = "B01b6a7EF8560017AA59A990e39f26b8df29F80f" |> Base.decode16!(case: :mixed)

      insert(:multichain_search_db_export_balances_queue, %{
        address_hash: address_hash,
        token_contract_address_hash_or_native: "native",
        value: 100,
        token_id: nil
      })

      insert(:multichain_search_db_export_balances_queue, %{
        address_hash: address_hash,
        token_contract_address_hash_or_native: erc_20_contract_address_hash_bytes,
        value: 200,
        token_id: nil
      })

      insert(:multichain_search_db_export_balances_queue, %{
        address_hash: address_hash,
        token_contract_address_hash_or_native: nft_contract_address_hash_bytes,
        value: nil,
        token_id: 12345
      })

      reducer = fn data, acc -> [data | acc] end

      pid =
        []
        |> MultichainSearchDbExportBalancesExportQueue.Supervisor.child_spec()
        |> ExUnit.Callbacks.start_supervised!()

      results = MultichainSearchDbExportBalancesExportQueue.init([], reducer, nil)

      assert Enum.count(results) == 3

      assert Enum.member?(results, %{
               address_hash: address_hash,
               token_contract_address_hash_or_native: "native",
               value: %Wei{value: Decimal.new(100)},
               token_id: nil
             })

      assert Enum.member?(results, %{
               address_hash: address_hash,
               token_contract_address_hash_or_native: erc_20_contract_address_hash_bytes,
               value: %Wei{value: Decimal.new(200)},
               token_id: nil
             })

      assert Enum.member?(results, %{
               address_hash: address_hash,
               token_contract_address_hash_or_native: nft_contract_address_hash_bytes,
               value: nil,
               token_id: Decimal.new(12345)
             })

      :timer.sleep(10)
      GenServer.stop(pid)
    end
  end

  describe "run/2" do
    setup do
      bypass = Bypass.open()

      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:#{bypass.port}",
        api_key: "12345",
        addresses_chunk_size: 7000
      )

      on_exit(fn -> Bypass.down(bypass) end)

      {:ok, bypass: bypass}
    end

    test "running balances export retries HTTP 503 without a queue process restart", %{bypass: bypass} do
      assert_running_recovery(bypass, 503, ~s({"code":0,"message":"Unavailable"}))
    end

    test "running balances export retries malformed HTTP 200 without a queue process restart", %{bypass: bypass} do
      assert_running_recovery(bypass, 200, "<html>invalid successful response</html>")
    end

    test "successfully processes multichain search db export retry queue data", %{bypass: bypass} do
      {:ok, address_hash} =
        Chain.string_to_address_hash("0x66A9B160F6a06f53f23785F069882Ee7337180E8")

      {:ok, erc_20_contract_address_hash} =
        Chain.string_to_address_hash("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")

      {:ok, nft_contract_address_hash} =
        Chain.string_to_address_hash("0xB01b6a7EF8560017AA59A990e39f26b8df29F80f")

      export_data = [
        %{
          address_hash: address_hash,
          token_contract_address_hash_or_native: "native",
          value: %Wei{value: Decimal.new(100)}
        },
        %{
          address_hash: erc_20_contract_address_hash,
          token_contract_address_hash_or_native: erc_20_contract_address_hash.bytes,
          value: %Wei{value: Decimal.new(200)},
          token_id: nil
        },
        %{
          address_hash: nft_contract_address_hash,
          token_contract_address_hash_or_native: nft_contract_address_hash.bytes,
          value: nil,
          token_id: Decimal.new(12345)
        }
      ]

      TestHelper.get_chain_id_mock()

      Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

      expected = ExportQueueContract.empty_batch("1", "12345")

      expected =
        Map.merge(expected, %{
          "address_coin_balances" => [
            %{
              "address_hash" => to_string(address_hash),
              "token_contract_address_hash_or_native" => "native",
              "value" => "100"
            }
          ],
          "address_token_balances" => [
            %{
              "address_hash" => to_string(nft_contract_address_hash),
              "token_address_hash" => to_string(nft_contract_address_hash),
              "value" => nil,
              "token_id" => "12345"
            },
            %{
              "address_hash" => to_string(erc_20_contract_address_hash),
              "token_address_hash" => to_string(erc_20_contract_address_hash),
              "value" => "200",
              "token_id" => nil
            }
          ]
        })

      ExportQueueContract.expect_import(bypass, expected)

      assert :ok = MultichainSearchDbExportBalancesExportQueue.run(export_data, nil)
    end

    test "returns {:retry, failed_data} on error where failed_data is only chunks that failed to export", %{
      bypass: _bypass
    } do
      Application.put_env(:explorer, MultichainSearch,
        service_url: "http://localhost:1234",
        api_key: "12345",
        addresses_chunk_size: 1
      )

      address_1 = insert(:address)
      address_1_hash = address_1.hash
      address_1_hash_string = to_string(address_1_hash)
      address_2 = insert(:address)
      address_2_hash = address_2.hash
      address_2_hash_string = to_string(address_2_hash)
      address_3 = insert(:address)
      address_3_hash = address_3.hash
      address_3_hash_string = to_string(address_3_hash)
      address_4 = insert(:address)
      address_5 = insert(:address)
      address_5_hash = address_5.hash
      address_5_hash_string = to_string(address_5_hash)
      token_address_1 = insert(:address)
      token_address_1_hash_string = to_string(token_address_1) |> String.downcase()
      token_address_2 = insert(:address)
      token_address_2_hash_string = to_string(token_address_2) |> String.downcase()

      export_data = [
        %{
          address_hash: address_1.hash,
          token_contract_address_hash_or_native: "native",
          value: %Wei{value: Decimal.new(100)}
        },
        %{
          address_hash: address_2.hash,
          token_contract_address_hash_or_native: token_address_1.hash.bytes,
          value: %Wei{value: Decimal.new(200)},
          token_id: nil
        },
        %{
          address_hash: address_3.hash,
          token_contract_address_hash_or_native: token_address_2.hash.bytes,
          value: %Wei{value: Decimal.new(300)},
          token_id: nil
        },
        %{
          address_hash: address_4.hash,
          token_contract_address_hash_or_native: "native",
          value: %Wei{value: Decimal.new(400)}
        },
        %{
          address_hash: address_5.hash,
          token_contract_address_hash_or_native: token_address_1.hash.bytes,
          value: %Wei{value: Decimal.new(500)},
          token_id: nil
        }
      ]

      TestHelper.get_chain_id_mock()

      tesla_expectations(address_1_hash_string)

      val0 = Decimal.new(100) |> Wei.cast() |> elem(1)
      val1 = Decimal.new(200) |> Wei.cast() |> elem(1)
      val2 = Decimal.new(300) |> Wei.cast() |> elem(1)
      val4 = Decimal.new(500) |> Wei.cast() |> elem(1)

      log =
        capture_log(fn ->
          assert {:retry, retry_entries} = MultichainSearchDbExportBalancesExportQueue.run(export_data, nil)
          assert retry_entries == Enum.reject(export_data, &(&1.address_hash == address_4.hash))

          retry_chunks =
            retry_entries
            |> MultichainSearchDbExportBalancesExportQueue.prepare_export_data()
            |> MultichainSearch.extract_batch_import_params_into_chunks()

          expected_retry_payload = %{
            address_coin_balances: [
              %{
                address_hash: address_1_hash_string,
                token_contract_address_hash_or_native: "native",
                value: val0
              }
            ],
            address_token_balances: [
              %{
                address_hash: address_5_hash_string,
                token_address_hash: token_address_1_hash_string,
                value: val4,
                token_id: nil
              },
              %{
                address_hash: address_3_hash_string,
                token_address_hash: token_address_2_hash_string,
                value: val2,
                token_id: nil
              },
              %{
                address_hash: address_2_hash_string,
                token_address_hash: token_address_1_hash_string,
                value: val1,
                token_id: nil
              }
            ]
          }

          for {key, expected} <- expected_retry_payload do
            assert Enum.sort(Enum.flat_map(retry_chunks, & &1[key])) == Enum.sort(expected)
          end
        end)

      assert Repo.aggregate(BalancesExportQueue, :count, :id) == 4
      results = Repo.all(BalancesExportQueue)
      assert Enum.all?(results, &(&1.retries_number == nil))
      assert log =~ "Batch balances export retry to the Multichain Search DB failed"

      TestHelper.get_chain_id_mock()

      tesla_expectations(address_1_hash_string)

      MultichainSearchDbExportBalancesExportQueue.run(export_data, nil)

      assert Repo.aggregate(BalancesExportQueue, :count, :id) == 4
      results = Repo.all(BalancesExportQueue)
      assert Enum.all?(results, &(&1.retries_number == 1))

      # Check, that `retries_number` is incrementing

      TestHelper.get_chain_id_mock()

      tesla_expectations(address_1_hash_string)

      MultichainSearchDbExportBalancesExportQueue.run(export_data, nil)

      assert Repo.aggregate(BalancesExportQueue, :count, :id) == 4
      results = Repo.all(BalancesExportQueue)
      assert Enum.all?(results, &(&1.retries_number == 2))

      export_data_2 = [
        %{
          address_hash: address_2.hash,
          token_contract_address_hash_or_native: token_address_1.hash.bytes,
          value: %Wei{value: Decimal.new(200)},
          token_id: nil
        },
        %{
          address_hash: address_3.hash,
          token_contract_address_hash_or_native: token_address_2.hash.bytes,
          value: %Wei{value: Decimal.new(300)},
          token_id: nil
        },
        %{
          address_hash: address_1.hash,
          token_contract_address_hash_or_native: "native",
          value: %Wei{value: Decimal.new(100)}
        },
        %{
          address_hash: address_5.hash,
          token_contract_address_hash_or_native: token_address_1.hash.bytes,
          value: %Wei{value: Decimal.new(500)},
          token_id: nil
        }
      ]

      Tesla.Test.expect_tesla_call(
        times: 1,
        returns: fn %{url: "http://localhost:1234/api/v1/import:batch", body: body}, _opts ->
          case Jason.decode(body) do
            _ ->
              {:ok, %Tesla.Env{status: 200, body: Jason.encode!(%{"status" => "ok"})}}
          end
        end
      )

      MultichainSearchDbExportBalancesExportQueue.run(export_data_2, nil)

      assert Repo.aggregate(BalancesExportQueue, :count, :id) == 0
    end
  end

  defp assert_running_recovery(bypass, failure_status, failure_body) do
    alias Explorer.Chain.Cache.ChainId
    previous_chain_id = ConCache.get(ChainId.cache_name(), :id)

    on_exit(fn ->
      ChainId.set_id(previous_chain_id)
      assert ConCache.get(ChainId.cache_name(), :id) == previous_chain_id
    end)

    ChainId.set_id(125)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    address = insert(:address)
    token = insert(:address)
    native_value = Decimal.new("123456789012345678901234567890123456789")
    token_value = Decimal.new("987654321098765432109876543210987654321")
    token_id = Decimal.new("123456789012345678901234567890")

    entries = [
      %{
        address_hash: address.hash,
        token_contract_address_hash_or_native: "native",
        value: %Wei{value: native_value},
        token_id: nil
      },
      %{
        address_hash: address.hash,
        token_contract_address_hash_or_native: token.hash.bytes,
        value: %Wei{value: token_value},
        token_id: nil
      },
      %{
        address_hash: address.hash,
        token_contract_address_hash_or_native: token.hash.bytes,
        value: nil,
        token_id: token_id
      }
    ]

    rows = Enum.map(entries, &insert(:multichain_search_db_export_balances_queue, &1))

    expected =
      Map.merge(ExportQueueContract.empty_batch("125", "12345"), %{
        "address_coin_balances" => [
          %{
            "address_hash" => to_string(address.hash),
            "token_contract_address_hash_or_native" => "native",
            "value" => Decimal.to_string(native_value, :normal)
          }
        ],
        "address_token_balances" => [
          %{
            "address_hash" => to_string(address.hash),
            "token_address_hash" => to_string(token.hash),
            "value" => Decimal.to_string(token_value, :normal),
            "token_id" => nil
          },
          %{
            "address_hash" => to_string(address.hash),
            "token_address_hash" => to_string(token.hash),
            "value" => nil,
            "token_id" => Decimal.to_string(token_id, :normal)
          }
        ]
      })

    owner = self()

    Bypass.expect(bypass, "POST", "/api/v1/import:batch", fn conn ->
      assert Plug.Conn.get_req_header(conn, "content-type") == ["application/json"]
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      payload = Jason.decode!(body)
      send(owner, {:export_waiting, self(), payload})

      receive do
        {:respond, status, response} -> Plug.Conn.resp(conn, status, response)
      after
        5_000 -> flunk("Timed out waiting for upstream recovery response")
      end
    end)

    supervisor = MultichainSearchDbExportBalancesExportQueue.Supervisor

    supervisor_pid =
      start_supervised!({supervisor, [[poll: false, flush_interval: 1_000, max_concurrency: 1, max_batch_size: 100]]})

    queue = Process.whereis(MultichainSearchDbExportBalancesExportQueue)
    assert is_pid(queue)
    monitor = Process.monitor(queue)
    assert_receive {:export_waiting, first_request, first_payload}, 5_000
    assert canonical_batch(first_payload) == canonical_batch(expected)
    first_state = GenServer.call(queue, :state)
    assert first_state.max_concurrency == 1
    [first_ref] = Map.keys(first_state.task_ref_to_batch)
    assert Enum.sort(first_state.task_ref_to_batch[first_ref]) == Enum.sort(entries)
    send(first_request, {:respond, failure_status, failure_body})

    ExportQueueContract.eventually(fn ->
      state = GenServer.call(queue, :state)
      map_size(state.task_ref_to_batch) == 0 and state.bound_queue.size == length(entries)
    end)

    state = GenServer.call(queue, :state)
    assert Enum.sort(Enum.to_list(state.bound_queue)) == Enum.sort(entries)
    persisted = Repo.all(BalancesExportQueue)
    fields = Map.keys(hd(entries))
    assert Enum.sort(Enum.map(persisted, &Map.take(&1, fields))) == Enum.sort(entries)
    assert Enum.all?(persisted, &(&1.retries_number == 1))

    assert Enum.sort(Enum.map(persisted, &Map.drop(Map.from_struct(&1), [:id, :updated_at, :retries_number]))) ==
             Enum.sort(Enum.map(rows, &Map.drop(Map.from_struct(&1), [:id, :updated_at, :retries_number])))

    assert Process.whereis(MultichainSearchDbExportBalancesExportQueue) == queue
    refute_receive {:DOWN, ^monitor, :process, ^queue, _}, 0

    assert_receive {:export_waiting, retry_request, retry_payload}, 5_000
    assert canonical_batch(retry_payload) == canonical_batch(expected)
    retry_state = GenServer.call(queue, :state)
    refute Map.has_key?(retry_state.task_ref_to_batch, first_ref)
    [retry_entries] = Map.values(retry_state.task_ref_to_batch)
    assert Enum.sort(retry_entries) == Enum.sort(entries)
    assert Repo.all(BalancesExportQueue) == persisted
    assert Process.whereis(MultichainSearchDbExportBalancesExportQueue) == queue
    send(retry_request, {:respond, 200, ~s({"status":"ok"})})

    ExportQueueContract.eventually(fn ->
      Repo.aggregate(BalancesExportQueue, :count) == 0 and
        map_size(GenServer.call(queue, :state).task_ref_to_batch) == 0
    end)

    assert Process.alive?(queue)
    assert Process.whereis(MultichainSearchDbExportBalancesExportQueue) == queue

    assert {MultichainSearchDbExportBalancesExportQueue, ^queue, :worker, _} =
             Enum.find(Supervisor.which_children(supervisor_pid), fn {id, _, _, _} ->
               id == MultichainSearchDbExportBalancesExportQueue
             end)

    refute_receive {:DOWN, ^monitor, :process, ^queue, _}, 0
    assert :ok = stop_supervised(supervisor)
    Process.demonitor(monitor, [:flush])
  end

  defp canonical_batch(batch) do
    Map.new(batch, fn
      {key, records} when is_list(records) -> {key, Enum.sort(records)}
      entry -> entry
    end)
  end

  defp tesla_expectations(address_hash_string) do
    Tesla.Test.expect_tesla_call(
      times: 2,
      returns: fn %{url: "http://localhost:1234/api/v1/import:batch", body: body}, _opts ->
        case Jason.decode(body) do
          {:ok, %{"address_coin_balances" => [%{"address_hash" => ^address_hash_string}]}} ->
            {:ok, %Tesla.Env{status: 500, body: Jason.encode!(%{"code" => 0, "message" => "Error"})}}

          _ ->
            {:ok, %Tesla.Env{status: 200, body: Jason.encode!(%{"status" => "ok"})}}
        end
      end
    )
  end
end
