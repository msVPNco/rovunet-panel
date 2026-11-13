#!/usr/bin/env bash
# Build the official userspace runtime and tools from immutable source commits.
# Produces private staging assets; does not install or publish anything.
set -euo pipefail
output=${1:?usage: build-managed-awg.sh OUTPUT_DIR}
arch=${GOARCH:-amd64}
go_commit=b5928efb6ca19f0153958460c3d141f04abc5c2e
tools_commit=ee0f0a9aa34ff0a0da4b3433b9512781cfe02843
build_dir=$(mktemp -d)
trap 'rm -rf -- "$build_dir"' EXIT
mkdir -p "$output"
output=$(realpath "$output")
git clone --quiet https://github.com/amnezia-vpn/amneziawg-go.git "$build_dir/go"
git -C "$build_dir/go" checkout --quiet --detach "$go_commit"
test "$(git -C "$build_dir/go" rev-parse HEAD)" = "$go_commit"
(
    cd "$build_dir/go"
    go mod verify
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" GOAMD64=v1 go build -trimpath -buildvcs=true -ldflags='-s -w' -o "$output/amneziawg-go-linux-$arch" .
)
git clone --quiet https://github.com/amnezia-vpn/amneziawg-tools.git "$build_dir/tools"
git -C "$build_dir/tools" checkout --quiet --detach "$tools_commit"
test "$(git -C "$build_dir/tools" rev-parse HEAD)" = "$tools_commit"
CFLAGS=-O3 make -s -C "$build_dir/tools/src" -j2 CC="${CC:-gcc}" LDFLAGS=-static wg
cp "$build_dir/tools/src/wg" "$output/awg-linux-$arch"
cp "$build_dir/go/LICENSE" "$output/LICENSE-amneziawg-go.txt"
cp "$build_dir/tools/COPYING" "$output/LICENSE-amneziawg-tools.txt"
git -C "$build_dir/go" archive --format=tar.gz --prefix=amneziawg-go/ --output="$output/amneziawg-go-source.tar.gz" "$go_commit"
git -C "$build_dir/tools" archive --format=tar.gz --prefix=amneziawg-tools/ --output="$output/amneziawg-tools-source.tar.gz" "$tools_commit"
python3 - "$output" "$arch" "$go_commit" "$tools_commit" << 'PY'
import hashlib,json,os,pathlib,subprocess,sys
out=pathlib.Path(sys.argv[1]);arch=sys.argv[2]
sources={'amneziawg-go':('https://github.com/amnezia-vpn/amneziawg-go',sys.argv[3],'MIT'), 'awg':('https://github.com/amnezia-vpn/amneziawg-tools',sys.argv[4],'GPL-2.0')}
records=[]
for name,(url,commit,license) in sources.items():
    asset=f'{name}-linux-{arch}'
    records.append(dict(asset=asset,source=url,commit=commit,license=license,sha256=hashlib.sha256((out/asset).read_bytes()).hexdigest()))
(out/f'awg-provenance-linux-{arch}.json').write_text(json.dumps(dict(toolchain=subprocess.check_output(['go','version'],text=True).strip(),c_compiler=subprocess.check_output([os.environ.get('CC','gcc'),'--version'],text=True).splitlines()[0],go_flags='CGO_ENABLED=0 GOAMD64=v1 -trimpath -buildvcs=true -ldflags=-s -w',c_flags='-O3 (plus pinned Makefile flags); -static',architecture=arch,engines=records),indent=2)+'\n')
PY
(
    cd "$output"
    sha256sum "amneziawg-go-linux-$arch" "awg-linux-$arch" "awg-provenance-linux-$arch.json" > "AWG-SHA256SUMS-$arch"
)
