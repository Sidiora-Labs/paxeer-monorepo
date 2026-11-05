defmodule BlockScoutWeb.API.V2.PaxeerX.ArchiveView do
  use BlockScoutWeb, :view

  alias Explorer.Chain
  alias Explorer.Chain.{Block, Hash}
  alias Explorer.Chain.PaxeerX.Receipt

  def render("anchors.json", %{anchors: anchors, next_page_params: next_page_params}) do
    %{"items" => Enum.map(anchors, &prepare_anchor/1), "next_page_params" => next_page_params}
  end

  def render("receipts.json", %{receipts: receipts, next_page_params: next_page_params}) do
    %{"items" => Enum.map(receipts, &prepare_receipt/1), "next_page_params" => next_page_params}
  end

  def render("receipt.json", %{receipt: receipt}), do: prepare_receipt(receipt)

  defp prepare_anchor(anchor) do
    %{
      "batch_number" => anchor.batch_number,
      "checkpoint_id" => to_string(anchor.checkpoint_id),
      "checkpoint_height" => anchor.checkpoint_height,
      "sealed_height" => anchor.sealed_height,
      "state_root" => hex(anchor.state_root),
      "block_number" => anchor.block_number,
      "timestamp" => anchor.timestamp,
      "frozen" => true
    }
  end

  defp prepare_receipt(%Receipt{} = receipt) do
    %{
      "id" => to_string(receipt.receipt_id),
      "account" => hex(receipt.account),
      "block_number" => receipt.block_number,
      "origin" => to_string(receipt.origin),
      "verification" => verification(receipt),
      "provenance" => provenance(receipt),
      "verification_status" => to_string(receipt.status),
      "payload_hash" => hex(receipt.payload_hash),
      "transaction_hash" => hex(receipt.transaction_hash),
      "timestamp" => timestamp(receipt),
      "frozen" => true
    }
  end

  defp verification(%Receipt{origin: :kernel, kernel_verification: verification}), do: to_string(verification)
  defp verification(%Receipt{origin: :evm, status: status}), do: to_string(status)

  defp provenance(%Receipt{origin: :kernel} = receipt) do
    %{
      "kernel" => %{
        "batch_number" => to_string(receipt.kernel_batch_number),
        "batch_id" => hex(receipt.kernel_batch_id),
        "sequence" => to_string(receipt.kernel_sequence),
        "activity_id" => hex(receipt.kernel_activity_id),
        "result_code" => receipt.kernel_result_code,
        "receipt_sha256" => hex(receipt.kernel_receipt_sha256),
        "canonical_sha256" => hex(receipt.kernel_canonical_sha256),
        "batch_raw_sha256" => hex(receipt.kernel_batch_raw_sha256),
        "state_root" => hex(receipt.kernel_state_root),
        "proof" => %{
          "header_hex" => receipt.kernel_proof["header_hex"],
          "signature_hex" => receipt.kernel_proof["signature_hex"]
        }
      }
    }
  end

  defp provenance(%Receipt{origin: :evm} = receipt) do
    %{
      "evm" => %{
        "transaction_hash" => hex(receipt.transaction_hash),
        "log_index" => receipt.log_index,
        "block_hash" => hex(receipt.block_hash),
        "block_number" => receipt.block_number
      }
    }
  end

  defp timestamp(%Receipt{origin: :evm, block_hash: %Hash{} = block_hash}) do
    case Chain.hash_to_block(block_hash, api?: true) do
      {:ok, %Block{timestamp: timestamp}} -> timestamp
      _missing -> nil
    end
  end

  defp timestamp(%Receipt{}), do: nil

  defp hex(nil), do: nil
  defp hex(value), do: to_string(value)
end
