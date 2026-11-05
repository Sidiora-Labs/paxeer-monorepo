defmodule BlockScoutWeb.Repair22_33_486Test do
  @moduledoc """
  Repair record for audit 22.30.150 finding 007:
  delayed_predecessor preserves sequence, missing_predecessor bounds worker lifetime,
  overtaken_predecessor reconciles an advanced head, and late_batch preserves the watermark.
  Each case decodes committed RPC recordings through the production block schema,
  publishes through Chain.Events and MainPage, and consumes the real V2 block channel.
  No out-of-path wiring is required. Baseline verification was not run because
  the task's bounded gate order requires complete implementation before verification.
  The verified revision and gate evidence are recorded by hf-done.
  """

  use BlockScoutWeb.ConnCase, async: false

  import Phoenix.ChannelTest

  alias BlockScoutWeb.RealtimeEventHandlers.MainPage
  alias BlockScoutWeb.V2.UserSocket
  alias EthereumJSONRPC.Block, as: RPCBlock
  alias Explorer.Chain.{Address, Block}
  alias Explorer.Chain.Cache.Counters.{AverageBlockTime, Helper}
  alias Explorer.Chain.Events.Publisher
  alias Explorer.Repo

  @consecutive Path.expand(
                 "../../../ethereum_jsonrpc/test/support/fixture/paxeer_x/consecutive_blocks.json",
                 __DIR__
               )
  @older Path.expand(
           "../../../explorer/test/support/fixture/vcr_cassettes/block_importer_download_block_1_downloads_the_block.json",
           __DIR__
         )

  setup do
    previous_config = Application.fetch_env!(:explorer, AverageBlockTime)
    Application.put_env(:explorer, AverageBlockTime, Keyword.put(previous_config, :enabled, true))
    start_supervised!(AverageBlockTime)
    assert %Timex.Duration{} = AverageBlockTime.average_block_time()

    main_page = Process.whereis(MainPage)
    assert is_pid(main_page)
    previous_head = :ets.lookup(:last_broadcasted_block, :number)

    on_exit(fn ->
      Application.put_env(:explorer, AverageBlockTime, previous_config)
      :erlang.trace(main_page, false, [:procs])
      :ets.delete(:last_broadcasted_block, :number)
      :ets.insert(:last_broadcasted_block, previous_head)
    end)

    assert {:ok, %{}, _socket} =
             UserSocket
             |> socket("repair-22-33-486", %{})
             |> subscribe_and_join("blocks:new_block")

    [predecessor, target] =
      @consecutive
      |> File.read!()
      |> Jason.decode!()
      |> Map.values()
      |> Enum.sort_by(&EthereumJSONRPC.quantity_to_integer(&1["number"]))
      |> Enum.map(&persist_recorded_block/1)

    assert target.number == predecessor.number + 1
    assert target.parent_hash == predecessor.hash

    {:ok, predecessor: predecessor, target: target, main_page: main_page}
  end

  test "007 delayed_predecessor: the real channel receives the predecessor before the waiting block", %{
    predecessor: predecessor,
    target: target,
    main_page: main_page
  } do
    set_head(predecessor.number - 1)
    {worker, monitor} = publish_waiting_block(target, main_page)
    refute_push("new_block", _, 100)

    publish([predecessor])
    assert_notification(predecessor)
    assert_notification(target)
    assert_receive {:DOWN, ^monitor, :process, ^worker, :normal}, 1_000
    assert head() == target.number
    refute_push("new_block", _, 100)
  end

  test "007 missing_predecessor: notification is delivered and the worker exits within the bounded wait", %{
    predecessor: predecessor,
    target: target,
    main_page: main_page
  } do
    set_head(predecessor.number - 1)
    started = System.monotonic_time(:millisecond)
    {worker, monitor} = publish_waiting_block(target, main_page)
    refute_push("new_block", _, 100)

    assert_notification(target)
    assert_receive {:DOWN, ^monitor, :process, ^worker, :normal}, 1_000
    assert System.monotonic_time(:millisecond) - started < 3_500
    assert head() == target.number
    refute_push("new_block", _, 100)
  end

  test "007 overtaken_predecessor: a newer broadcast releases the waiting worker without regressing the head", %{
    target: newer,
    main_page: main_page
  } do
    recording =
      @older
      |> File.read!()
      |> Jason.decode!()
      |> Enum.map(&Jason.decode!(&1["response"]["body"]))
      |> Enum.find(&(is_map(&1["result"]) and Map.has_key?(&1["result"], "parentHash")))

    older = persist_recorded_block(recording["result"])
    assert older.number < newer.number - 4
    set_head(older.number - 2)
    {worker, monitor} = publish_waiting_block(older, main_page)
    refute_push("new_block", _, 100)

    publish([newer])
    assert_notification(newer)
    assert head() == newer.number
    assert_notification(older)
    assert_receive {:DOWN, ^monitor, :process, ^worker, :normal}, 1_000
    assert head() == newer.number
    refute_push("new_block", _, 100)
  end

  test "007 late_batch: older blocks remain observable without lowering the broadcast watermark", %{
    predecessor: predecessor,
    target: target
  } do
    set_head(0)
    publish([target, predecessor])
    assert_notification(predecessor)
    assert_notification(target)
    assert head() == target.number

    publish([predecessor])
    assert_notification(predecessor)
    assert head() == target.number
  end

  defp persist_recorded_block(recording) do
    params = recording |> RPCBlock.to_elixir() |> RPCBlock.elixir_to_params()
    Repo.get_by(Address, hash: params.miner_hash) || insert(:address, hash: params.miner_hash)

    %Block{}
    |> Block.changeset(Map.put(params, :consensus, true))
    |> Repo.insert!()
  end

  defp publish(blocks) do
    Publisher.broadcast([{:blocks, blocks}], :realtime)
    :sys.get_state(MainPage)
  end

  defp publish_waiting_block(block, main_page) do
    :erlang.trace(main_page, true, [:procs])
    publish([block])
    assert_receive {:trace, ^main_page, :spawn, worker, _entry}, 1_000
    :erlang.trace(main_page, false, [:procs])
    {worker, Process.monitor(worker)}
  end

  defp assert_notification(block) do
    assert_push("new_block", %{block: received, average_block_time: average}, 3_500)
    assert received["height"] == block.number
    assert to_string(received["hash"]) == to_string(block.hash)
    assert to_string(received["parent_hash"]) == to_string(block.parent_hash)
    assert received["timestamp"] == block.timestamp
    assert received["gas_used"] == block.gas_used
    assert received["gas_limit"] == block.gas_limit
    assert is_binary(average)
  end

  defp set_head(number), do: :ets.insert(:last_broadcasted_block, {:number, number})
  defp head, do: Helper.fetch_from_ets_cache(:last_broadcasted_block, :number)
end
