Code.require_file("../block_scout_web/features/viewing_app_test.exs", __DIR__)

defmodule BlockScoutWeb.Repair22_33_475Test do
  @moduledoc """
  Regression evidence for audit 22.30.149. The required file also loads all five
  restored ViewingAppTest scenarios, exercising the indexing API and V2 channels.
  Contract data is the committed SimpleStorage source, ABI, creation input and
  deployed bytecode in Explorer.Factory.contract_code_info/0. Compiler discovery
  uses the committed solc_bin.json over loopback HTTP, with the production client.
  The browser/compiler success execution remains an owning-domain wiring request;
  the focused gate proves publishing, persisted metadata, result delivery and routing.
  """

  use BlockScoutWeb.ConnCase, async: false

  import Phoenix.ChannelTest

  alias BlockScoutWeb.{AddressChannel, UserSocket}
  alias Explorer.Chain.{Address, SmartContract}
  alias Explorer.Chain.Events.Publisher, as: EventsPublisher
  alias Explorer.SmartContract.Solidity.Publisher

  setup %{conn: conn} do
    bypass = Bypass.open()
    versions = File.read!(Path.expand("../support/fixture/smart_contract/solc_bin.json", __DIR__))

    Bypass.expect(bypass, "GET", "/bin/list.json", fn request ->
      request |> put_resp_content_type("application/json") |> resp(200, versions)
    end)

    for {app, key, value} <- [
          {:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, [enabled: false]},
          {:explorer, :solc_bin_api_url, "http://localhost:#{bypass.port}"},
          {:tesla, :adapter, Tesla.Adapter.Mint}
        ] do
      previous = Application.fetch_env(app, key)
      Application.put_env(app, key, value)

      on_exit(fn ->
        case previous do
          {:ok, old} -> Application.put_env(app, key, old)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    code = Explorer.Factory.contract_code_info()
    address = insert(:contract_address, contract_code: code.bytecode)

    :transaction
    |> insert(created_contract_address_hash: address.hash, input: code.tx_input)
    |> with_block(status: :ok)

    conn =
      conn
      |> Plug.Test.init_test_session(%{})
      |> put_private(:phoenix_endpoint, BlockScoutWeb.Endpoint)
      |> Map.put(:secret_key_base, BlockScoutWeb.Endpoint.config(:secret_key_base))

    {:ok, conn: conn, address: address, code: code}
  end

  test "verification-browser-test-skipped: real form exposes the headless interaction targets", %{
    conn: conn,
    address: address,
    code: code
  } do
    chooser =
      web_get(conn, "/address/#{Address.checksum(address.hash)}/contract_verifications/new") |> html_response(200)

    [next_path] = chooser |> Floki.parse_document!() |> Floki.attribute("#verify_via_flattened_code_button", "href")
    assert next_path == "/address/#{Address.checksum(address.hash)}/verify-via-flattened-code/new"

    form = web_get(conn, next_path) |> html_response(200) |> Floki.parse_document!()

    assert Floki.attribute(form, "[data-page='contract-verification'] form", "action") == [
             "/verify_smart_contract/contract_verifications"
           ]

    assert Floki.attribute(form, "[data-test='contract_name']", "name") == ["smart_contract[name]"]

    assert Floki.attribute(form, "#smart_contract_contract_source_code", "name") == [
             "smart_contract[contract_source_code]"
           ]

    assert code.version in Floki.attribute(form, "#smart_contract_compiler_version option", "value")
    assert "byzantium" in Floki.attribute(form, "#smart_contract_evm_version option", "value")
    assert Floki.text(Floki.find(form, "label[for='smart_contract_optimization_false']")) == "No"
    assert Floki.text(Floki.find(form, "label[for='smart_contract_optimization_true']")) == "Yes"
    assert Floki.text(Floki.find(form, "[data-submit-button]")) == "Verify & publish"
  end

  test "verification-browser-test-skipped: recorded contract publication reaches the success consumer", %{
    conn: conn,
    address: address,
    code: code
  } do
    conn |> web_get("/address/#{Address.checksum(address.hash)}/verify-via-flattened-code/new") |> html_response(200)
    join_address(address)
    params = contract_params(address, code)
    assert {:ok, %SmartContract{} = contract} = Publisher.publish_smart_contract(address.hash, params, code.abi, false)
    assert contract.name == code.name
    assert contract.contract_source_code == code.source_code
    assert contract.abi == code.abi
    assert contract.compiler_version == code.version

    insert(:proxy_implementation, proxy_address_hash: address.hash, address_hashes: [], names: [])

    EventsPublisher.broadcast(
      [{:contract_verification_result, {String.downcase(to_string(address.hash)), {:ok, contract}}}],
      :on_demand
    )

    assert_push("verification", %{verification_result: :ok}, 5_000)

    detail = conn |> get("/api/v2/smart-contracts/#{address.hash}") |> json_response(200)
    assert detail["is_verified"] == true
    assert detail["name"] == code.name
    assert detail["source_code"] == code.source_code
    assert detail["abi"] == code.abi

    redirect = web_get(conn, "/address/#{Address.checksum(address.hash)}/verify-via-flattened-code/new")
    assert redirected_to(redirect) == "/address/#{Address.checksum(address.hash)}/contracts"
  end

  test "verification-error-not-asserted: submitted empty source returns visible validation over the real channel", %{
    conn: conn,
    address: address,
    code: code
  } do
    join_address(address)
    params = Map.put(contract_params(address, code), "contract_source_code", "")

    request =
      post(conn, "/verify_smart_contract/contract_verifications", %{
        "smart_contract" => params,
        "external_libraries" => %{}
      })

    assert response(request, 204) == ""
    assert_push("verification", %{verification_result: result}, 5_000)
    assert is_binary(result)

    form = Floki.parse_document!(result)
    error = form |> Floki.find("[data-test='contract-source-code-error']") |> Floki.text()
    assert error == "Empty contract source code."
    assert Floki.attribute(form, "#smart_contract_address_hash", "value") == [Address.checksum(address.hash)]
    assert Floki.attribute(form, "[data-test='contract_name']", "value") == [code.name]
    refute Explorer.Repo.get_by(SmartContract, address_hash: address.hash)
  end

  defp contract_params(address, code) do
    %{
      "address_hash" => to_string(address.hash),
      "name" => code.name,
      "compiler_version" => code.version,
      "optimization" => code.optimized,
      "contract_source_code" => code.source_code,
      "evm_version" => "byzantium"
    }
  end

  defp join_address(address) do
    assert {:ok, %{}, _socket} =
             UserSocket
             |> socket("verification-feature", %{locale: "en"})
             |> subscribe_and_join(AddressChannel, "addresses_old:#{address.hash}")
  end

  defp web_get(conn, path) do
    Phoenix.ConnTest.dispatch(conn, BlockScoutWeb.Routers.WebRouter, :get, path)
  end
end
