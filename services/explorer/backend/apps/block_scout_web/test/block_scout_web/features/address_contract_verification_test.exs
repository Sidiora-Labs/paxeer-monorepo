defmodule BlockScoutWeb.AddressContractVerificationTest do
  use BlockScoutWeb.FeatureCase, async: false

  alias BlockScoutWeb.{AddressContractPage, ContractVerifyPage}
  alias Explorer.Factory
  alias Plug.Conn

  setup do
    bypass = Bypass.open()

    configuration = Application.get_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour)
    Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, enabled: false)

    Application.put_env(:explorer, :solc_bin_api_url, "http://localhost:#{bypass.port}")

    Application.put_env(:tesla, :adapter, Tesla.Adapter.Mint)

    on_exit(fn ->
      Application.put_env(:explorer, Explorer.SmartContract.RustVerifierInterfaceBehaviour, configuration)
      Application.put_env(:tesla, :adapter, Explorer.Mock.TeslaAdapter)
    end)

    {:ok, bypass: bypass}
  end

  test "users validates smart contract", %{session: session, bypass: bypass} do
    Bypass.expect(bypass, fn conn -> Conn.resp(conn, 200, solc_bin_versions()) end)

    %{name: name, source_code: source_code, bytecode: bytecode, version: version} = Factory.contract_code_info()

    transaction = :transaction |> insert() |> with_block()
    address = insert(:address, contract_code: bytecode)

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

    assert AddressContractPage.on_page?(session, address)
  end

  test "with invalid data shows error messages", %{session: session, bypass: bypass} do
    Bypass.expect(bypass, fn conn -> Conn.resp(conn, 200, solc_bin_versions()) end)

    address = insert(:address, contract_code: Factory.contract_code_info().bytecode)

    has_message =
      session
      |> ContractVerifyPage.visit_page(address.hash)
      |> ContractVerifyPage.fill_form(%{
        contract_name: "name",
        version: Factory.contract_code_info().version,
        optimization: true,
        source_code: "",
        evm_version: "byzantium"
      })
      |> ContractVerifyPage.verify_and_publish()
      |> ContractVerifyPage.has_message?("Empty contract source code.")

    assert has_message
  end

  defp solc_bin_versions do
    File.read!("./test/support/fixture/smart_contract/solc_bin.json")
  end
end
