Code.require_file("../indexer/fetcher/coin_balance/catchup_test.exs", __DIR__)

defmodule Indexer.Repair2233556Test do
  @moduledoc """
  Repair record for task 22.33.556, audit 22.30.158 finding 010.

  CatchupTest's duplicate-address case now requires :ok and unconditionally
  checks both historical balances and the latest address balance. Requiring
  that suite here executes the repaired case and its separate retry cases.
  The tests below additionally replay its retained Nethermind balance data
  over real HTTP through the production balance decoder and import runners.

  Baseline: no earlier gate ledger exists; no unchanged-tree verification was
  run because the ordered gate instructions require implementation first.
  Wiring requests: none. The verified candidate revision and private log are
  recorded by hf-done in its gate ledger, without changing this source tree.
  """

  use Explorer.DataCase, async: false

  alias EthereumJSONRPC.FetchedBalances
  alias Explorer.Chain
  alias Explorer.Chain.{Address, Wei}
  alias Indexer.Fetcher.CoinBalance.Helper

  @moduletag :capture_log

  @address "0xe8ddc5c7a2d2f0d7a9798459c0104fdf5e987aca"
  @balances %{1 => 252_460_801_000_000_000_000_000_000, 2 => 252_460_802_000_000_000_000_000_000}

  setup do
    rpc = Bypass.open()

    arguments = [
      transport: EthereumJSONRPC.HTTP,
      transport_options: [
        http: EthereumJSONRPC.HTTP.HTTPoison,
        urls: ["http://localhost:#{rpc.port}"],
        http_options: [timeout: 5_000, recv_timeout: 5_000]
      ],
      variant: EthereumJSONRPC.Nethermind
    ]

    %{rpc: rpc, arguments: arguments}
  end

  test "010: recorded duplicate-address quantities reach both balance import runners", context do
    import_recorded_balances(context, [1, 2])
    assert_imported_balances()
  end

  test "010: reversed requests and responses retain historical values and the newest address balance", context do
    import_recorded_balances(context, [2, 1])
    assert_imported_balances()
  end

  defp import_recorded_balances(%{rpc: rpc, arguments: arguments}, numbers) do
    owner = self()

    Bypass.expect_once(rpc, "POST", "/", fn conn ->
      {:ok, body, conn} = Plug.Conn.read_body(conn)
      requests = Jason.decode!(body)
      send(owner, {:balance_requests, requests})

      responses =
        requests
        |> Enum.map(fn %{"id" => id, "jsonrpc" => "2.0", "method" => "eth_getBalance", "params" => [@address, quantity]} ->
          number = EthereumJSONRPC.quantity_to_integer(quantity)

          %{
            jsonrpc: "2.0",
            id: id,
            result: EthereumJSONRPC.integer_to_quantity(Map.fetch!(@balances, number))
          }
        end)
        |> Enum.reverse()

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, Jason.encode!(responses))
    end)

    params =
      Enum.map(numbers, fn number ->
        %{hash_data: @address, block_quantity: EthereumJSONRPC.integer_to_quantity(number)}
      end)

    assert {:ok, %FetchedBalances{errors: [], params_list: balances}} =
             EthereumJSONRPC.fetch_balances(params, arguments, 2)

    assert_receive {:balance_requests, requests}

    assert requests ==
             numbers
             |> Enum.with_index()
             |> Enum.map(fn {number, id} ->
               %{
                 "id" => id,
                 "jsonrpc" => "2.0",
                 "method" => "eth_getBalance",
                 "params" => [@address, EthereumJSONRPC.integer_to_quantity(number)]
               }
             end)

    assert Map.new(balances, &{&1.block_number, &1.value}) == @balances

    fetched_at = DateTime.utc_now()

    assert {:ok, imported} =
             Chain.import(%{
               addresses: %{params: Helper.balances_params_to_address_params(balances), with: :balance_changeset},
               address_coin_balances: %{params: Enum.map(balances, &Map.put(&1, :value_fetched_at, fetched_at))},
               broadcast: false
             })

    assert length(imported.addresses) == 1
    assert length(imported.address_coin_balances) == 2
  end

  defp assert_imported_balances do
    balances = Repo.all(from(balance in Address.CoinBalance, where: balance.address_hash == ^@address))
    assert length(balances) == 2

    for {number, value} <- @balances do
      expected = %Wei{value: Decimal.new(value)}
      balance = Enum.find(balances, &(&1.block_number == number))
      assert %Address.CoinBalance{value: ^expected, value_fetched_at: %DateTime{}} = balance
    end

    address = Repo.get!(Address, @address)
    assert address.fetched_coin_balance == %Wei{value: Decimal.new(Map.fetch!(@balances, 2))}
    assert address.fetched_coin_balance_block_number == 2
  end
end
