defmodule Explorer.SmartContract.SolcDownloader do
  @moduledoc """
  Checks to see if the requested solc compiler version exists, and if not it
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
        case CompilerVersion.fetch_versions(:solc) do
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
    case CompilerDownload.prepare_directory(compiler_dir(), "*-tmp.js") do
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
             :ok <- CompilerDownload.publish(path, contents, false) do
          path
        else
          {:error, reason} ->
            Logger.error("Failed to install solc compiler #{version}: #{inspect(reason)}")
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
    Path.join(compiler_dir(), "#{version}.js")
  end

  defp compiler_dir do
    Application.app_dir(:explorer, "priv/solc_compilers/")
  end

  defp download(version) do
    base_url = Application.fetch_env!(:explorer, :solc_bin_api_url)

    with {:ok, metadata} <- CompilerDownload.get("#{base_url}/bin/list.json"),
         {:ok, %{"builds" => builds} = distribution} when is_list(builds) <- Jason.decode(metadata),
         build when is_map(build) <- find_build(builds, distribution, version),
         %{"path" => artifact_path, "sha256" => digest} <- build,
         true <- is_binary(artifact_path) and Path.basename(artifact_path) == artifact_path,
         {:ok, body} <- CompilerDownload.get("#{base_url}/bin/#{artifact_path}"),
         :ok <- CompilerDownload.sha256(body, digest) do
      {:ok, body}
    else
      {:error, _} = error -> error
      _ -> {:error, :invalid_distribution_metadata}
    end
  end

  defp find_build(builds, distribution, "latest") do
    path = get_in(distribution, ["releases", distribution["latestRelease"]])
    Enum.find(builds, &(&1["path"] == path))
  end

  defp find_build(builds, _distribution, version) do
    Enum.find(builds, &(&1["path"] == "soljson-#{version}.js"))
  end
end
