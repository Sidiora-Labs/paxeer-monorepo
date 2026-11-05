defmodule Explorer.Chain.PaxeerX.ArchiveReadonlyTest do
  use ExUnit.Case, async: true

  alias Explorer.Chain.PaxeerX.{Anchor, Receipt}

  test "archive_schemas_export_no_changeset" do
    archive_schemas_export_no_changeset()
  end

  defp archive_schemas_export_no_changeset do
    Code.ensure_loaded!(Anchor)
    Code.ensure_loaded!(Receipt)

    refute function_exported?(Anchor, :changeset, 2)
    refute function_exported?(Receipt, :changeset, 2)
  end
end
