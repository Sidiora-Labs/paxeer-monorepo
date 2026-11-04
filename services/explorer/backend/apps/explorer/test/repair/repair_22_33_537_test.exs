Code.require_file("../explorer/migrator/switch_pending_operations_test.exs", __DIR__)

defmodule Explorer.Repair.Repair2233537Test do
  use ExUnit.Case, async: false

  alias Explorer.Migrator.PendingOperationsRestart

  setup do
    PendingOperationsRestart.setup()
  end

  test "pending-conversion-restart: recorded pending work reaches the real consumer and repeated startup is a no-op",
       context do
    PendingOperationsRestart.assert_uninterrupted_conversion(context)
  end
end
