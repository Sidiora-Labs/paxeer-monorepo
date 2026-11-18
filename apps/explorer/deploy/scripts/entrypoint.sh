#!/bin/bash


export_envs_from_preset() {
  if [ -z "$ENVS_PRESET" ]; then
      return
  fi

  if [ "$ENVS_PRESET" = "none" ]; then
      return
  fi

  local dot_preset_file="./configs/envs/.env.$ENVS_PRESET"
  local tracked_preset_file="./configs/envs/$ENVS_PRESET.env"
  local preset_file="$dot_preset_file"

  if [[ ! $ENVS_PRESET =~ ^[a-zA-Z0-9._-]+$ ]]; then
      printf 'Invalid preset %q; preset paths: %q and %q\n' "$ENVS_PRESET" "$dot_preset_file" "$tracked_preset_file" >&2
      return 1
  fi

  if [ ! -f "$preset_file" ]; then
      preset_file="$tracked_preset_file"
  fi

  if [ ! -f "$preset_file" ]; then
      printf 'Preset %q not found; preset paths: %q and %q\n' "$ENVS_PRESET" "$dot_preset_file" "$tracked_preset_file" >&2
      return 1
  fi

  local deployment_names=(
    "NEXT_PUBLIC_APP_PROTOCOL"
    "NEXT_PUBLIC_APP_HOST"
    "NEXT_PUBLIC_APP_PORT"
    "NEXT_PUBLIC_APP_ENV"
    "NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL"
    "NEXT_PUBLIC_API_HOST"
    "NEXT_PUBLIC_ASSISTANT_ENABLED"
  )

  local name value deployment_name preserve_process
  while IFS='=' read -r name value; do
      name="${name#"${name%%[![:space:]]*}"}"  # Trim leading whitespace
      if [[ -n $name && $name == "NEXT_PUBLIC_"* ]]; then
          preserve_process=false
          for deployment_name in "${deployment_names[@]}"; do
              if [[ $name == "$deployment_name" && -v $name ]]; then
                  preserve_process=true
                  break
              fi
          done
          if [ "$preserve_process" = false ]; then
              export "$name"="$value"
          fi
      fi
  done < <(grep "^[^#;]" "$preset_file")
  return 0
}

# If there is a preset, load the environment variables from the its file
export_envs_from_preset || exit 1

# Download external assets
./download_assets.sh ./public/assets/configs
if [ $? -ne 0 ]; then
  echo "🛑 Failed to download external assets. The application cannot start."
  exit 1
fi

# Check run-time ENVs values
if [ "$SKIP_ENVS_VALIDATION" != "true" ]; then
  ./validate_envs.sh
  if [ $? -ne 0 ]; then
    exit 1
  fi
else
  echo "😱 Skipping ENVs validation."
  echo
fi

# Generate favicons bundle
./favicon_generator.sh
if [ $? -ne 0 ]; then
  echo "👎 Unable to generate favicons bundle."
else
  echo "👍 Favicons bundle successfully generated."
fi
echo

# Generate OG image
node --no-warnings ./og_image_generator.js

# Create envs.js file with run-time environment variables for the client app
envs_output="./public/assets/envs.js"
if ! ./make_envs_script.sh "$envs_output"; then
  echo "🛑 Failed to generate runtime environment script $envs_output. The application cannot start." >&2
  exit 1
fi

# Generate multichain config
node --no-warnings ./deploy/tools/multichain-config-generator/dist/index.js
if [ $? -ne 0 ]; then
  echo "👎 Unable to generate multichain config."
  exit 1
fi

# Generate essential dapps chains config
node --no-warnings ./deploy/tools/essential-dapps-chains-config-generator/dist/index.js
if [ $? -ne 0 ]; then
  echo "👎 Unable to generate essential dapps chains config."
  exit 1
fi

# Generate sitemap.xml and robots.txt files
./sitemap_generator.sh

# Generate llms.txt file
node --no-warnings ./deploy/tools/llms-txt-generator/dist/index.js

# Print list of enabled features
node --no-warnings ./feature-reporter.js

echo "Starting Next.js application"
exec "$@"
