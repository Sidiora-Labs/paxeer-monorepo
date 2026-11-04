Code.require_file("../indexer/fetcher/on_demand/test_environment.exs", __DIR__)

defmodule Indexer.Repair2233558Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain
  alias Explorer.Chain.Address
  alias Explorer.Chain.Cache.Counters.AverageBlockTime
  alias Explorer.Chain.Events.Subscriber
  alias Explorer.Chain.Token.Instance
  alias Explorer.Utility.{AddressContractCodeFetchAttempt, TokenInstanceMetadataRefetchAttempt}
  alias Indexer.Fetcher.OnDemand.{ContractCode, TestEnvironment, TokenInstanceMetadataRefetch}

  @moduletag :capture_log
  @timeout 5_000
  @address "0x3078000000000000000000000000000000000001"
  @token_address "0x3078000000000000000000000000000000000002"
  @metadata %{"name" => "Super Token"}
  @transcript Path.expand("../indexer/fetcher/on_demand/repair_22_33_558_transcript.json", __DIR__)
  @blocks Path.expand("../../../explorer/test/support/fixture/vcr_cassettes", __DIR__)

  defmodule TranscriptServer do
    @behaviour Plug
    import Plug.Conn

    @impl true
    def init(options), do: options

    @impl true
    def call(conn, options) do
      {:ok, body, conn} = read_body(conn)
      requests = Jason.decode!(body)

      responses =
        Enum.map(requests, fn %{"id" => id, "jsonrpc" => "2.0", "method" => method, "params" => params} = request ->
          entry =
            Enum.find(options[:transcript], fn entry ->
              entry["request"] == %{"method" => method, "params" => params}
            end)

          send(options[:owner], {:rpc_request, request})
          Map.put(Map.fetch!(entry, "response"), "id", id)
        end)

      conn
      |> put_resp_content_type("application/json")
      |> send_resp(200, Jason.encode!(responses))
    end
  end

  setup do
    transcript = @transcript |> File.read!() |> Jason.decode!()
    ref = __MODULE__.HTTP

    start_supervised!(
      Plug.Cowboy.child_spec(
        scheme: :http,
        plug: {TranscriptServer, [owner: self(), transcript: transcript]},
        ref: ref,
        options: [ip: {127, 0, 0, 1}, port: 0]
      )
    )

    url = "http://127.0.0.1:#{:ranch.get_port(ref)}"

    rpc = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        http_options: [recv_timeout: @timeout, timeout: @timeout],
        urls: [url],
        fallback_urls: []
      ]
    ]

    TestEnvironment.preserve(:explorer, :json_rpc_named_arguments)
    Application.put_env(:explorer, :json_rpc_named_arguments, rpc)
    %{rpc: rpc}
  end

  for finding <- ["031", "032"] do
    test "#{finding}: complete AverageBlockTime settings survive an on-demand test exit" do
      TestEnvironment.preserve(:explorer, AverageBlockTime)
      configuration = [enabled: true, cache_period: 91_000, period: 93_000, num_of_blocks: 2]
      Application.put_env(:explorer, AverageBlockTime, configuration)

      assert :normal ==
               isolated_test(fn ->
                 TestEnvironment.preserve(:explorer, AverageBlockTime)
                 Application.put_env(:explorer, AverageBlockTime, enabled: true, cache_period: 1_800_000)

                 assert Application.fetch_env!(:explorer, AverageBlockTime) ==
                          [enabled: true, cache_period: 1_800_000]
               end)

      assert Application.fetch_env!(:explorer, AverageBlockTime) == configuration
      assert_recorded_block_average()
      assert Application.fetch_env!(:explorer, AverageBlockTime) == configuration
    end

    test "#{finding}: an originally absent AverageBlockTime setting stays absent" do
      TestEnvironment.preserve(:explorer, AverageBlockTime)
      Application.delete_env(:explorer, AverageBlockTime)

      assert :normal ==
               isolated_test(fn ->
                 TestEnvironment.preserve(:explorer, AverageBlockTime)
                 Application.put_env(:explorer, AverageBlockTime, enabled: true, cache_period: 1_800_000)
               end)

      assert Application.fetch_env(:explorer, AverageBlockTime) == :error
    end
  end

  test "054: a failed bytecode assertion restores the whole configuration before the next fetch", %{rpc: rpc} do
    TestEnvironment.preserve(:indexer, ContractCode)
    configuration = [threshold: 93_000, disabled?: false, retained_option: :original]
    Application.put_env(:indexer, ContractCode, configuration)
    start_supervised!({ContractCode, [rpc, [name: ContractCode]]})
    address = insert(:address, hash: @address)

    reason =
      isolated_test(fn ->
        TestEnvironment.preserve(:indexer, ContractCode)
        Application.put_env(:indexer, ContractCode, threshold: 0)
        assert :error = ContractCode.get_or_fetch_bytecode(address.hash)
        assert Repo.get!(AddressContractCodeFetchAttempt, address.hash).retries_number == 1
        raise ExUnit.AssertionError, message: "early assertion failure after a real bytecode fetch"
      end)

    assert {%ExUnit.AssertionError{message: "early assertion failure after a real bytecode fetch"}, _} = reason
    assert Application.fetch_env!(:indexer, ContractCode) == configuration
    assert :error = ContractCode.get_or_fetch_bytecode(address.hash)
    assert Repo.get!(AddressContractCodeFetchAttempt, address.hash).retries_number == 1
    assert_receive {:rpc_request, %{"method" => "eth_getCode", "params" => [@address, "latest"]}}, @timeout
    refute_receive {:rpc_request, %{"method" => "eth_getCode"}}
    assert is_nil(Repo.get!(Address, address.hash).contract_code)
  end

  test "054: absent and explicitly nil ContractCode settings are restored even after an early exit" do
    TestEnvironment.preserve(:indexer, ContractCode)

    for previous <- [:error, {:ok, nil}] do
      case previous do
        :error -> Application.delete_env(:indexer, ContractCode)
        {:ok, value} -> Application.put_env(:indexer, ContractCode, value)
      end

      assert :shutdown ==
               isolated_test(fn ->
                 TestEnvironment.preserve(:indexer, ContractCode)
                 Application.put_env(:indexer, ContractCode, threshold: 1)
                 exit(:shutdown)
               end)

      assert Application.fetch_env(:indexer, ContractCode) == previous
    end
  end

  test "014: raw success metadata and a failing refresh reach real subscribers without a false success", %{rpc: rpc} do
    start_supervised!({TokenInstanceMetadataRefetch, [rpc, [name: TokenInstanceMetadataRefetch]]})
    Subscriber.to(:fetched_token_instance_metadata, :on_demand)
    Subscriber.to(:not_fetched_token_instance_metadata, :on_demand)
    contract_address = insert(:address, hash: @token_address)

    token =
      insert(:token,
        contract_address: contract_address,
        contract_address_hash: contract_address.hash,
        type: "ERC-721"
      )

    success =
      insert(:token_instance, token_id: 1, token_contract_address_hash: token.contract_address_hash, metadata: %{})
      |> Repo.preload(:token)

    assert :ok = TokenInstanceMetadataRefetch.trigger_refetch(success)

    assert_receive {:chain_event, :fetched_token_instance_metadata, :on_demand, [@token_address, 1, @metadata]},
                   @timeout

    assert Repo.get_by!(Instance, token_contract_address_hash: token.contract_address_hash, token_id: 1).metadata ==
             @metadata

    refute Repo.get_by(TokenInstanceMetadataRefetchAttempt,
             token_contract_address_hash: token.contract_address_hash,
             token_id: 1
           )

    failed =
      insert(:token_instance,
        token_id: 2,
        token_contract_address_hash: token.contract_address_hash,
        metadata: @metadata
      )
      |> Repo.preload(:token)

    assert :ok = TokenInstanceMetadataRefetch.trigger_refetch(failed)

    assert_receive {:chain_event, :not_fetched_token_instance_metadata, :on_demand, [@token_address, 2, "error"]},
                   @timeout

    :sys.get_state(TokenInstanceMetadataRefetch)

    assert Repo.get_by!(TokenInstanceMetadataRefetchAttempt,
             token_contract_address_hash: token.contract_address_hash,
             token_id: 2
           ).retries_number == 1

    assert Repo.get_by!(Instance, token_contract_address_hash: token.contract_address_hash, token_id: 2).metadata ==
             @metadata

    refute_receive {:chain_event, :fetched_token_instance_metadata, :on_demand, [@token_address, 2, _raw_metadata]}

    for token_id <- [1, 2] do
      data = "0xc87b56dd" <> String.pad_leading(Integer.to_string(token_id, 16), 64, "0")

      assert_receive {:rpc_request,
                      %{"method" => "eth_call", "params" => [%{"to" => @token_address, "data" => ^data}, "latest"]}},
                     @timeout
    end
  end

  defp isolated_test(fun) do
    {pid, monitor} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())
        fun.()
      end)

    assert_receive {:DOWN, ^monitor, :process, ^pid, reason}, @timeout
    assert :ok = ExUnit.OnExitHandler.run(pid, @timeout)
    reason
  end

  defp assert_recorded_block_average do
    raw_blocks =
      Enum.map(
        ["block_importer_download_block_1_downloads_the_block.json", "block_importer_import_1_pending.json"],
        fn name ->
          [record] = @blocks |> Path.join(name) |> File.read!() |> Jason.decode!()
          record["response"]["body"] |> Jason.decode!() |> Map.fetch!("result")
        end
      )

    blocks =
      Enum.map(raw_blocks, fn raw ->
        raw
        |> EthereumJSONRPC.Block.to_elixir()
        |> EthereumJSONRPC.Block.elixir_to_params()
        |> Map.put(:consensus, true)
      end)

    addresses = blocks |> Enum.map(&%{hash: &1.miner_hash}) |> Enum.uniq()

    assert {:ok, %{blocks: imported}} =
             Chain.import(%{addresses: %{params: addresses}, blocks: %{params: blocks}, broadcast: false})

    assert length(imported) == 2
    last_number = blocks |> Enum.map(& &1.number) |> Enum.max()

    for number <- (last_number + 1)..(last_number + 100) do
      insert(:block, number: number)
    end

    start_supervised!(AverageBlockTime)
    assert :ok = AverageBlockTime.refresh()
    [first, second] = Enum.sort_by(blocks, & &1.number)

    expected_ms =
      round(DateTime.diff(second.timestamp, first.timestamp, :millisecond) / (second.number - first.number))

    assert expected_ms > 0
    assert AverageBlockTime.average_block_time() == Timex.Duration.from_milliseconds(expected_ms)
  end
end
