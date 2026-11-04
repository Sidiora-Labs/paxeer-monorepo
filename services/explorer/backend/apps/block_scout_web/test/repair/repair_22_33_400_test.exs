defmodule BlockScoutWeb.Repair22_33_400Test do
  @moduledoc """
  Repair record 22.33.400 for audit 22.30.142, finding 23.

  Proving cases: "finding 23: routed invalid address renders 422",
  "finding 23: routed unverified contract renders 404",
  "finding 23: routed restricted address renders 403", and
  "finding 23: routed empty upstream response renders 500".
  The missing-address, non-contract and Vyper cases cover the other failure tuples.
  Contracts use Explorer.Factory's committed SimpleStorage bytecode, source and ABI.
  The upstream cases traverse the production SolidityScan integration and HTTP client
  over a real Mint connection to a loopback HTTP fixture, using the adapter's existing
  connection-reuse option because the integration fixes its remote origin.
  No out-of-path wiring changes are required.
  Baseline verification was not run: the explicit bounded gate order requires the
  complete implementation before hf-build and hf-done; no baseline failures are claimed.
  The gate ledger records the verified candidate revision and private evidence path.
  """

  use BlockScoutWeb.ConnCase, async: false

  alias Explorer.ThirdPartyIntegrations.SolidityScan

  defmodule UpstreamFixture do
    @moduledoc false
    @behaviour Plug

    def init(options), do: options

    def call(conn, {owner, status, body}) do
      send(owner, {:upstream_request, conn.method, conn.request_path, Plug.Conn.get_req_header(conn, "authorization")})

      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.send_resp(status, body)
    end
  end

  setup do
    for {app, key} <- [
          {:explorer, :addresses_blacklist},
          {:explorer, :addresses_blacklist_key},
          {:explorer, :http_client},
          {:explorer, SolidityScan},
          {:tesla, :adapter}
        ] do
      previous = Application.fetch_env(app, key)

      on_exit(fn ->
        case previous do
          {:ok, value} -> Application.put_env(app, key, value)
          :error -> Application.delete_env(app, key)
        end
      end)
    end

    Application.put_env(:explorer, :addresses_blacklist, nil)
    Application.put_env(:explorer, :addresses_blacklist_key, "repair-22-33-400")
    Application.put_env(:explorer, :http_client, Explorer.HttpClient.Tesla)
    Application.put_env(:explorer, SolidityScan, platform_id: "ethereum", chain_id: "1", api_key: "recorded-token")
    :ok
  end

  test "finding 23: routed invalid address renders 422", %{conn: conn} do
    assert %{"message" => "Invalid address hash"} = conn |> get(report_path("invalid")) |> json_response(422)
  end

  test "finding 23: routed unverified contract renders 404", %{conn: conn} do
    address = insert(:address, contract_code: contract_code_info().bytecode)

    assert %{"message" => "Smart-contract is unverified"} =
             conn |> get(report_path(address.hash)) |> json_response(404)
  end

  test "finding 23: routed restricted address renders 403", %{conn: conn} do
    contract = insert(:smart_contract, language: :solidity, is_vyper_contract: false)
    Application.put_env(:explorer, :addresses_blacklist, to_string(contract.address_hash))

    for params <- [%{}, %{"key" => "wrong"}] do
      assert %{"message" => "Restricted access"} =
               conn |> get(report_path(contract.address_hash), params) |> json_response(403)
    end
  end

  test "finding 23: routed empty upstream response renders 500", %{conn: conn} do
    contract = insert(:smart_contract, language: :solidity, is_vyper_contract: false)
    serve_upstream(200, "null")

    assert %{"message" => "Empty response"} = conn |> get(report_path(contract.address_hash)) |> json_response(500)
    assert_upstream_request(contract.address_hash)
  end

  test "finding 23: routed unavailable upstream renders 500", %{conn: conn} do
    contract = insert(:smart_contract, language: :solidity, is_vyper_contract: false)
    serve_upstream(503, ~s({"detail":"Service unavailable"}))

    assert %{"message" => "Empty response"} = conn |> get(report_path(contract.address_hash)) |> json_response(500)
    assert_upstream_request(contract.address_hash)
  end

  test "finding 23: routed missing address renders 404", %{conn: conn} do
    address = build(:address)
    assert %{"message" => "Address not found"} = conn |> get(report_path(address.hash)) |> json_response(404)
  end

  test "finding 23: routed non-contract address renders 404", %{conn: conn} do
    address = insert(:address, contract_code: nil)

    assert %{"message" => "Address is not smart-contract"} =
             conn |> get(report_path(address.hash)) |> json_response(404)
  end

  test "finding 23: routed Vyper contract renders 404", %{conn: conn} do
    info = contract_code_info_vyper()
    {:ok, code} = Explorer.Chain.Data.cast(info.bytecode)
    address = insert(:address, contract_code: info.bytecode, verified: true)

    contract =
      insert(:smart_contract,
        address_hash: address.hash,
        language: :vyper,
        is_vyper_contract: true,
        name: info.name,
        contract_source_code: info.source_code,
        contract_code_md5: Explorer.SmartContract.Helper.md5(code.bytes),
        abi: info.abi,
        compiler_version: info.version
      )

    assert %{"message" => "Vyper smart-contracts are not supported by SolidityScan"} =
             conn |> get(report_path(contract.address_hash)) |> json_response(404)
  end

  defp report_path(hash) do
    "/api/v2/proxy/3rdparty/solidityscan/smart-contracts/#{hash}/report"
  end

  defp serve_upstream(status, body) do
    start_supervised!(
      {Plug.Cowboy,
       scheme: :http, plug: {UpstreamFixture, {self(), status, body}}, options: [ip: {127, 0, 0, 1}, port: 0]}
    )

    port = :ranch.get_port(Module.concat(UpstreamFixture, HTTP))

    {:ok, connection} =
      Mint.HTTP.connect(:http, "127.0.0.1", port, mode: :passive, hostname: "api.solidityscan.com")

    Application.put_env(
      :tesla,
      :adapter,
      {Tesla.Adapter.Mint, conn: connection, original: "api.solidityscan.com:443", mode: :passive}
    )
  end

  defp assert_upstream_request(hash) do
    path = "/api/v1/quickscan/ethereum/1/#{hash}"
    assert_receive {:upstream_request, "GET", ^path, ["Token recorded-token"]}
  end
end
