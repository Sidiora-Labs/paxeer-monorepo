defmodule Explorer.EnvVarTranslator do
  @moduledoc """
  The module for transformation of environment variables
  """

  alias Poison.Parser

  @spec map_array_env_var_to_list(atom()) :: [%{tag: String.t(), title: String.t()}]
  def map_array_env_var_to_list(config_name) do
    setting = config_name |> Atom.to_string() |> String.upcase()

    case Application.get_env(:block_scout_web, config_name) do
      nil ->
        []

      env_var when is_binary(env_var) ->
        env_var
        |> decode!(setting)
        |> validate_tags!(setting)

      _ ->
        raise ArgumentError, "#{setting} must be a JSON string containing a list of tag/title records"
    end
  end

  defp decode!(env_var, setting) do
    Parser.parse!(env_var)
  rescue
    Poison.ParseError ->
      raise ArgumentError, "#{setting} contains malformed JSON"
  end

  defp validate_tags!(tags, setting) when is_list(tags) do
    tags
    |> Enum.with_index()
    |> Enum.map(fn
      {%{"tag" => tag, "title" => title} = entry, index}
      when map_size(entry) == 2 and is_binary(tag) and is_binary(title) ->
        if String.trim(tag) == "" or String.trim(title) == "" do
          raise ArgumentError, "#{setting}[#{index}] tag and title must be non-empty strings"
        end

        %{tag: tag, title: title}

      {_, index} ->
        raise ArgumentError, "#{setting}[#{index}] must contain exactly tag and title as string fields"
    end)
  end

  defp validate_tags!(_, setting) do
    raise ArgumentError, "#{setting} must be a JSON list of tag/title records"
  end
end
