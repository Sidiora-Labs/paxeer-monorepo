defmodule Explorer.SmartContract.VyperDownloader do
  @moduledoc """
  Checks to see if the requested Vyper compiler version exists, and if not it
  downloads and stores the file.
  """
  use GenServer

  require Logger

  alias Explorer.SmartContract.{CompilerDownload, CompilerVersion}

  @latest_compiler_refetch_time :timer.minutes(30)

  def ensure_exists(version) do
    path = file_path(version)

    if File.exists?(path) && version !== "latest" do
      path
    else
      compiler_versions =
        case CompilerVersion.fetch_versions(:vyper) do
          {:ok, compiler_versions} ->
            compiler_versions

          {:error, _} ->
            []
        end

      if version in compiler_versions do
        GenServer.call(__MODULE__, {:ensure_exists, version}, 60_000)
      else
        false
      end
    end
  end

  def start_link(_) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  # sobelow_skip ["Traversal"]
  @impl true
  def init([]) do
    case CompilerDownload.prepare_directory(compiler_dir(), "*-tmp") do
      :ok -> {:ok, []}
      {:error, reason} -> {:stop, reason}
    end
  end

  # sobelow_skip ["Traversal"]
  @impl true
  def handle_call({:ensure_exists, version}, _from, state) do
    path = file_path(version)

    result =
      if fetch?(version, path) do
        with {:ok, contents} <- download(version),
             :ok <- CompilerDownload.publish(path, contents, true) do
          path
        else
          {:error, reason} ->
            Logger.error("Failed to install vyper compiler #{version}: #{inspect(reason)}")
            false
        end
      else
        path
      end

    {:reply, result, state}
  end

  defp fetch?("latest", path) do
    case File.stat(path) do
      {:error, _reason} ->
        true

      {:ok, %{mtime: mtime}} ->
        last_modified = NaiveDateTime.from_erl!(mtime)
        diff = Timex.diff(NaiveDateTime.utc_now(), last_modified, :milliseconds)

        diff > @latest_compiler_refetch_time
    end
  end

  defp fetch?(_, path) do
    not File.exists?(path)
  end

  defp file_path(version) do
    Path.join(compiler_dir(), "#{version}")
  end

  defp compiler_dir do
    Application.app_dir(:explorer, "priv/vyper_compilers/")
  end

  defp download(version) do
    version = CompilerVersion.get_strict_compiler_version(:vyper, version)

    with {:ok, metadata} <- CompilerDownload.get(CompilerVersion.vyper_releases_url()),
         {:ok, releases} when is_list(releases) <- Jason.decode(metadata),
         release when is_map(release) <- Enum.find(releases, &(&1["tag_name"] == version)),
         assets when is_list(assets) <- release["assets"],
         asset when is_map(asset) <- Enum.find(assets, &linux_asset?/1),
         {:ok, body} <- CompilerDownload.get(asset["browser_download_url"]),
         :ok <- validate_artifact(body, asset) do
      {:ok, body}
    else
      {:error, _} = error -> error
      _ -> {:error, :invalid_release_metadata}
    end
  end

  defp linux_asset?(%{"browser_download_url" => url}) when is_binary(url), do: String.contains?(url, "linux")
  defp linux_asset?(_), do: false

  defp validate_artifact(body, %{"size" => size} = asset) when is_integer(size) and size > 0 do
    if byte_size(body) == size do
      case asset["digest"] do
        nil -> :ok
        "sha256:" <> _ = digest -> CompilerDownload.sha256(body, digest)
        _ -> {:error, :unsupported_checksum}
      end
    else
      {:error, :size_mismatch}
    end
  end

  defp validate_artifact(_, _), do: {:error, :missing_artifact_size}
end
