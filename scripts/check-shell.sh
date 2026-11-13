#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
files=(install.sh setup.sh dns-resolvers.sh scripts/*.sh)
for script in "${files[@]}"; do bash -n "$script"; done
shellcheck "${files[@]}"
shfmt -d -i 4 -ci -sr "${files[@]}"
