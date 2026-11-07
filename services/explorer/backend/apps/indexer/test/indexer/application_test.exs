defmodule Indexer.ApplicationTest do
  use ExUnit.Case, async: false

  setup do
    standalone = Application.get_env(:nft_media_handler, :standalone_media_worker?)
    mode = Application.get_env(:explorer, :mode)
    indexer_supervisor = Application.get_env(:indexer, Indexer.Supervisor)
    relay_url = System.get_env("INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL")

    on_exit(fn ->
      Application.put_env(:nft_media_handler, :standalone_media_worker?, standalone)
      Application.put_env(:explorer, :mode, mode)
      Application.put_env(:indexer, Indexer.Supervisor, indexer_supervisor)

      if relay_url do
        System.put_env("INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL", relay_url)
      else
        System.delete_env("INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL")
      end
    end)

    :ok
  end

  test "indexer_children_ignore_kernel_receipts_env" do
    Application.put_env(:nft_media_handler, :standalone_media_worker?, false)
    Application.put_env(:explorer, :mode, :indexer)
    Application.put_env(:indexer, Indexer.Supervisor, enabled: false)
    System.put_env("INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL", "https://relay.invalid")

    assert Explorer.mode() == :indexer
    assert {:ok, supervisor} = Indexer.Application.start(:normal, [])

    try do
      assert Process.whereis(Indexer.Application) == supervisor
      children = Supervisor.which_children(supervisor)

      assert List.keymember?(children, Indexer.Memory.Monitor, 0)

      refute Enum.any?(children, fn {id, _pid, _type, modules} ->
               id == Indexer.Fetcher.PaxeerXKernelReceipts or Indexer.Fetcher.PaxeerXKernelReceipts in modules
             end)
    after
      :ok = Supervisor.stop(supervisor)
    end
  end
end
