Code.require_file("../block_scout_web/views/address_view_test.exs", __DIR__)
Code.require_file("../block_scout_web/views/layout_view_test.exs", __DIR__)

defmodule BlockScoutWeb.Repair2233378Test do
  @moduledoc """
  Task 22.33.378, audit 22.30.140:
  address-test-global-state: "address examples restore present, nil and absent settings";
  layout-test-global-state: "layout examples restore present, nil and absent settings".
  Both findings also use "restored configuration reaches real address HTML and JSON responses".
  Baseline verification was not run: the task's gate order requires implementation first
  and prohibits running the verify command by hand. No baseline failures are claimed.
  Wiring requests: none; both proposals are wholly within the permitted test paths.
  The qualified revision and command evidence are recorded by hf-done.
  """

  use ExUnit.Case, async: false

  import Explorer.Factory
  import Phoenix.ConnTest

  alias BlockScoutWeb.{AddressViewTest, ConnCase, LayoutView, LayoutViewTest}
  alias BlockScoutWeb.Routers.WebRouter
  alias Explorer.Chain.Cache.Counters.AddressesCount
  alias Explorer.Chain.Supply.ProofOfAuthority

  @endpoint BlockScoutWeb.Endpoint
  @address_settings [{:block_scout_web, :show_percentage}, {:explorer, :supply}]
  @layout_settings Enum.map([BlockScoutWeb.Chain, :blockscout_version, :release_link, :other_networks], fn key ->
                     {:block_scout_web, key}
                   end)
  @settings @address_settings ++ @layout_settings ++ [{:block_scout_web, :version}]
  @chain [network: "Recorded POA", subnetwork: "Recorded Sokol", logo: "/images/recorded.svg", footer: []]
  @networks ~s([{"title":"Recorded Network","url":"https://blockscout.com/poa/core"}])

  setup do
    original = snapshot(@settings)
    on_exit(fn -> restore(original) end)
  end

  test "address examples restore present, nil and absent settings" do
    tests =
      Enum.filter(AddressViewTest.__ex_unit__().tests, fn test ->
        test.tags[:describe] == "balance_percentage_enabled/1" or test.name == :"test balance_percentage/1"
      end)

    assert length(tests) == 4
    assert AddressViewTest.__ex_unit__(:config).async? == false

    for state <- [:present, nil, :absent], test <- tests do
      seed(state)
      before = snapshot(@settings)
      run_example(AddressViewTest, test)
      assert snapshot(@settings) == before
    end
  end

  test "layout examples restore present, nil and absent settings" do
    tests = LayoutViewTest.__ex_unit__().tests
    assert length(tests) > 0
    assert LayoutViewTest.__ex_unit__(:config).async? == false

    for state <- [:present, nil, :absent], test <- tests do
      seed(state)
      before = snapshot(@settings)
      run_example(LayoutViewTest, test)
      assert snapshot(@settings) == before
    end
  end

  test "restored configuration reaches real address HTML and JSON responses" do
    seed(:present)
    before = snapshot(@settings)

    for module <- [AddressViewTest, LayoutViewTest] do
      tests =
        Enum.filter(module.__ex_unit__().tests, fn test ->
          module == LayoutViewTest or test.tags[:describe] == "balance_percentage_enabled/1"
        end)

      Enum.each(tests, &run_example(module, &1))
    end

    assert snapshot(@settings) == before
    ConnCase.__ex_unit__(:setup, %{async: false})

    address =
      insert(:address,
        hash: "0x8bf38d4764929064f2d4d3a56520a76ab3df415b",
        fetched_coin_balance: 2_524_608_000_000_000_000_000_000,
        fetched_coin_balance_block_number: 0
      )

    if is_nil(Process.whereis(AddressesCount)) do
      start_supervised!(AddressesCount)
    end

    AddressesCount.consolidate()

    # The hash and POA balance are the committed AddressViewTest chain examples.
    body = "/accounts?type=JSON" |> accounts_request() |> json_response(200)
    assert %{"items" => [tile], "next_page_path" => nil} = body
    assert String.downcase(tile) =~ to_string(address.hash)
    assert tile =~ "1.0000% Market Cap"

    page = "/accounts" |> accounts_request() |> html_response(200)
    assert page =~ "Recorded Sokol"
    assert page =~ "/images/recorded.svg"
    assert page =~ "https://blockscout.com/poa/core"
    assert page =~ "Recorded Network"
    assert page =~ "https://github.com/blockscout/blockscout/releases/tag/v1.3.4-beta"
    assert LayoutView.network_title() == "Recorded POA"
    assert snapshot(@settings) == before
  end

  defp accounts_request(path) do
    Plug.Test.conn(:get, path)
    |> Map.put(:secret_key_base, @endpoint.config(:secret_key_base))
    |> Plug.Test.init_test_session(%{})
    |> Plug.Conn.put_private(:phoenix_endpoint, @endpoint)
    |> WebRouter.call(WebRouter.init([]))
  end

  defp run_example(module, test) do
    {pid, ref} =
      spawn_monitor(fn ->
        :ok = ExUnit.OnExitHandler.register(self())
        tags = Map.put(test.tags, :async, module.__ex_unit__(:config).async?)
        context = module.__ex_unit__(:setup, tags)
        apply(module, test.name, [context])
      end)

    assert_receive {:DOWN, ^ref, :process, ^pid, reason}, 10_000
    cleanup = ExUnit.OnExitHandler.run(pid, 10_000)
    assert reason == :normal
    assert cleanup == :ok
  end

  defp snapshot(settings) do
    Enum.map(settings, fn {app, key} -> {app, key, Application.fetch_env(app, key)} end)
  end

  defp restore(settings) do
    Enum.each(settings, fn
      {app, key, {:ok, value}} -> Application.put_env(app, key, value)
      {app, key, :error} -> Application.delete_env(app, key)
    end)
  end

  defp seed(:present) do
    Application.put_env(:block_scout_web, :show_percentage, true)
    Application.put_env(:explorer, :supply, ProofOfAuthority)
    Application.put_env(:block_scout_web, BlockScoutWeb.Chain, @chain)
    Application.put_env(:block_scout_web, :blockscout_version, "v1.3.4-beta")
    Application.put_env(:block_scout_web, :version, "v1.3.4-beta")

    Application.put_env(
      :block_scout_web,
      :release_link,
      "https://github.com/blockscout/blockscout/releases/tag/v1.3.4-beta"
    )

    Application.put_env(:block_scout_web, :other_networks, @networks)
  end

  defp seed(nil) do
    Enum.each(@settings, fn {app, key} -> Application.put_env(app, key, nil) end)
  end

  defp seed(:absent) do
    Enum.each(@settings, fn {app, key} -> Application.delete_env(app, key) end)
  end
end
