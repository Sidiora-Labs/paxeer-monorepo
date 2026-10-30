defmodule EthereumJSONRPC.PaxeerXTraceParityTest do
  use ExUnit.Case, async: false

  import Mox

  alias EthereumJSONRPC.Geth

  setup :verify_on_exit!

  setup do
    initial_env = Application.get_env(:ethereum_jsonrpc, Geth)

    on_exit(fn ->
      if is_nil(initial_env) do
        Application.delete_env(:ethereum_jsonrpc, Geth)
      else
        Application.put_env(:ethereum_jsonrpc, Geth, initial_env)
      end
    end)

    Application.put_env(
      :ethereum_jsonrpc,
      Geth,
      Keyword.merge(initial_env || [], tracer: "call_tracer", debug_trace_timeout: "5s")
    )

    :ok
  end

  test "call_tracer_block_decodes_internal_transactions" do
    %{"id" => id, "jsonrpc" => jsonrpc, "result" => result} =
      __DIR__
      |> Path.join("../support/fixture/paxeer_x/call_tracer_block.json")
      |> File.read!()
      |> Jason.decode!()

    transaction_hash = "0x32b17f27ddb546eab3c4c33f31eb22c1cb992d4ccc50dae26922805b717efe5c"

    expect(EthereumJSONRPC.Mox, :json_rpc, fn
      [
        %{
          id: ^id,
          jsonrpc: "2.0",
          method: "debug_traceBlockByNumber",
          params: ["0x2A", %{"tracer" => "callTracer", :timeout => "5s"}]
        }
      ],
      [] ->
        {:ok, [%{id: id, jsonrpc: jsonrpc, result: result}]}
    end)

    assert Geth.fetch_block_internal_transactions([42],
             transport: EthereumJSONRPC.Mox,
             transport_options: [],
             variant: EthereumJSONRPC.PaxeerX
           ) ==
             {:ok,
              [
                %{
                  block_number: 42,
                  transaction_index: 0,
                  transaction_hash: transaction_hash,
                  index: 0,
                  trace_address: [],
                  type: "create",
                  from_address_hash: "0x1111111111111111111111111111111111111111",
                  created_contract_address_hash: "0x2222222222222222222222222222222222222222",
                  created_contract_code: "0x00",
                  gas: 100_000,
                  gas_used: 50_000,
                  init: "0x60006000f3",
                  value: 42
                },
                %{
                  block_number: 42,
                  transaction_index: 0,
                  transaction_hash: transaction_hash,
                  index: 1,
                  trace_address: [0],
                  type: "call",
                  call_type: "call",
                  from_address_hash: "0x2222222222222222222222222222222222222222",
                  to_address_hash: "0x3333333333333333333333333333333333333333",
                  gas: 30_000,
                  gas_used: 21_000,
                  input: "0x12345678",
                  output: "0x01",
                  value: 7
                }
              ]}
  end
end
