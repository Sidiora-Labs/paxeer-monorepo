defmodule BlockScoutWeb.API.V2.PaxeerX.ArchiveController do
  use BlockScoutWeb, :controller
  use OpenApiSpex.ControllerSpecs

  import BlockScoutWeb.Chain, only: [next_page_params: 5, split_list_by_page: 1]
  import Ecto.Query
  import Explorer.PagingOptions, only: [default_paging_options: 0]

  alias BlockScoutWeb.Schemas.API.V2.ErrorResponses.NotFoundResponse
  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.Parameters
  alias Explorer.Chain
  alias Explorer.Chain.Hash
  alias Explorer.Chain.PaxeerX.{Anchor, Receipt}
  alias Explorer.Helper, as: ExplorerHelper

  action_fallback(BlockScoutWeb.API.V2.FallbackController)

  @api_true [api?: true]

  tags(["paxeer-x"])

  operation :anchors,
    summary: "List frozen anchor checkpoints",
    parameters:
      base_params() ++ Parameters.define_paging_params(["batch_number"]) ++ define_paging_params(["items_count"]),
    responses: [
      ok:
        {"Frozen anchors with pagination.", "application/json",
         paginated_response(
           items: Schemas.PaxeerX.ArchiveAnchor,
           next_page_params_example: %{"batch_number" => 2, "items_count" => 50}
         )}
    ]

  def anchors(conn, params) do
    query =
      from(anchor in Anchor.only_consensus_query(),
        distinct: [desc: anchor.batch_number],
        order_by: [desc: anchor.block_number, desc: anchor.log_index],
        limit: ^default_paging_options().page_size,
        select: %{
          batch_number: anchor.batch_number,
          checkpoint_id: anchor.checkpoint_id,
          checkpoint_height: anchor.kernel_height,
          sealed_height: anchor.sealed_height,
          state_root: anchor.state_root,
          block_number: anchor.block_number,
          timestamp: as(:block).timestamp
        }
      )

    query =
      case anchor_paging_key(params) do
        nil -> query
        batch_number -> where(query, [anchor], anchor.batch_number < ^batch_number)
      end

    {anchors, next_page} = query |> Chain.select_repo(@api_true).all() |> split_list_by_page()

    render(conn, :anchors, %{
      anchors: anchors,
      next_page_params: next_page_params(next_page, anchors, params, false, &anchor_paging_params/1)
    })
  end

  operation :receipts,
    summary: "List frozen receipts",
    parameters:
      base_params() ++ Parameters.define_paging_params(["receipt_id"]) ++ define_paging_params(["items_count"]),
    responses: [
      ok:
        {"Frozen receipts with pagination.", "application/json",
         paginated_response(
           items: Schemas.PaxeerX.ArchiveReceipt,
           next_page_params_example: %{
             "id" => "0x0000000000000000000000000000000000000000000000000000000000000065",
             "items_count" => 50
           }
         )}
    ]

  def receipts(conn, params) do
    {receipts, next_page} =
      params
      |> receipt_paging_key()
      |> Receipt.public_page(default_paging_options().page_size)
      |> split_list_by_page()

    render(conn, :receipts, %{
      receipts: receipts,
      next_page_params: next_page_params(next_page, receipts, params, false, &receipt_paging_params/1)
    })
  end

  operation :receipt,
    summary: "Retrieve a frozen receipt",
    parameters: [Parameters.receipt_id_param() | base_params()],
    responses: [
      ok: {"The frozen receipt.", "application/json", Schemas.PaxeerX.ArchiveReceipt},
      not_found: NotFoundResponse.response()
    ]

  def receipt(conn, %{"id" => id}) do
    with {:ok, receipt_id} <- Hash.Full.cast(id),
         %Receipt{} = receipt <- Receipt.public_get(receipt_id) do
      render(conn, :receipt, %{receipt: receipt})
    else
      _missing -> {:error, :not_found}
    end
  end

  defp anchor_paging_key(%{"batch_number" => batch_number}) do
    case ExplorerHelper.safe_parse_non_negative_integer(batch_number) do
      {:ok, parsed} -> parsed
      _invalid -> nil
    end
  end

  defp anchor_paging_key(_params), do: nil

  defp receipt_paging_key(%{"id" => id}) when is_binary(id) do
    case Hash.Full.cast(id) do
      {:ok, receipt_id} -> receipt_id
      :error -> nil
    end
  end

  defp receipt_paging_key(_params), do: nil

  defp anchor_paging_params(%{batch_number: batch_number}), do: %{batch_number: batch_number}

  defp receipt_paging_params(%Receipt{receipt_id: receipt_id}), do: %{id: to_string(receipt_id)}
end
