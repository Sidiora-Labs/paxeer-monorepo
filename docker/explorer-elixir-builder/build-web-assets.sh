#!/bin/sh
set -eu

assets_dir=/app/apps/block_scout_web/assets
cache_dir=$MIX_BUILD_PATH/web-assets/$MIX_ENV
export BLOCKSCOUT_STATIC_DIR=$cache_dir/static

assets_digest() {
  (
    cd "$assets_dir"
    find . -type f ! -path './node_modules/*' ! -path './coverage/*' ! -path './.git/*' -print |
      LC_ALL=C sort |
      while IFS= read -r input; do
        printf '%s\n' "$input"
        sha256sum "$input"
      done
    sha256sum /usr/local/bin/build-web-assets /app/mix.lock /app/deps/phoenix/package.json /app/deps/phoenix_html/package.json
    node --version
    npm --version
    printf '%s\n' "${MIXPANEL_TOKEN:-}" "${MIXPANEL_URL:-}" "${AMPLITUDE_API_KEY:-}" "${AMPLITUDE_URL:-}"
  ) | sha256sum | cut -d ' ' -f 1
}

mkdir -p "$cache_dir"
digest=$(assets_digest)
if [ -f "$cache_dir/inputs" ] && [ "$(cat "$cache_dir/inputs")" = "$digest" ] &&
   [ -f "$BLOCKSCOUT_STATIC_DIR/js/app.js" ] && [ -f "$BLOCKSCOUT_STATIC_DIR/css/app.css" ]; then
  echo 'mix-in-builder: reusing web page assets' >&2
  exit 0
fi

cd "$assets_dir"
echo 'mix-in-builder: building web page assets' >&2
npm ci --no-audit --no-fund
rm -rf "$cache_dir/static.next"
BLOCKSCOUT_STATIC_DIR="$cache_dir/static.next" npm run deploy
rm -rf "$BLOCKSCOUT_STATIC_DIR"
mv "$cache_dir/static.next" "$BLOCKSCOUT_STATIC_DIR"
printf '%s' "$digest" > "$cache_dir/inputs"
