defmodule BlockScoutWeb.AddressContractVerificationTest do
  use BlockScoutWeb.FeatureCase, async: false

  alias BlockScoutWeb.{AddressContractPage, ContractVerifyPage}
  alias Explorer.Factory

  setup do
    bypass = Bypass.open()
    compiler_version = "v0.4.24+commit.e67f0147"
    compiler_path = "soljson-#{compiler_version}.js"
    compiler_body = File.read!("/opt/solc-0.4.24/node_modules/solc/soljson.js")
    compiler_sha256 = :crypto.hash(:sha256, compiler_body) |> Base.encode16(case: :lower)

    versions =
      Path.expand("../../support/fixture/smart_contract/solc_bin.json", __DIR__)
      |> File.read!()
      |> Jason.decode!()
      |> Map.update!("builds", fn builds ->
        Enum.map(builds, fn build ->
          if build["path"] == compiler_path do
            Map.put(build, "sha256", compiler_sha256)
          else
            build
          end
        end)
      end)
      |> Jason.encode!()

    Bypass.stub(bypass, "GET", "/bin/list.json", fn conn ->
      conn
      |> Plug.Conn.put_resp_content_type("application/json")
      |> Plug.Conn.resp(200, versions)
    end)

    Bypass.stub(bypass, "GET", "/bin/#{compiler_path}", fn conn ->
      conn
      |> Plug.Conn.put_resp_content_type("application/javascript")
      |> Plug.Conn.resp(200, compiler_body)
    end)

    previous_adapter = Application.get_env(:tesla, :adapter)
    previous_solc_url = Application.get_env(:explorer, :solc_bin_api_url)

    configuration = Application.get_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour)
    Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, enabled: false)

    Application.put_env(:explorer, :solc_bin_api_url, "http://localhost:#{bypass.port}")

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, configuration)
      Application.put_env(:tesla, :adapter, previous_adapter)
      Application.put_env(:explorer, :solc_bin_api_url, previous_solc_url)
    end)

    :ok
  end

  test "users validates smart contract", %{session: session} do
    %{name: name, source_code: source_code, bytecode: bytecode, tx_input: tx_input, version: version} =
      Factory.contract_code_info()

    address = insert(:address, contract_code: bytecode)

    transaction =
      insert(:transaction, input: tx_input)
      |> with_block(status: :ok)
      |> with_contract_creation(address)

    insert(
      :internal_transaction_create,
      created_contract_address: address,
      created_contract_code: bytecode,
      index: 0,
      transaction: transaction,
      transaction_index: transaction.index,
      block_number: transaction.block_number
    )

    session
    |> AddressContractPage.visit_page(address)
    |> AddressContractPage.click_verify_and_publish()
    |> ContractVerifyPage.fill_form(%{
      contract_name: name,
      version: version,
      optimization: false,
      source_code: source_code,
      evm_version: "byzantium"
    })
    |> ContractVerifyPage.verify_and_publish()

    wait_for_published_contract(session, address)
  end

  test "with invalid data shows error messages", %{session: session} do
    previous_max_wait_time = Application.get_env(:wallaby, :max_wait_time)
    Application.put_env(:wallaby, :max_wait_time, 30_000)

    on_exit(fn ->
      Application.put_env(:wallaby, :max_wait_time, previous_max_wait_time)
    end)

    address = insert(:address, contract_code: Factory.contract_code_info().bytecode)

    session
    |> ContractVerifyPage.visit_page(address.hash)
    |> ContractVerifyPage.fill_form(%{
      contract_name: "name",
      version: Factory.contract_code_info().version,
      optimization: true,
      source_code: "",
      evm_version: "byzantium"
    })
    |> then(fn session ->
      Process.sleep(2_000)
      session
    end)
    |> ContractVerifyPage.verify_and_publish()
    |> assert_has(ContractVerifyPage.validation_error())
    |> ContractVerifyPage.assert_message("Empty contract source code.")
  end

  defp wait_for_published_contract(session, address) do
    Process.sleep(10_000)
    AddressContractPage.visit_page(session, address)
    assert AddressContractPage.on_page?(session, address)
  end
end
