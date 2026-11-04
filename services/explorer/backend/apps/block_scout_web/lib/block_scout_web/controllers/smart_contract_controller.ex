defmodule BlockScoutWeb.SmartContractController do
  use BlockScoutWeb, :controller

  alias BlockScoutWeb.{AccessHelper, AddressView}
  alias Explorer.Chain
  alias Explorer.Chain.{Address, SmartContract}
  alias Explorer.Chain.SmartContract.Proxy.Models.Implementation
  alias Explorer.SmartContract.{Reader, Writer}

  import Explorer.Helper, only: [parse_boolean: 1]
  import Explorer.Chain.SmartContract, only: [burn_address_hash_string: 0]

  def index(conn, %{"hash" => address_hash_string, "type" => contract_type, "action" => action} = params) do
    address_options = [
      necessity_by_association: %{
        :smart_contract => :optional
      },
      ip: AccessHelper.conn_to_ip_string(conn)
    ]

    is_custom_abi = parse_boolean(params["is_custom_abi"])

    with {:contract_interaction_disabled, false} <-
           {:contract_interaction_disabled, write_contract_api_disabled?(action)},
         true <- ajax?(conn),
         {:ok, address_hash} <- Chain.string_to_address_hash(address_hash_string),
         {:ok, false} <- AccessHelper.restricted_access?(address_hash_string, params),
         {:custom_abi, false} <- {:custom_abi, is_custom_abi},
         {:ok, address} <- Address.find_contract_address(address_hash, address_options) do
      implementation_address_hash_string = implementation_address_hash(contract_type, address)

      functions =
        if action == "write" do
          if contract_type == "proxy" do
            Writer.write_functions_proxy(implementation_address_hash_string)
          else
            Writer.write_functions(address.smart_contract)
          end
        else
          if contract_type == "proxy" do
            Reader.read_only_functions_proxy(address_hash, implementation_address_hash_string, nil)
          else
            Reader.read_only_functions(address.smart_contract, address_hash, params["from"])
          end
        end

      read_functions_required_wallet =
        if action == "read" do
          if contract_type == "proxy" do
            Reader.read_functions_required_wallet_proxy(implementation_address_hash_string)
          else
            Reader.read_functions_required_wallet(address.smart_contract)
          end
        else
          []
        end

      contract_abi = Poison.encode!(address.smart_contract.abi)

      implementation_abi =
        if contract_type == "proxy" do
          implementation_address_hash_string
          |> SmartContract.get_abi()
          |> Poison.encode!()
        else
          []
        end

      conn
      |> put_status(200)
      |> put_layout(false)
      |> render(
        "_functions.html",
        read_functions_required_wallet: read_functions_required_wallet,
        read_only_functions: functions,
        address: address,
        contract_abi: contract_abi,
        implementation_address: implementation_address_hash_string,
        implementation_abi: implementation_abi,
        contract_type: contract_type,
        action: action
      )
    else
      {:custom_abi, true} ->
        custom_abi_render(conn, params)

      :error ->
        unprocessable_entity(conn)

      {:contract_interaction_disabled, true} ->
        not_found(conn)

      {:error, :not_found} ->
        not_found(conn)

      _ ->
        not_found(conn)
    end
  end

  def index(conn, _), do: not_found(conn)

  defp implementation_address_hash(contract_type, address) do
    if contract_type == "proxy" do
      implementation = Implementation.get_implementation(address.smart_contract)
      (implementation && implementation.address_hashes |> List.first()) || burn_address_hash_string()
    else
      burn_address_hash_string()
    end
  end

  defp custom_abi_render(conn, %{"hash" => address_hash_string, "type" => contract_type, "action" => action} = params) do
    with {:ok, address_hash} <- Chain.string_to_address_hash(address_hash_string),
         {:ok, false} <- AccessHelper.restricted_access?(address_hash_string, params),
         custom_abi <- AddressView.fetch_custom_abi(conn, address_hash_string),
         false <- is_nil(custom_abi),
         abi <- custom_abi.abi do
      functions =
        if action == "write" do
          Writer.filter_write_functions(abi)
        else
          Reader.read_only_functions_from_abi_with_sender(abi, address_hash, params["from"])
        end

      read_functions_required_wallet =
        if action == "read" do
          Reader.read_functions_required_wallet_from_abi(abi)
        else
          []
        end

      contract_abi = Poison.encode!(abi)

      conn
      |> put_status(200)
      |> put_layout(false)
      |> render(
        "_functions.html",
        read_functions_required_wallet: read_functions_required_wallet,
        read_only_functions: functions,
        address: %{hash: address_hash},
        custom_abi: true,
        contract_abi: contract_abi,
        implementation_address: burn_address_hash_string(),
        implementation_abi: [],
        contract_type: contract_type,
        action: action
      )
    else
      :error ->
        unprocessable_entity(conn)

      _ ->
        not_found(conn)
    end
  end

  def show(conn, params) do
    address_options = [
      necessity_by_association: %{
        :names => :optional,
        :smart_contract => :optional,
        :token => :optional,
        Address.contract_creation_transaction_associations() => :optional
      },
      ip: AccessHelper.conn_to_ip_string(conn)
    ]

    with true <- ajax?(conn),
         {:ok, address_hash} <- Chain.string_to_address_hash(params["id"]),
         {:ok, false} <- AccessHelper.restricted_access?(params["id"], params),
         {:ok, address} <- Address.find_contract_address(address_hash, address_options),
         custom_abi <- custom_abi_for_request(conn, params),
         contract_type <- if(params["type"] == "proxy", do: :proxy, else: :regular),
         abi <- selected_abi(address, custom_abi, contract_type),
         {:args, {:ok, args}} <- {:args, read_args(params, abi)} do
      %{output: outputs, names: names} =
        if custom_abi do
          Reader.query_function_with_names_custom_abi(
            address_hash,
            %{method_id: params["method_id"], args: args},
            params["from"],
            custom_abi.abi
          )
        else
          Reader.query_function_with_names(
            address_hash,
            %{method_id: params["method_id"], args: args},
            contract_type,
            params["from"],
            address.smart_contract && address.smart_contract.abi,
            true
          )
        end

      conn
      |> put_status(200)
      |> put_layout(false)
      |> render(
        "_function_response.html",
        function_name: params["function_name"],
        method_id: params["method_id"],
        outputs: outputs,
        names: names,
        smart_contract_address: address_hash
      )
    else
      {:args, :error} ->
        unprocessable_entity(conn)

      :error ->
        unprocessable_entity(conn)

      {:error, :not_found} ->
        not_found(conn)

      _ ->
        not_found(conn)
    end
  end

  defp custom_abi_for_request(conn, params) do
    if parse_boolean(params["is_custom_abi"]), do: AddressView.fetch_custom_abi(conn, params["id"]), else: nil
  end

  defp selected_abi(_address, %{abi: abi}, _type), do: abi

  defp selected_abi(address, nil, :proxy),
    do: Explorer.Chain.SmartContract.Proxy.get_implementation_abi_from_proxy(address.smart_contract, [])

  defp selected_abi(address, nil, :regular), do: address.smart_contract && address.smart_contract.abi

  defp read_args(params, abi) when is_list(abi) do
    method =
      abi
      |> Reader.get_abi_with_method_id()
      |> Enum.find(&(&1["type"] == "function" and &1["method_id"] == params["method_id"]))

    with %{"inputs" => inputs} when is_list(inputs) <- method,
         expected = length(inputs),
         {:ok, args} <- collect_args(params, expected),
         true <- is_list(args) and length(args) == expected do
      {:ok, args}
    else
      _ -> :error
    end
  end

  defp read_args(_, _), do: :error

  defp collect_args(%{"args_count" => count} = params, expected) when is_binary(count) do
    with {:ok, ^expected} <- Explorer.Helper.safe_parse_non_negative_integer(count, expected) do
      if expected == 0 do
        {:ok, []}
      else
        Enum.reduce_while(0..(expected - 1), {:ok, []}, fn index, {:ok, args} ->
          with {:ok, value} <- Map.fetch(params, "arg_" <> to_string(index)),
               {:ok, value} <- convert_map_to_array(value) do
            {:cont, {:ok, [value | args]}}
          else
            _ -> {:halt, :error}
          end
        end)
        |> reverse_args()
      end
    else
      _ -> :error
    end
  end

  defp collect_args(%{"args_count" => _}, _), do: :error
  defp collect_args(params, _), do: convert_map_to_array(Map.get(params, "args") || [])

  defp convert_map_to_array(map) when is_map(map) do
    map
    |> Enum.reduce_while({:ok, []}, fn {key, value}, {:ok, values} ->
      case numeric_index(key) do
        {:ok, index} -> {:cont, {:ok, [{index, value} | values]}}
        _ -> {:halt, :error}
      end
    end)
    |> case do
      {:ok, values} ->
        sorted = Enum.sort_by(values, &elem(&1, 0))

        if Enum.with_index(sorted) |> Enum.all?(fn {{key, _}, index} -> key == index end) do
          sorted |> Enum.map(&elem(&1, 1)) |> convert_map_to_array()
        else
          :error
        end

      :error ->
        :error
    end
  end

  defp convert_map_to_array(values) when is_list(values) do
    Enum.reduce_while(values, {:ok, []}, fn value, {:ok, acc} ->
      case convert_map_to_array(value) do
        {:ok, converted} -> {:cont, {:ok, [converted | acc]}}
        :error -> {:halt, :error}
      end
    end)
    |> reverse_args()
  end

  defp convert_map_to_array(value), do: {:ok, value}
  defp reverse_args({:ok, args}), do: {:ok, Enum.reverse(args)}
  defp reverse_args(:error), do: :error

  defp numeric_index(index) when is_integer(index) and index >= 0, do: {:ok, index}
  defp numeric_index(index) when is_binary(index), do: Explorer.Helper.safe_parse_non_negative_integer(index)
  defp numeric_index(_), do: :error

  defp write_contract_api_disabled?(action), do: AddressView.contract_interaction_disabled?() && action == "write"
end
