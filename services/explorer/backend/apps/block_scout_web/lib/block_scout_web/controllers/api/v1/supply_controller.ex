defmodule BlockScoutWeb.API.V1.SupplyController do
  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  operation :supply,
    summary: "Total and circulating coin supply",
    responses: [
      ok:
        {"Coin supply", "application/json",
         %OpenApiSpex.Schema{
           type: :object,
           properties: %{
             total_supply: %OpenApiSpex.Schema{
               oneOf: [
                 %OpenApiSpex.Schema{type: :integer, minimum: 0},
                 %OpenApiSpex.Schema{type: :string, pattern: ~r/^\d+(\.\d+)?$/}
               ]
             },
             circulating_supply: %OpenApiSpex.Schema{
               nullable: true,
               oneOf: [
                 %OpenApiSpex.Schema{type: :integer, minimum: 0},
                 %OpenApiSpex.Schema{type: :string, pattern: ~r/^\d+(\.\d+)?$/}
               ]
             }
           },
           required: [:total_supply, :circulating_supply],
           additionalProperties: false
         }}
    ]

  alias Explorer.Chain

  def supply(conn, _) do
    total_supply = Chain.total_supply()
    circulating_supply = Chain.circulating_supply()

    render(conn, :supply, total: total_supply, circulating: circulating_supply)
  end
end
