#!/bin/bash

echo "🌀 Creating client script with ENV values..."

output_file="${1:-./public/assets/envs.js}"

# collect_envs.sh packages raw NAME=value defaults. Deployment/preset values
# already in the environment win, including explicitly empty values.
load_defaults() {
    local name value
    if [ ! -f .env ]; then
        return 0
    fi
    while IFS='=' read -r name value || [[ -n "$name" ]]; do
        if [[ "$name" =~ ^NEXT_PUBLIC_[a-zA-Z0-9_]+$ ]] && [[ ! -v "$name" ]]; then
            if ! export "$name=$value"; then
                echo "Unable to load packaged default $name." >&2
                return 1
            fi
        fi
    done < .env
}

if ! load_defaults; then
    exit 1
fi

# Fail before invoking the encoder when the output directory is unavailable.
output_dir="${output_file%/*}"
if [[ "$output_dir" == "$output_file" ]]; then
    output_dir="."
fi
if [[ ! -d "$output_dir" || ! -w "$output_dir" ]]; then
    echo "Unable to write runtime environment script $output_file." >&2
    exit 1
fi

# Node is part of the production image; encode the complete object without
# editing its strings, and publish only a fully written file in the same dir.
if ! node - "$output_file" <<'JS'
const fs = require('node:fs');
const path = require('node:path');
const outputFile = process.argv[2];
let temporaryDir;
try {
  const envs = Object.fromEntries(Object.entries(process.env).filter(([name]) =>
    name.startsWith('NEXT_PUBLIC_') && !name.startsWith('NEXT_PUBLIC_VERCEL')));
  const script = `window.__envs = ${JSON.stringify(envs)};\n`;
  temporaryDir = fs.mkdtempSync(path.join(path.dirname(outputFile), '.envs-'));
  const temporaryFile = path.join(temporaryDir, 'envs.js');
  fs.writeFileSync(temporaryFile, script, { encoding: 'utf8', mode: 0o644, flag: 'wx' });
  fs.renameSync(temporaryFile, outputFile);
} catch (error) {
  console.error(`Unable to write runtime environment script ${outputFile}: ${error.message}`);
  process.exitCode = 1;
} finally {
  if (temporaryDir) fs.rmSync(temporaryDir, { recursive: true, force: true });
}
JS
then
    echo "Failed to generate runtime environment script $output_file." >&2
    exit 1
fi

echo "✅ Done."
