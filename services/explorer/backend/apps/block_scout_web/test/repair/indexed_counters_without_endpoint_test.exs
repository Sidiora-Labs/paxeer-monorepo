defmodule BlockScoutWeb.IndexedCountersWithoutEndpointTest do
  use BlockScoutWeb.ConnCase, async: false

  import ExUnit.CaptureLog

  alias BlockScoutWeb.Counters.{BlocksIndexedCounter, InternalTransactionsIndexedCounter}
  alias BlockScoutWeb.Endpoint
  alias Explorer.Chain
  alias Phoenix.Socket.Broadcast

  setup tags do
    assert is_pid(Process.whereis(BlockScoutWeb.PubSub))

    if tags[:publisher_absent] do
      publisher_id = Supervisor.child_spec({Phoenix.PubSub, name: BlockScoutWeb.PubSub}, []).id
      assert :ok = Supervisor.terminate_child(BlockScoutWeb.Supervisor, publisher_id)

      on_exit(fn ->
        assert {:ok, _pid} = Supervisor.restart_child(BlockScoutWeb.Supervisor, publisher_id)
      end)

      assert Process.whereis(BlockScoutWeb.PubSub) == nil
      assert is_pid(Process.whereis(Endpoint))
    end

    :ok
  end

  @tag :publisher_absent
  test "blocks counter completes quietly without the endpoint publisher" do
    assert_quiet_completion(&BlocksIndexedCounter.calculate_blocks_indexed_and_broadcast/0)
  end

  @tag :publisher_absent
  test "internal transactions counter completes quietly without the endpoint publisher" do
    assert_quiet_completion(&InternalTransactionsIndexedCounter.calculate_internal_transactions_indexed/0)
  end

  test "blocks counter publishes its ratio on both indexing topics" do
    assert_ratio_delivery(
      &BlocksIndexedCounter.calculate_blocks_indexed_and_broadcast/0,
      Chain.indexed_ratio_blocks(),
      ["blocks_old:indexing", "blocks:indexing"]
    )
  end

  test "internal transactions counter publishes its ratio on both indexing topics" do
    assert_ratio_delivery(
      &InternalTransactionsIndexedCounter.calculate_internal_transactions_indexed/0,
      Chain.indexed_ratio_internal_transactions(),
      ["blocks_old:indexing_internal_transactions", "blocks:indexing_internal_transactions"]
    )
  end

  defp assert_quiet_completion(calculate) do
    log =
      capture_log([level: :error], fn ->
        task = Task.async(calculate)
        assert Task.await(task) == :ok
      end)

    assert log == ""
    assert Process.whereis(BlockScoutWeb.PubSub) == nil
  end

  defp assert_ratio_delivery(calculate, ratio, topics) do
    for topic <- topics do
      assert :ok = Endpoint.subscribe(topic)
    end

    assert calculate.() == :ok
    payload = %{ratio: Decimal.to_string(ratio), finished: Chain.finished_indexing_from_ratio?(ratio)}

    for topic <- topics do
      assert_receive %Broadcast{topic: ^topic, event: "index_status", payload: ^payload}
    end
  end
end
