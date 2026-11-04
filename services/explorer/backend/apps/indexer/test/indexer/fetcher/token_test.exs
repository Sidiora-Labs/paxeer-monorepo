Code.require_file("repair_22_33_555_support.exs", __DIR__)

defmodule Indexer.Fetcher.TokenTest do
  use EthereumJSONRPC.Case
  use Explorer.DataCase

  import Mox

  alias Explorer.Chain
  alias Explorer.Chain.Token
  alias Indexer.Fetcher.Token, as: TokenFetcher

  setup :verify_on_exit!

  describe "init/3" do
    test "returns uncataloged tokens", %{json_rpc_named_arguments: json_rpc_named_arguments} do
      insert(:token, cataloged: true)
      %Token{contract_address_hash: uncatalog_address} = insert(:token, cataloged: false)

      assert TokenFetcher.init([], &[&1 | &2], json_rpc_named_arguments) == [uncatalog_address]
    end
  end

  describe "run/3" do
    test "catalogs tokens that haven't been cataloged", %{json_rpc_named_arguments: json_rpc_named_arguments} do
      json_rpc_named_arguments =
        case Keyword.fetch!(json_rpc_named_arguments, :transport) do
          EthereumJSONRPC.Mox ->
            json_rpc_named_arguments

          EthereumJSONRPC.HTTP ->
            http = json_rpc_named_arguments |> Keyword.fetch!(:transport_options) |> Keyword.fetch!(:http)
            Indexer.Fetcher.Repair2233555.catalog_arguments(http)
        end

      Indexer.Fetcher.Repair2233555.put_env(:explorer, :json_rpc_named_arguments, json_rpc_named_arguments)
      address = insert(:address, hash: Indexer.Fetcher.Repair2233555.fixture("catalog")["address"])

      token =
        insert(:token,
          contract_address: address,
          name: nil,
          symbol: nil,
          total_supply: nil,
          decimals: nil,
          cataloged: false
        )

      contract_address_hash = token.contract_address_hash

      if json_rpc_named_arguments[:transport] == EthereumJSONRPC.Mox do
        expect(
          EthereumJSONRPC.Mox,
          :json_rpc,
          1,
          fn requests, _opts ->
            {:ok,
             Enum.map(requests, fn
               %{id: id, method: "eth_call", params: [%{data: "0x313ce567", to: _}, "latest"]} ->
                 %{
                   id: id,
                   result: "0x0000000000000000000000000000000000000000000000000000000000000012"
                 }

               %{id: id, method: "eth_call", params: [%{data: "0x06fdde03", to: _}, "latest"]} ->
                 %{
                   id: id,
                   result:
                     "0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000642616e636f720000000000000000000000000000000000000000000000000000"
                 }

               %{id: id, method: "eth_call", params: [%{data: "0x95d89b41", to: _}, "latest"]} ->
                 %{
                   id: id,
                   result:
                     "0x00000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003424e540000000000000000000000000000000000000000000000000000000000"
                 }

               %{id: id, method: "eth_call", params: [%{data: "0x18160ddd", to: _}, "latest"]} ->
                 %{
                   id: id,
                   result: "0x0000000000000000000000000000000000000000000000000de0b6b3a7640000"
                 }
             end)}
          end
        )
      end

      assert TokenFetcher.run([contract_address_hash], json_rpc_named_arguments) == :ok

      expected_supply = Decimal.new(1_000_000_000_000_000_000)

      decimals_expected = Decimal.new(18)

      assert {:ok,
              %Token{
                name: "Bancor",
                symbol: "BNT",
                total_supply: ^expected_supply,
                decimals: ^decimals_expected,
                cataloged: true
              }} = Chain.token_from_address_hash(contract_address_hash)
    end
  end
end
