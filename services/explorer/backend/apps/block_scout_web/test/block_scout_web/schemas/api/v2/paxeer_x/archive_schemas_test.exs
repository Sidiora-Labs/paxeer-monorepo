defmodule BlockScoutWeb.Schemas.API.V2.PaxeerX.ArchiveSchemasTest do
  use ExUnit.Case, async: true

  alias BlockScoutWeb.Schemas.API.V2.PaxeerX.{ArchiveAnchor, ArchiveReceipt}
  alias OpenApiSpex.{Cast, Components, Info, OpenApi}

  test "archive_schemas_require_frozen_marker" do
    archive_schemas_require_frozen_marker()
  end

  def archive_schemas_require_frozen_marker do
    hash = "0x" <> String.duplicate("1", 64)
    timestamp = "2026-01-01T00:00:00Z"

    anchor = %{
      "batch_number" => 1,
      "checkpoint_id" => hash,
      "checkpoint_height" => 100,
      "sealed_height" => 99,
      "state_root" => hash,
      "block_number" => 10,
      "timestamp" => timestamp,
      "frozen" => true
    }

    receipt = %{
      "id" => hash,
      "account" => nil,
      "block_number" => 10,
      "origin" => "evm",
      "verification" => "batch_included",
      "provenance" => %{
        "evm" => %{
          "transaction_hash" => hash,
          "log_index" => 0,
          "block_hash" => hash,
          "block_number" => 10
        }
      },
      "verification_status" => "batch_included",
      "payload_hash" => nil,
      "transaction_hash" => hash,
      "timestamp" => timestamp,
      "frozen" => true
    }

    spec =
      %OpenApi{
        info: %Info{title: "Archive schemas", version: "1.0"},
        paths: %{},
        components: %Components{schemas: %{}}
      }
      |> OpenApiSpex.add_schemas([ArchiveAnchor, ArchiveReceipt])

    for {module, body} <- [{ArchiveAnchor, anchor}, {ArchiveReceipt, receipt}] do
      schema = Map.fetch!(spec.components.schemas, module.schema().title)
      schemas = spec.components.schemas

      assert {:ok, item} = Cast.cast(schema, body, schemas)
      assert item.__struct__ == module
      assert item.frozen == true
      assert {:error, _} = Cast.cast(schema, Map.delete(body, "frozen"), schemas)

      for invalid_marker <- [false, nil, "false"] do
        assert {:error, _} = Cast.cast(schema, Map.put(body, "frozen", invalid_marker), schemas)
      end

      for field <- schema.required do
        assert {:error, _} = Cast.cast(schema, Map.delete(body, Atom.to_string(field)), schemas)
      end
    end

    receipt_schema = Map.fetch!(spec.components.schemas, "PaxeerXArchiveReceipt")

    refute Map.has_key?(receipt_schema.properties, :status)
    refute :status in receipt_schema.required

    assert {:error, _} =
             Cast.cast(receipt_schema, Map.put(receipt, "status", "batch_included"), spec.components.schemas)
  end
end
