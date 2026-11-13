# NFTMediaHandler

NFTMediaHandler fetches NFT media, generates thumbnails and uploads data to
an R2/S3-compatible bucket for the explorer. It processes PNG, JPEG, GIF and
WebP images and MP4 videos. The fetcher follows redirects and requests a
20,000,000-byte maximum response body. Unsupported media returns
`{:error, :unsupported_media_type}`; non-200 responses return
`{:error, status_code}` and HTTP failures return `{:error, reason}`.

The resizer produces 60, 250 and 500 pixel thumbnails when the source is larger
than the target. Each successful result is `{pixel_size, binary, file_name}`;
a skipped or failed size is omitted, so an empty list is possible. File names
are the lowercase SHA-1 of the source URL plus a size suffix and extension.
Images upload their original bytes as well as generated thumbnails. Videos
are written to temporary storage and their first frame generates JPEG
thumbnails; the video branch uploads thumbnails only.

This is explorer media processing for the merged Paxeer network. Its remote
mode uses distributed Erlang workers and the indexer's media queue; it does
not create a blockchain or a tenant domain. See
[EthereumJSONRPC](../ethereum_jsonrpc/README.md) for node access.

## Configuration

The umbrella's [runtime.exs](../../config/runtime.exs) maps these variables to
`:nft_media_handler` settings:

| Setting | Environment variable | Default |
| --- | --- | --- |
| `enabled?` | `NFT_MEDIA_HANDLER_ENABLED` | false |
| `remote?` | `NFT_MEDIA_HANDLER_REMOTE_DISPATCHER_NODE_MODE_ENABLED` | false |
| `worker?` | `NFT_MEDIA_HANDLER_IS_WORKER` | false |
| `worker_concurrency` | `NFT_MEDIA_HANDLER_WORKER_CONCURRENCY` | 10 |
| `worker_batch_size` | `NFT_MEDIA_HANDLER_WORKER_BATCH_SIZE` | 10 |
| `worker_spawn_tasks_timeout` | `NFT_MEDIA_HANDLER_WORKER_SPAWN_TASKS_TIMEOUT` | 100ms |
| `r2_folder` | `NFT_MEDIA_HANDLER_BUCKET_FOLDER` | nil; configure an object prefix |
| `cache_uniqueness_max_size` | `NFT_MEDIA_HANDLER_CACHE_UNIQUENESS_MAX_SIZE` | 100,000 |

`tmp_dir` is configured as `"./temp"` in runtime.exs, not read from an
`NFT_MEDIA_HANDLER_*` variable. The application creates this directory at
startup. Ensure it is writable and that any custom value suits the code's
string concatenation of the directory and file name.

When enabled in local mode, the supervisor starts the dispatcher and task
supervisor. Remote mode starts those workers only when `worker?` is true.
`standalone_media_worker?` is derived from all three flags being true and also
starts `DispatcherInterface`, which discovers indexer nodes and rotates among
them to retrieve work from `Indexer.NFTMediaHandler.Queue`. Distributed Erlang
connectivity and indexer discovery must be configured for that mode.

Uploads use `NFT_MEDIA_HANDLER_AWS_ACCESS_KEY_ID` and
`NFT_MEDIA_HANDLER_AWS_SECRET_ACCESS_KEY` through ExAws. The S3 configuration
uses `NFT_MEDIA_HANDLER_AWS_BUCKET_HOST`, `NFT_MEDIA_HANDLER_AWS_BUCKET_NAME`
and `NFT_MEDIA_HANDLER_AWS_PUBLIC_BUCKET_URL`, with HTTPS. Supply credentials
through the deployment environment. The configured `r2_folder` is the object
key prefix.

## Project Structure

- [lib/nft_media_handler/application.ex](lib/nft_media_handler/application.ex): supervision and startup modes.
- [lib/nft_media_handler.ex](lib/nft_media_handler.ex): URL normalization, image/video processing and upload orchestration.
- [lib/nft_media_handler/dispatcher.ex](lib/nft_media_handler/dispatcher.ex): concurrent task dispatch.
- [lib/nft_media_handler/dispatcher_interface.ex](lib/nft_media_handler/dispatcher_interface.ex): local or distributed queue access and result storage.
- [lib/nft_media_handler/image/resizer.ex](lib/nft_media_handler/image/resizer.ex): thumbnail generation and file names.
- [lib/nft_media_handler/media/fetcher.ex](lib/nft_media_handler/media/fetcher.ex): supported media detection and download.
- [lib/nft_media_handler/r2/uploader.ex](lib/nft_media_handler/r2/uploader.ex): ExAws object uploads.

## Usage Examples

Run these in the configured backend with its image/video dependencies and
bucket settings available. Supply `source_url` and `media_url` from the media being processed; the file path below is illustrative.

### Resizing an Image

`Vix.Vips.Image.new_from_file/1` returns a result tuple. Pass only the image
inside a successful `{:ok, image}` result to `NFTMediaHandler.Image.Resizer.resize/3`.
If opening fails, return the error without calling the resizer:

```elixir
case Vix.Vips.Image.new_from_file("path/to/image.jpg") do
  {:ok, image} ->
    thumbnails = NFTMediaHandler.Image.Resizer.resize(image, source_url, ".jpg")
    {:ok, thumbnails}

  {:error, reason} ->
    {:error, {:image_open_failed, reason}}
end
```

The successful wrapper contains the list of generated thumbnails. Resizing
can return `[]` for a small source or if all conversions fail; inspect the
list and logs before deciding what to upload.

### Uploading an Image

`upload_image/3` takes encoded bytes, a file name and a folder, not a Vix image
or an image-open tuple. With `image_binary` holding encoded JPEG bytes:

```elixir
case NFTMediaHandler.R2.Uploader.upload_image(image_binary, "image.jpg", "nft-media") do
  {:ok, result} -> {:ok, result}
  {:error, reason} -> {:error, {:upload_failed, reason}}
end
```

`upload_images/2` accepts the resizer's tuple list and stops on the first
upload error. An empty list returns `{:ok, nil}` without uploading an object.

### Fetching Media

`fetch_media/2` takes a URL and request-header list:

```elixir
case NFTMediaHandler.Media.Fetcher.fetch_media(media_url, []) do
  {:ok, media_type, body} -> {:ok, media_type, body}
  {:error, reason} -> {:error, {:media_fetch_failed, reason}}
end
```

`media_type` is a tuple such as `{"image", "jpeg"}`. For the full processing
path use `NFTMediaHandler.prepare_and_upload_by_url(url, folder)`. It also
normalizes supported IPFS and Arweave references using the metadata retriever.
It returns `{[file_path_pattern, uploaded_sizes, original_uploaded?], media_type}`
on success, or `{:error, reason}`. The file pattern contains `{}` for the size
suffix. The video result sets `original_uploaded?` to true; the uploaded video
objects are JPEG thumbnails.
