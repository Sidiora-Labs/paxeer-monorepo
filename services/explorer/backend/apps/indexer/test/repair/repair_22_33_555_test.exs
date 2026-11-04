Code.require_file("../indexer/fetcher/repair_22_33_555_support.exs", __DIR__)
Code.require_file("../indexer/fetcher/token_test.exs", __DIR__)

defmodule Indexer.Repair2233555Test do
  use Explorer.DataCase, async: false

  alias Explorer.Chain.{Block, Cache.BlockNumber}
  alias Indexer.Fetcher.EmptyBlocksSanitizer
  alias Indexer.Fetcher.Repair2233555, as: Repair

  @moduletag capture_log: true

  test "009: waits for the real reward buffer and persisted reward" do
    Repair.assert_reward_completion()
  end

  test "009: timeout fails in the caller and leaves no polling child" do
    owner = self()
    before_links = Process.info(self(), :links)
    before_monitors = Process.info(self(), :monitors)
    Process.put(:poll_attempts, 0)

    assert_raise ExUnit.AssertionError, ~r/completion predicate did not succeed before timeout/, fn ->
      Repair.wait_until(20, fn ->
        assert self() == owner
        Process.put(:poll_attempts, Process.get(:poll_attempts) + 1)
        false
      end)
    end

    assert Process.get(:poll_attempts) > 1
    assert Process.info(self(), :links) == before_links
    assert Process.info(self(), :monitors) == before_monitors
    attempts = Process.get(:poll_attempts)
    Process.sleep(30)
    assert Process.get(:poll_attempts) == attempts
    refute_receive {_, :ok}
  end

  for result <- [:normal, :failure] do
    test "011: sanitizer setup restores its complete keyword list after #{result}" do
      assert_sanitizer_restoration(unquote(result))
    end
  end

  for variant <- [EthereumJSONRPC.Geth, EthereumJSONRPC.PaxeerX] do
    test "012: #{inspect(variant)} migrates the durable queue consumed by both tracing modes" do
      Repair.assert_trace_queue_selection(unquote(variant))
    end

    test "012: #{inspect(variant)} recovers a pending recorded Paxeer trace after restart" do
      Repair.assert_trace_recovery(unquote(variant))
    end
  end

  for http <- [EthereumJSONRPC.HTTP.HTTPoison, EthereumJSONRPC.HTTP.Tesla] do
    test "028: #{inspect(http)} catalogs the known contract through the real reader and token schema" do
      Repair.assert_catalog(unquote(http))
    end
  end

  test "017: withdrawal catchup persists requested blocks and resumes after restart" do
    Repair.assert_withdrawal_recovery()
  end

  defp assert_sanitizer_restoration(outcome) do
    original = [head_offset: 17, interval: 60_000, batch_size: 10, retained_option: [nested: :preserve_me]]
    Repair.put_env(:indexer, EmptyBlocksSanitizer, original)
    owner = self()

    {pid, monitor} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())
        :ok = Repair.configure_sanitizer()
        send(owner, {:configured, self()})

        receive do
          :finish ->
            case outcome do
              :normal -> :ok
              :failure -> flunk("deliberate failure after sanitizer import")
            end
        end
      end)

    on_exit(fn ->
      if Process.alive?(pid), do: Process.exit(pid, :kill)
      if :ets.member(ExUnit.Server, pid), do: ExUnit.OnExitHandler.run(pid, 5_000)
    end)

    assert_receive {:configured, ^pid}, 5_000
    assert Application.fetch_env!(:indexer, EmptyBlocksSanitizer) == Keyword.put(original, :head_offset, 1)

    [raw | _] = Repair.fixture("withdrawals")
    number = EthereumJSONRPC.quantity_to_integer(raw["number"])
    block = insert(:block, hash: raw["hash"], number: number, is_empty: nil, refetch_needed: false)
    insert(:block, number: number + 1, is_empty: true)
    BlockNumber.set_max(number + 1)
    quantity = EthereumJSONRPC.integer_to_quantity(number)

    arguments =
      Repair.serve(fn [%{"id" => id, "method" => "eth_getBlockByNumber", "params" => [^quantity, false]}] ->
        [%{jsonrpc: "2.0", id: id, result: raw}]
      end)

    state = %EmptyBlocksSanitizer{json_rpc_named_arguments: arguments}
    assert {:noreply, ^state} = EmptyBlocksSanitizer.handle_info(:sanitize_empty_blocks, state)
    assert Repo.get!(Block, block.hash).is_empty == true
    assert Repo.get!(Block, block.hash).refetch_needed == false
    assert_receive {:rpc_requests, [%{"method" => "eth_getBlockByNumber", "params" => [^quantity, false]}]}, 5_000

    send(pid, :finish)

    case outcome do
      :normal -> assert_receive {:DOWN, ^monitor, :process, ^pid, :normal}, 5_000
      :failure -> assert_receive {:DOWN, ^monitor, :process, ^pid, {%ExUnit.AssertionError{}, _}}, 5_000
    end

    assert :ok = ExUnit.OnExitHandler.run(pid, 5_000)
    assert Application.fetch_env!(:indexer, EmptyBlocksSanitizer) == original
  end
end
