defmodule BlockScoutWeb.ContractVerifyPage do
  @moduledoc false

  use Wallaby.DSL

  import Wallaby.Query

  def visit_page(session, address_hash) do
    visit(session, "/address/#{Explorer.Chain.Address.checksum(address_hash)}/verify-via-flattened-code/new")
  end

  def fill_form(session, %{
        contract_name: contract_name,
        version: version,
        optimization: optimization,
        source_code: source_code,
        evm_version: evm_version
      }) do
    if current_path(session) =~ "/contract_verifications/new" do
      click(session, css("#verify_via_flattened_code_button"))
    end

    session
    |> fill_in(css("[data-test='contract_name']"), with: contract_name)
    |> fill_in(text_field("Enter the Solidity Contract Code"), with: source_code)

    case version do
      nil -> nil
      _ -> session |> find(select("Compiler")) |> click(option(version))
    end

    case evm_version do
      nil -> nil
      _ -> session |> find(select("EVM Version")) |> click(option(evm_version))
    end

    case optimization do
      true ->
        click(session, css("label[for='smart_contract_optimization_true']"))

      false ->
        click(session, css("label[for='smart_contract_optimization_false']"))

      _ ->
        nil
    end

    session
  end

  def validation_error do
    css("[data-test='contract-source-code-error']")
  end

  def has_message?(session, message) do
    has_text?(session, message)
  end

  def verify_and_publish(session) do
    click(session, button("Verify & publish"))
  end
end
