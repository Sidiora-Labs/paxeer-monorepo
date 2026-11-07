defmodule Explorer.SmartContract.CompilerDownload do
  @moduledoc false

  alias Explorer.HttpClient

  def get(url) do
    case HttpClient.get(url, [], timeout: 60_000, recv_timeout: 60_000, follow_redirect: true) do
      {:ok, %{status_code: status, body: body}} when status in 200..299 and is_binary(body) -> {:ok, body}
      {:ok, %{status_code: status}} -> {:error, {:http_status, status}}
      {:error, reason} -> {:error, reason}
    end
  end

  def sha256(body, "0x" <> digest), do: sha256(body, digest)
  def sha256(body, "sha256:" <> digest), do: sha256(body, digest)

  def sha256(body, digest) when is_binary(digest) do
    case Base.decode16(digest, case: :mixed) do
      {:ok, expected} when byte_size(expected) == 32 ->
        if :crypto.hash(:sha256, body) == expected, do: :ok, else: {:error, :checksum_mismatch}

      _ ->
        {:error, :invalid_checksum}
    end
  end

  def sha256(_, _), do: {:error, :missing_checksum}

  def prepare_directory(directory, legacy_pattern) do
    with :ok <- File.mkdir_p(directory) do
      (Path.wildcard(Path.join(directory, legacy_pattern)) ++ Path.wildcard(Path.join(directory, "*.download")))
      |> Enum.reduce_while(:ok, fn path, :ok ->
        case remove(path) do
          :ok -> {:cont, :ok}
          error -> {:halt, error}
        end
      end)
    end
  end

  def publish(path, body, executable?) do
    temporary = path <> ".download"

    with :ok <- remove(temporary) do
      result =
        with {:ok, file} <- File.open(temporary, [:write, :binary, :exclusive]) do
          written = with :ok <- IO.binwrite(file, body), do: :file.sync(file)
          closed = File.close(file)

          with :ok <- written,
               :ok <- closed,
               :ok <- set_mode(temporary, executable?),
               :ok <- File.rename(temporary, path) do
            :ok
          end
        end

      case remove(temporary) do
        :ok -> result
        error -> error
      end
    end
  end

  defp set_mode(path, true), do: File.chmod(path, 0o755)
  defp set_mode(_path, false), do: :ok

  defp remove(path) do
    case File.rm(path) do
      :ok -> :ok
      {:error, :enoent} -> :ok
      error -> error
    end
  end
end
