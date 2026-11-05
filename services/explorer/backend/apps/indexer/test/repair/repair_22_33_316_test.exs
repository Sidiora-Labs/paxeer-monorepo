defmodule Indexer.Repair2233316Test do
  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.HTTP
  alias Explorer.Chain.{Block, PendingBlockOperation}
  alias Explorer.Chain.Cache.BlockNumber
  alias Indexer.Fetcher.EmptyBlocksSanitizer

  @fixture Path.expand("../../lib/indexer/fetcher/repair_22_33_316_blocks.json", __DIR__)
  @moduletag capture_log: true

  setup do
    previous = Application.get_env(:indexer, EmptyBlocksSanitizer)
    Application.put_env(:indexer, EmptyBlocksSanitizer, head_offset: 0, batch_size: 10, interval: 60_000)

    on_exit(fn ->
      if is_nil(previous) do
        Application.delete_env(:indexer, EmptyBlocksSanitizer)
      else
        Application.put_env(:indexer, EmptyBlocksSanitizer, previous)
      end
    end)

    blocks = @fixture |> File.read!() |> Jason.decode!()

    rows =
      Enum.map(blocks, fn block ->
        row =
          insert(:block,
            hash: block["hash"],
            number: EthereumJSONRPC.quantity_to_integer(block["number"]),
            parent_hash: block["parentHash"],
            is_empty: nil,
            refetch_needed: false
          )

        insert(:pending_block_operation, block_hash: row.hash, block_number: row.number)
        row
      end)

    BlockNumber.set_max(Enum.max(Enum.map(rows, & &1.number)))
    responses = start_supervised!({Agent, fn -> {:initial, %{}} end})
    server = Bypass.open()
    owner = self()
    by_number = Map.new(blocks, &{&1["number"], &1})

    Bypass.expect(server, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      {phase, outcomes} = Agent.get(responses, & &1)

      result =
        requests
        |> Enum.reverse()
        |> Enum.flat_map(fn request ->
          assert %{
                   "jsonrpc" => "2.0",
                   "method" => "eth_getBlockByNumber",
                   "params" => [number, false],
                   "id" => id
                 } = request

          assert is_integer(id)
          block = Map.fetch!(by_number, String.downcase(number))
          reply(id, block, Map.get(outcomes, block["number"], :valid))
        end)

      send(owner, {:rpc_batch, phase, requests})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(result))
    end)

    arguments = [
      transport: HTTP,
      transport_options: [
        http: HTTP.HTTPoison,
        http_options: [recv_timeout: 5_000, timeout: 5_000],
        urls: ["http://127.0.0.1:#{server.port}/"]
      ]
    ]

    %{blocks: blocks, rows: rows, responses: responses, arguments: arguments}
  end

  test "empty-block-batch-errors: mixed batch processes valid siblings and retains null and error candidates",
       context do
    [empty, missing, failed, non_empty] = context.blocks
    serve(context, :mixed, %{missing["number"] => :null, failed["number"] => :error})
    pid = start_sanitizer(context)
    assert_batch(pid, :mixed, context.rows)

    assert_classified(empty)
    assert_classified(non_empty)
    assert_retryable(missing)
    assert_retryable(failed)
  end

  test "empty-block-batch-errors: unresolved candidates recover after the sanitizer stops and reopens", context do
    [empty, missing, failed, non_empty] = context.blocks
    serve(context, :before_restart, %{missing["number"] => :null, failed["number"] => :error})
    first = start_sanitizer(context)
    assert_batch(first, :before_restart, context.rows)
    assert_classified(empty)
    assert_classified(non_empty)
    assert_retryable(missing)
    assert_retryable(failed)

    assert :ok = stop_supervised(:sanitizer)
    refute Process.alive?(first)
    serve(context, :after_restart, %{})
    reopened = start_sanitizer(context)
    assert reopened != first
    retry_rows = Enum.filter(context.rows, &(to_string(&1.hash) in [missing["hash"], failed["hash"]]))
    assert_batch(reopened, :after_restart, retry_rows)
    Enum.each(context.blocks, &assert_classified/1)
  end

  for outcome <- [
        :missing_number,
        :invalid_number,
        :negative_number,
        :wrong_number,
        :missing_hash,
        :invalid_hash,
        :wrong_hash,
        :missing_transactions,
        :null_transactions,
        :object_transactions,
        :invalid_transaction_hash,
        :transaction_object,
        :unknown_id,
        :missing_response,
        :duplicate_id
      ] do
    @outcome outcome
    test "empty-block-batch-errors: #{@outcome} stays retryable while valid siblings progress", context do
      [invalid | siblings] = context.blocks
      serve(context, @outcome, %{invalid["number"] => @outcome})
      pid = start_sanitizer(context)
      assert_batch(pid, @outcome, context.rows)
      assert_retryable(invalid)
      Enum.each(siblings, &assert_classified/1)
    end
  end

  test "empty-block-batch-errors: swapped block identities cannot classify either candidate", context do
    [first, second | siblings] = context.blocks
    serve(context, :swapped, %{first["number"] => {:block, second}, second["number"] => {:block, first}})
    pid = start_sanitizer(context)
    assert_batch(pid, :swapped, context.rows)
    assert_retryable(first)
    assert_retryable(second)
    Enum.each(siblings, &assert_classified/1)
  end

  test "empty-block-batch-errors: retry in the same process requests only unresolved candidates", context do
    [missing, failed | siblings] = context.blocks
    serve(context, :first_attempt, %{missing["number"] => :null, failed["number"] => :error})
    pid = start_sanitizer(context)
    assert_batch(pid, :first_attempt, context.rows)
    assert_retryable(missing)
    assert_retryable(failed)
    Enum.each(siblings, &assert_classified/1)

    serve(context, :retry, %{})
    send(pid, :sanitize_empty_blocks)
    retry_rows = Enum.filter(context.rows, &(to_string(&1.hash) in [missing["hash"], failed["hash"]]))
    assert_batch(pid, :retry, retry_rows)
    Enum.each(context.blocks, &assert_classified/1)
  end

  defp start_sanitizer(context) do
    start_supervised!(%{
      id: :sanitizer,
      start: {EmptyBlocksSanitizer, :start_link, [[json_rpc_named_arguments: context.arguments]]},
      restart: :temporary
    })
  end

  defp serve(context, phase, outcomes) do
    Agent.update(context.responses, fn _ -> {phase, outcomes} end)
  end

  defp assert_batch(pid, phase, rows) do
    assert_receive {:rpc_batch, ^phase, requests}, 10_000
    assert %EmptyBlocksSanitizer{} = :sys.get_state(pid, 10_000)
    assert Process.alive?(pid)

    assert Enum.sort(
             Enum.map(requests, fn %{"params" => [number, false]} ->
               EthereumJSONRPC.quantity_to_integer(number)
             end)
           ) == Enum.sort(Enum.map(rows, & &1.number))

    ids = Enum.map(requests, & &1["id"])
    assert length(Enum.uniq(ids)) == length(rows)
  end

  defp assert_retryable(block) do
    row = Repo.get!(Block, block["hash"])
    assert is_nil(row.is_empty)
    assert row.refetch_needed == false
    assert Repo.get(PendingBlockOperation, row.hash)
  end

  defp assert_classified(block) do
    row = Repo.get!(Block, block["hash"])

    if block["transactions"] == [] do
      assert row.is_empty == true
      assert row.refetch_needed == false
      refute Repo.get(PendingBlockOperation, row.hash)
    else
      assert is_nil(row.is_empty)
      assert row.refetch_needed == true
      assert Repo.get(PendingBlockOperation, row.hash)
    end
  end

  defp reply(id, block, outcome) do
    response = %{"jsonrpc" => "2.0", "id" => id, "result" => block}

    case outcome do
      :valid -> [response]
      :null -> [%{response | "result" => nil}]
      :error -> [rpc_error(id)]
      :missing_response -> []
      :duplicate_id -> [response, rpc_error(id)]
      :unknown_id -> [%{response | "id" => id + 10_000}]
      {:block, other} -> [%{response | "result" => other}]
      invalid -> [%{response | "result" => corrupt(block, invalid)}]
    end
  end

  defp rpc_error(id) do
    %{"jsonrpc" => "2.0", "id" => id, "error" => %{"code" => -32000, "message" => "header not found"}}
  end

  defp corrupt(block, :missing_number), do: Map.delete(block, "number")
  defp corrupt(block, :invalid_number), do: Map.put(block, "number", "0x123xyz")
  defp corrupt(block, :negative_number), do: Map.put(block, "number", "0x-1")
  defp corrupt(block, :wrong_number), do: Map.put(block, "number", "0x0")
  defp corrupt(block, :missing_hash), do: Map.delete(block, "hash")
  defp corrupt(block, :invalid_hash), do: Map.put(block, "hash", "0x123")
  defp corrupt(block, :wrong_hash), do: Map.put(block, "hash", "0x" <> String.duplicate("0", 64))
  defp corrupt(block, :missing_transactions), do: Map.delete(block, "transactions")
  defp corrupt(block, :null_transactions), do: Map.put(block, "transactions", nil)
  defp corrupt(block, :object_transactions), do: Map.put(block, "transactions", %{})
  defp corrupt(block, :invalid_transaction_hash), do: Map.put(block, "transactions", ["0x123"])
  defp corrupt(block, :transaction_object), do: Map.put(block, "transactions", [%{"hash" => block["hash"]}])
end
