defmodule NFTMediaHandler.Repair2233489Test do
  use ExUnit.Case, async: true

  alias NFTMediaHandler.Image.Resizer
  alias Vix.Vips.Image, as: VipsImage

  @moduletag :tmp_dir
  @readme Path.expand("../../README.md", __DIR__)
  @external_resource @readme
  @url "https://example.com/image.jpg"

  test "010 README unwraps the opened image and produces decodable thumbnails", %{tmp_dir: directory} do
    path = write_image(directory, 600)
    assert {:ok, %VipsImage{}} = VipsImage.new_from_file(path)

    assert {:ok, thumbnails} = documented_resize(path)
    assert Enum.map(thumbnails, &elem(&1, 0)) == [60, 250, 500]

    for {size, binary, name} <- thumbnails do
      assert name == Resizer.generate_file_name(@url, ".jpg", "#{size}x#{size}")
      assert <<0xFF, 0xD8, _::binary>> = binary
      assert {:ok, thumbnail} = VipsImage.new_from_buffer(binary)
      assert Image.width(thumbnail) == size
      assert Image.height(thumbnail) == size
    end
  end

  test "010 README handles a missing file before attempting resize", %{tmp_dir: directory} do
    path = Path.join(directory, "missing.jpg")
    refute File.exists?(path)
    assert {:error, reason} = VipsImage.new_from_file(path)
    assert documented_resize(path) == {:error, {:image_open_failed, reason}}
  end

  test "010 README handles an invalid image before attempting resize", %{tmp_dir: directory} do
    path = Path.join(directory, "invalid.jpg")
    File.write!(path, "invalid JPEG data")
    assert {:error, reason} = VipsImage.new_from_file(path)
    assert documented_resize(path) == {:error, {:image_open_failed, reason}}
  end

  test "010 README preserves the resizer's empty list for a small image", %{tmp_dir: directory} do
    path = write_image(directory, 32)
    assert documented_resize(path) == {:ok, []}
  end

  defp write_image(directory, size) do
    path = Path.join(directory, "image.jpg")
    {:ok, image} = Image.new(size, size, color: :red)
    {:ok, binary} = NFTMediaHandler.image_to_binary(image, "image.jpg", ".jpg")
    File.write!(path, binary)
    path
  end

  defp documented_resize(path) do
    [_, section] = String.split(File.read!(@readme), "### Resizing an Image", parts: 2)
    [_, example] = Regex.run(~r/```elixir\n(.*?)\n```/s, section)

    ast =
      example
      |> Code.string_to_quoted!()
      |> Macro.prewalk(fn
        "path/to/image.jpg" -> path
        node -> node
      end)

    {result, _bindings} = Code.eval_quoted(ast)
    result
  end
end
