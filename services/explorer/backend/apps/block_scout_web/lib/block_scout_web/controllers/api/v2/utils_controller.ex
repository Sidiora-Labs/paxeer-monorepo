defmodule BlockScoutWeb.API.V2.UtilsController do
  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  alias BlockScoutWeb.API.V2.TransactionView
  alias Explorer.Chain
  alias Explorer.Chain.{Address, Data, SmartContract, Transaction}
  alias BlockScoutWeb.Schemas.API.V2.ErrorResponses.BadRequestResponse
  alias OpenApiSpex.Schema

  @api_true [api?: true]

  action_fallback(BlockScoutWeb.API.V2.FallbackController)

  tags(["utils"])

  operation :decode_calldata,
    summary: "Decode calldata using a verified contract or known method selector",
    parameters: [
      calldata: [in: :query, schema: Schemas.General.HexString],
      address_hash: [in: :query, schema: Schemas.General.AddressHash]
    ],
    responses: [
      ok:
        {"Decoded call, or null when the selector is unknown", "application/json",
         %Schema{
           type: :object,
           properties: %{result: %Schema{allOf: [Schemas.General.DecodedInput], nullable: true}},
           required: [:result],
           additionalProperties: false
         }},
      unprocessable_entity: BadRequestResponse.response()
    ]

  @doc """
    Function to handle GET and POST requests to `/api/v2/utils/decode-calldata`
  """
  @spec decode_calldata(Plug.Conn.t(), map()) :: {:format, :error} | Plug.Conn.t()
  def decode_calldata(conn, params) do
    with {:format, {:ok, data}} <- {:format, Data.cast(params["calldata"])},
         address_hash <- params["address_hash"] && Chain.string_to_address_hash(params["address_hash"]),
         {:format, true} <- {:format, match?({:ok, _hash}, address_hash) || is_nil(address_hash)} do
      smart_contract =
        if address_hash do
          {updated_smart_contract, _} =
            SmartContract.address_hash_to_smart_contract_with_bytecode_twin(elem(address_hash, 1), @api_true)

          updated_smart_contract
        end

      decoded_input =
        Transaction.decoded_input_data(
          %Transaction{
            input: data,
            to_address: %Address{contract_code: %Data{bytes: ""}, smart_contract: smart_contract}
          },
          @api_true
        )

      decoded_input_data = decoded_input |> Transaction.format_decoded_input() |> TransactionView.decoded_input()

      conn
      |> json(%{result: decoded_input_data})
    end
  end
end
