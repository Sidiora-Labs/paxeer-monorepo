defmodule Explorer.Repair.Repair2233588Test do
  @moduledoc """
  Task 22.33.588, audit 22.30.160, finding transaction-error-only-update.
  The error-only reimport cases prove adding, correcting and clearing error;
  the identical replay case proves the conflict UPDATE remains a no-op.
  The predicate coverage case checks every chain-specific variant's SET fields.

  The embedded transaction, receipt, trace and block exchanges are copied from
  the committed Geth 1.13.15 Shanghai recording in repair_22_31_413_test.exs.
  Production JSON-RPC decoding and schema casting feed Transactions.run/3,
  its import Multi and the disposable database supplied by the declared gate.
  No live node, credentials or additional test setup is required.

  Baseline: no earlier gate ledger or scoped implementation existed. The
  implementation-first gate order forbids an unchanged-tree verification;
  no baseline test failures are claimed. hf-build and hf-done retain revision,
  command, exit and private evidence location. No out-of-path wiring is needed.
  This module is the repair record because spec and gotcha edits are out of scope.
  """

  use ExUnit.Case, async: false

  import Explorer.Factory, only: [insert: 2]

  alias Ecto.Multi
  alias Explorer.Chain.{Hash, Transaction}
  alias Explorer.Chain.Import.Runner.Transactions
  alias Explorer.Repo

  setup do
    :ok = Ecto.Adapters.SQL.Sandbox.checkout(Repo)
    recording = recorded_exchanges()
    raw_transaction = recording["transaction"]["response"]["result"]
    raw_receipt = recording["receipt"]["response"]["result"]
    raw_block = recording["block"]["response"]["result"]
    error = recording["trace"]["response"]["result"]["error"]

    transaction_params =
      raw_transaction
      |> EthereumJSONRPC.Transaction.to_elixir()
      |> EthereumJSONRPC.Transaction.elixir_to_params()

    receipt_params =
      raw_receipt
      |> EthereumJSONRPC.Receipt.to_elixir()
      |> EthereumJSONRPC.Receipt.elixir_to_params()

    params = transaction_params |> Map.merge(receipt_params) |> Map.put(:error, error)
    changeset = Transaction.changeset(%Transaction{}, params)
    assert changeset.valid?, inspect(changeset.errors)
    changes = changeset.changes
    assert changes.status == :error
    assert changes.error == "execution reverted"
    assert to_string(changes.hash) == raw_receipt["transactionHash"]

    {:ok, block_hash} = Hash.Full.cast(raw_block["hash"])
    insert(:block, hash: block_hash, number: changes.block_number)

    for hash <- Enum.uniq([changes.from_address_hash, changes.to_address_hash]) do
      insert(:address, hash: hash)
    end

    %{changes: changes, timestamp: ~U[2026-11-04 00:00:00.000000Z]}
  end

  for initial_error <- [nil, "Out of gas"] do
    @initial_error initial_error
    test "transaction-error-only-update reimport corrects #{inspect(initial_error)} without other execution changes",
         ctx do
      initial = Map.put(ctx.changes, :error, @initial_error)
      assert {:ok, %{transactions: [%Transaction{}]}} = run_transactions(initial, ctx.timestamp)
      before = Repo.get!(Transaction, ctx.changes.hash)
      assert before.error == @initial_error
      assert Map.drop(initial, [:error]) == Map.drop(ctx.changes, [:error])

      corrected_at = DateTime.add(ctx.timestamp, 1, :second)
      assert {:ok, %{transactions: [updated]}} = run_transactions(ctx.changes, corrected_at)
      assert updated.error == ctx.changes.error
      persisted = Repo.get!(Transaction, ctx.changes.hash)
      assert persisted.error == "execution reverted"
      assert persisted.updated_at == corrected_at
      assert execution_fields(persisted) == execution_fields(before)
      assert_identical_replay(ctx.changes, persisted, DateTime.add(corrected_at, 1, :second))
    end
  end

  test "transaction-error-only-update clears a stale error and identical replay stays a no-op", ctx do
    assert {:ok, %{transactions: [_]}} = run_transactions(ctx.changes, ctx.timestamp)
    before = Repo.get!(Transaction, ctx.changes.hash)
    cleared = Map.put(ctx.changes, :error, nil)
    corrected_at = DateTime.add(ctx.timestamp, 1, :second)

    assert {:ok, %{transactions: [updated]}} = run_transactions(cleared, corrected_at)
    assert is_nil(updated.error)
    persisted = Repo.get!(Transaction, ctx.changes.hash)
    assert is_nil(persisted.error)
    assert persisted.updated_at == corrected_at
    assert execution_fields(persisted) == execution_fields(before)
    assert_identical_replay(cleared, persisted, DateTime.add(corrected_at, 1, :second))
  end

  test "transaction-error-only-update identical replay does not update even when import timestamps advance", ctx do
    assert {:ok, %{transactions: [_]}} = run_transactions(ctx.changes, ctx.timestamp)
    persisted = Repo.get!(Transaction, ctx.changes.hash)
    assert_identical_replay(ctx.changes, persisted, DateTime.add(ctx.timestamp, 1, :second))
  end

  test "transaction-error-only-update all conflict predicates compare every directly assigned SET field" do
    source = Path.expand("../../lib/explorer/chain/import/runner/transactions.ex", __DIR__)
    ast = source |> File.read!() |> Code.string_to_quoted!()

    {_, variants} =
      Macro.prewalk(ast, [], fn
        {:defp, _, [{:default_on_conflict, _, _}, [do: query]]} = node, variants ->
          {node, [query | variants]}

        node, variants ->
          {node, variants}
      end)

    assert length(variants) == 6

    for {:from, _, [_binding, clauses]} <- variants do
      assigned_fields =
        for {field, {:fragment, _, ["EXCLUDED." <> column]}} <- clauses[:update][:set] do
          assert Atom.to_string(field) == column
          column
        end

      {:fragment, _, [predicate | existing_fields]} = clauses[:where]
      excluded_fields = Regex.scan(~r/EXCLUDED\.(\w+)/, predicate, capture: :all_but_first) |> List.flatten()

      compared_fields =
        Enum.map(existing_fields, fn {{:., _, [{:transaction, _, _}, field]}, _, []} -> Atom.to_string(field) end)

      assert "error" in assigned_fields
      assert Enum.sort(assigned_fields) == Enum.sort(excluded_fields)
      assert excluded_fields == compared_fields
      assert length(Regex.scan(~r/\?/, predicate)) == length(compared_fields)
    end
  end

  defp execution_fields(transaction) do
    Map.drop(transaction, [:error, :old_block_hash, :updated_at])
  end

  defp assert_identical_replay(changes, persisted, timestamp) do
    assert {:ok, %{transactions: [], recollated_transactions: [], new_pending_transaction_operations: []}} =
             run_transactions(changes, timestamp)

    assert Repo.get!(Transaction, changes.hash) == persisted
    assert Repo.aggregate(Transaction, :count) == 1
  end

  defp run_transactions(changes, timestamp) do
    Multi.new()
    |> Transactions.run([changes], %{timestamps: %{inserted_at: timestamp, updated_at: timestamp}})
    |> Repo.transaction()
  end

  defp recorded_exchanges do
    Jason.decode!(~S'''
    {
      "transaction": {
        "request": {
          "jsonrpc": "2.0",
          "id": 0,
          "method": "eth_getTransactionByHash",
          "params": [
            "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
          ]
        },
        "response": {
          "jsonrpc": "2.0",
          "id": 0,
          "result": {
            "blockHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
            "blockNumber": "0x3",
            "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
            "gas": "0xf4240",
            "gasPrice": "0x77359400",
            "hash": "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
            "input": "0x",
            "nonce": "0x2",
            "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
            "transactionIndex": "0x0",
            "value": "0x0",
            "type": "0x0",
            "chainId": "0x539",
            "v": "0xa96",
            "r": "0xaefeb422af7d0a8744d9130bf2d58e58283f0b5f518e137f2a8ea69574786a28",
            "s": "0x46cab4b0ce1bf03d6d8ed89b251a034d5fd125372967178f0036ecd6e880843c"
          }
        }
      },
      "receipt": {
        "request": {
          "jsonrpc": "2.0",
          "id": 0,
          "method": "eth_getTransactionReceipt",
          "params": [
            "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
          ]
        },
        "response": {
          "jsonrpc": "2.0",
          "id": 0,
          "result": {
            "blockHash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
            "blockNumber": "0x3",
            "contractAddress": null,
            "cumulativeGasUsed": "0x6fd6",
            "effectiveGasPrice": "0x77359400",
            "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
            "gasUsed": "0x6fd6",
            "logs": [],
            "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "status": "0x0",
            "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
            "transactionHash": "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
            "transactionIndex": "0x0",
            "type": "0x0"
          }
        }
      },
      "trace": {
        "request": {
          "jsonrpc": "2.0",
          "id": 0,
          "method": "debug_traceTransaction",
          "params": [
            "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05",
            {
              "tracer": "callTracer",
              "timeout": "5s"
            }
          ]
        },
        "response": {
          "jsonrpc": "2.0",
          "id": 0,
          "result": {
            "from": "0x57b4e1616ddcb5312d3fe708001b4425b1280b7c",
            "gas": "0xf4240",
            "gasUsed": "0x6fd6",
            "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
            "input": "0x",
            "error": "execution reverted",
            "calls": [
              {
                "from": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
                "gas": "0xeaa65",
                "gasUsed": "0x138a",
                "to": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
                "input": "0x",
                "calls": [
                  {
                    "from": "0x9b6650c4de61ac49d05eb30c3f10eb4693a55f1f",
                    "gas": "0x0",
                    "gasUsed": "0x0",
                    "to": "0xbf67e961430f05b2d23b11e3aa5e883d15a85283",
                    "input": "0x",
                    "value": "0x0",
                    "type": "SELFDESTRUCT"
                  }
                ],
                "value": "0x0",
                "type": "CALL"
              }
            ],
            "value": "0x0",
            "type": "CALL"
          }
        }
      },
      "block": {
        "request": {
          "jsonrpc": "2.0",
          "id": 0,
          "method": "eth_getBlockByHash",
          "params": [
            "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
            false
          ]
        },
        "response": {
          "jsonrpc": "2.0",
          "id": 0,
          "result": {
            "baseFeePerGas": "0x280b6c2b",
            "difficulty": "0x0",
            "extraData": "0xd883010d0f846765746888676f312e32312e36856c696e7578",
            "gasLimit": "0xaffd98",
            "gasUsed": "0x6fd6",
            "hash": "0x11f833f1cbee9dd834eedb603eaf977eac04150fa57a1b23f05ab76b43a69486",
            "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
            "miner": "0x0000000000000000000000000000000000000000",
            "mixHash": "0x663ab9c4492329904cdffd62696dca1b301c0c7aec3f915a52095baab30175ac",
            "nonce": "0x0000000000000000",
            "number": "0x3",
            "parentHash": "0x661d8d8d7a85684e590a7541665d14cd70093cf4708000e77d13b5766a63babc",
            "receiptsRoot": "0x261c53ff5540428fdeac963885f93123b7efa67c46eec26a852bc6039cb31095",
            "sha3Uncles": "0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347",
            "size": "0x2a7",
            "stateRoot": "0x5b13be8a6a852e49beeabd4d30e08ddcc113dcee9d60af65b90450bb1f2649ff",
            "timestamp": "0x6aeaa341",
            "totalDifficulty": "0x1",
            "transactions": [
              "0xea227cfb11a0c20427c20cc17e4662c8b5528fa4e2158396d69e8b4487893d05"
            ],
            "transactionsRoot": "0x7c47274f5f950e823f3b7591714409f1924a363f0ed3e2d421f8b9883b942437",
            "uncles": [],
            "withdrawals": [],
            "withdrawalsRoot": "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
          }
        }
      }
    }
    ''')
  end
end
