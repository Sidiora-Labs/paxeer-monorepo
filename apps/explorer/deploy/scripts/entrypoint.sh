#!/bin/bash


export_envs_from_preset() {
  if [ -z "$ENVS_PRESET" ]; then
      return
  fi

  if [ "$ENVS_PRESET" = "none" ]; then
      return
  fi

  local preset_file="./configs/envs/.env.$ENVS_PRESET"

  if [ ! -f "$preset_file" ]; then
      return
  fi

  local blacklist=(
    "NEXT_PUBLIC_APP_PROTOCOL"
    "NEXT_PUBLIC_APP_HOST"
    "NEXT_PUBLIC_APP_PORT"
    "NEXT_PUBLIC_APP_ENV"
    "NEXT_PUBLIC_API_WEBSOCKET_PROTOCOL"
  )

  while IFS='=' read -r name value; do
      name="${name#"${name%%[![:space:]]*}"}"  # Trim leading whitespace
      if [[ -n $name && $name == "NEXT_PUBLIC_"* && ! "${blacklist[*]}" =~ "$name" ]]; then
          export "$name"="$value"
      fi
  done < <(grep "^[^#;]" "$preset_file")
}

# If there is a preset, load the environment variables from the its file
export_envs_from_preset

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
