defmodule BlockScoutWeb.API.V2.PaxeerX.ArchiveControllerTest do
  use BlockScoutWeb.ConnCase

  alias Explorer.Chain.PaxeerX.{Anchor, Receipt}
  alias Explorer.Repo

  test "archive_anchors_lists_frozen_rows", context do
    archive_anchors_lists_frozen_rows(context)
  end

  defp archive_anchors_lists_frozen_rows(%{conn: conn}) do
    assert json_response(get(conn, "/api/v2/paxeer-x/archive/anchors"), 200) == %{
             "items" => [],
             "next_page_params" => nil
           }

    for batch_number <- 1..51, do: insert_anchor(batch_number)
    latest = insert_anchor(51, block_number: 600, sealed_height: 5_200, status: :final)
    insert_anchor(52, consensus: false)

    response = json_response(get(conn, "/api/v2/paxeer-x/archive/anchors"), 200)

    assert Enum.map(response["items"], & &1["batch_number"]) == Enum.to_list(51..2)
    assert Enum.all?(response["items"], &(&1["frozen"] == true))
    assert response["next_page_params"] == %{"batch_number" => 2, "items_count" => 50}

    [anchor | _rest] = response["items"]
    assert anchor["checkpoint_id"] == hash(51)
    assert anchor["checkpoint_height"] == 5_100
    assert anchor["sealed_height"] == 5_200
    assert anchor["state_root"] == to_string(latest.state_root)
    assert anchor["block_number"] == 600
    refute is_nil(anchor["timestamp"])
    refute Map.has_key?(anchor, "status")

    page = json_response(get(conn, "/api/v2/paxeer-x/archive/anchors", response["next_page_params"]), 200)
    assert [%{"batch_number" => 1, "frozen" => true}] = page["items"]
    assert page["next_page_params"] == nil
  end

  test "archive_receipts_lists_frozen_rows", context do
    archive_receipts_lists_frozen_rows(context)
  end

  defp archive_receipts_lists_frozen_rows(%{conn: conn}) do
    assert json_response(get(conn, "/api/v2/paxeer-x/archive/receipts"), 200) == %{
             "items" => [],
             "next_page_params" => nil
           }

    for index <- 1..50, do: insert_receipt(index)
    latest = insert_receipt(50, block_number: 600, status: :state_proven)
    kernel = insert_kernel_receipt(51)
    insert_receipt(52, consensus: false)

    response = json_response(get(conn, "/api/v2/paxeer-x/archive/receipts"), 200)

    assert Enum.map(response["items"], & &1["id"]) == Enum.map(51..2, &hash/1)
    assert Enum.all?(response["items"], &(&1["frozen"] == true))
    assert Enum.all?(response["items"], &(not Map.has_key?(&1, "status")))
    assert response["next_page_params"] == %{"id" => hash(2), "items_count" => 50}

    [kernel_item, evm_item | _rest] = response["items"]
    assert kernel_item["origin"] == "kernel"
    assert kernel_item["verification"] == "sequencer_verified"
    assert kernel_item["verification_status"] == "batch_included"
    assert kernel_item["block_number"] == nil
    assert kernel_item["transaction_hash"] == nil
    assert kernel_item["timestamp"] == nil
    assert kernel_item["payload_hash"] == to_string(kernel.payload_hash)
    assert kernel_item["provenance"]["kernel"]["batch_number"] == "51"
    assert kernel_item["provenance"]["kernel"]["activity_id"] == hash(51)
    assert kernel_item["provenance"]["kernel"]["proof"]["signature_hex"] == String.duplicate("cd", 64)

    assert evm_item["origin"] == "evm"
    assert evm_item["account"] == hash(99)
    assert evm_item["verification"] == "state_proven"
    assert evm_item["verification_status"] == "state_proven"
    assert evm_item["payload_hash"] == to_string(latest.payload_hash)
    assert evm_item["transaction_hash"] == to_string(latest.transaction_hash)
    assert evm_item["provenance"]["evm"]["block_hash"] == to_string(latest.block_hash)
    assert evm_item["block_number"] == 600
    refute is_nil(evm_item["timestamp"])

    assert json_response(get(conn, "/api/v2/paxeer-x/archive/receipts/#{hash(51)}"), 200) == kernel_item
    assert json_response(get(conn, "/api/v2/paxeer-x/archive/receipts/#{hash(50)}"), 200) == evm_item

    page = json_response(get(conn, "/api/v2/paxeer-x/archive/receipts", response["next_page_params"]), 200)
    assert [%{"id" => id, "frozen" => true}] = page["items"]
    assert id == hash(1)
    refute Map.has_key?(hd(page["items"]), "status")
    assert page["next_page_params"] == nil
  end

  test "archive_receipt_404_for_unknown_id", context do
    archive_receipt_404_for_unknown_id(context)
  end

  defp archive_receipt_404_for_unknown_id(%{conn: conn}) do
    insert_receipt(1)

    assert json_response(get(conn, "/api/v2/paxeer-x/archive/receipts/#{hash(2)}"), 404) == %{
             "message" => "Not found"
           }

    assert json_response(get(conn, "/api/v2/paxeer-x/archive/receipts/invalid"), 404) == %{
             "message" => "Not found"
           }
  end

  defp insert_anchor(batch_number, overrides \\ []) do
    overrides = Map.new(overrides)

    block =
      insert(:block,
        number: Map.get(overrides, :block_number, batch_number * 10),
        consensus: true
      )

    transaction = :transaction |> insert() |> with_block(block)
    block = block |> Ecto.Changeset.change(consensus: Map.get(overrides, :consensus, true)) |> Repo.update!()

    attrs =
      Map.merge(
        %{
          transaction_hash: transaction.hash,
          log_index: 0,
          block_hash: block.hash,
          block_number: block.number,
          block_consensus: block.consensus,
          batch_number: batch_number,
          checkpoint_id: hash(batch_number),
          state_root: to_string(block_hash()),
          status: :submitted,
          kernel_height: batch_number * 100,
          sealed_height: batch_number * 100 - 1
        },
        Map.drop(overrides, [:block_number, :consensus])
      )

    %Anchor{}
    |> Ecto.Changeset.cast(attrs, Map.keys(attrs))
    |> Repo.insert!()
  end

  defp insert_receipt(index, overrides \\ []) do
    overrides = Map.new(overrides)

    block =
      insert(:block,
        number: Map.get(overrides, :block_number, index * 10),
        consensus: true
      )

    transaction = :transaction |> insert() |> with_block(block)
    block = block |> Ecto.Changeset.change(consensus: Map.get(overrides, :consensus, true)) |> Repo.update!()

    attrs =
      Map.merge(
        %{
          transaction_hash: transaction.hash,
          log_index: 0,
          block_hash: block.hash,
          block_number: block.number,
          block_consensus: block.consensus,
          receipt_id: hash(index),
          account: hash(99),
          payload_hash: to_string(block_hash()),
          status: :sequencer_signed
        },
        Map.drop(overrides, [:block_number, :consensus])
      )

    %Receipt{}
    |> Ecto.Changeset.cast(attrs, Map.keys(attrs))
    |> Repo.insert!()
  end

  defp insert_kernel_receipt(index) do
    attrs = %{
      origin: :kernel,
      receipt_id: hash(index),
      account: hash(99),
      payload_hash: hash(100),
      status: :batch_included,
      kernel_batch_number: index,
      kernel_batch_id: hash(101),
      kernel_sequence: 1,
      kernel_activity_id: hash(index),
      kernel_result_code: 0,
      kernel_receipt_sha256: hash(102),
      kernel_canonical_sha256: hash(103),
      kernel_batch_raw_sha256: hash(104),
      kernel_state_root: hash(105),
      kernel_proof: %{
        "batch_id" => String.trim_leading(hash(101), "0x"),
        "header_hex" => "0a0b",
        "signature_hex" => String.duplicate("cd", 64),
        "raw_sha256" => String.trim_leading(hash(104), "0x")
      },
      kernel_verification: :sequencer_verified
    }

    %Receipt{}
    |> Ecto.Changeset.cast(attrs, Map.keys(attrs))
    |> Repo.insert!()
  end

  defp hash(index), do: "0x" <> Base.encode16(<<index::256>>, case: :lower)
end
