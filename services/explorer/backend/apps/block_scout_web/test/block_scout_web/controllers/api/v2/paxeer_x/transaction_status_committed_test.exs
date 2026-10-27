defmodule BlockScoutWeb.API.V2.PaxeerX.TransactionStatusCommittedTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain
  alias Explorer.Chain.PaxeerX.SequencerReceipt
  alias Plug.Conn

  @vectors Path.expand(
             "../../../../../../../explorer/test/support/fixture/paxeer_x/committed_vectors.json",
             __DIR__
           )

  setup do
    original_receipt_config = Application.get_env(:explorer, SequencerReceipt)
    original_adapter = Application.get_env(:tesla, :adapter)
    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      restore_env(:explorer, SequencerReceipt, original_receipt_config)
      restore_env(:tesla, :adapter, original_adapter)
    end)

    %{"cases" => cases} = @vectors |> File.read!() |> Jason.decode!()
    {:ok, vector: Enum.find(cases, &(&1["id"] == "committed-ok"))}
  end

  test "status_answers_committed_for_router_verified_hash", context do
    status_answers_committed_for_router_verified_hash(context)
  end

  defp status_answers_committed_for_router_verified_hash(%{conn: conn, vector: vector}) do
    "0x" <> raw_transaction = vector["rawTransaction"]

    hash =
      "0x" <>
        (raw_transaction |> Base.decode16!(case: :mixed) |> ExKeccak.hash_256() |> Base.encode16(case: :lower))

    {:ok, transaction_hash} = Chain.string_to_full_hash(hash)
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    calls = [
      {"pax_sequencerReceipt", [hash], vector["receiptJson"]},
      {"px_getReceiptKeys", [], [vector["pinnedKey"]]},
      {"eth_getTransactionReceipt", [hash], vector["eth_receipt"]},
      {"px_getCommit", [1_000], vector["commit"]},
      {"px_getValidators", [1_000], vector["validators"]},
      {"px_getSequenceBatch", [1_000], vector["batch"]}
    ]

    serve_rpc(calls)

    response = json_response(get(conn, "/api/v2/transactions/#{hash}/status"), 200)

    assert response == %{"rung" => "committed", "block_number" => nil, "committed_height" => 1_000}
    assert {:error, :not_found} = Chain.hash_to_transaction(transaction_hash, api?: true)

    for {method, params, _result} <- calls do
      assert_received {:rpc_call, ^method, ^params}
    end

    refute_received {:rpc_call, _method, _params}
  end

  defp serve_rpc(calls) do
    bypass = Bypass.open()
    test_pid = self()
    answers = Map.new(calls, fn {method, params, result} -> {method, {params, result}} end)

    Bypass.expect(bypass, "POST", "/", fn conn ->
      {:ok, body, conn} = Conn.read_body(conn)
      assert %{"jsonrpc" => "2.0", "id" => id, "method" => method, "params" => params} = Jason.decode!(body)
      assert {^params, result} = Map.fetch!(answers, method)
      send(test_pid, {:rpc_call, method, params})

      conn
      |> Conn.put_resp_content_type("application/json")
      |> Conn.resp(200, Jason.encode!(%{jsonrpc: "2.0", id: id, result: result}))
    end)

    Application.put_env(:explorer, SequencerReceipt, router_url: "http://localhost:#{bypass.port}/")
  end

  defp restore_env(app, key, nil), do: Application.delete_env(app, key)
  defp restore_env(app, key, value), do: Application.put_env(app, key, value)
end
