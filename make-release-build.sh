#!/bin/sh -e

cat <<EOF
Note that it is your responsibility that the code that is built as a specific
version comes from a clean version tag.

This script only builds for linux and windows. The macos version requires CGO
to compile, hence it cannot be cross-compiled, i.e. use podman.

Requires tkey-libs to be cloned next to this top folder.

EOF

version="$1"
if [ -z "$version" ]; then
  printf "give me a version number\n"
  exit 1
fi
shift

if ! hash 2>/dev/null sha512sum; then
  sha512sum() {
    shasum -a 512 "$@"
  }
fi

# look for tkey-libs
if [ ! -e ../tkey-libs ]; then
  printf "Could not find tkey-libs.\n"
  exit 1
fi

# build a fresh tkey-libs
make -C ../tkey-libs clean
make -C ../tkey-libs podman

# build application binary using podman
# Start from scratch
printf "Start from scratch\n"
make clean
podman run --rm --mount type=bind,source="$(pwd)",target=/src --mount \
        type=bind,source="$(pwd)/../tkey-libs",target=/tkey-libs -w /src \
        -it ghcr.io/tillitis/tkey-builder:2 make random-generator/app.bin -j

cp -af random-generator/app.bin client-app/app.bin

targets="linux windows"
printf "Will build for: %s\n" "$targets"

outd="release-builds"
mkdir -p "$outd"

cmd="client-app"
exec_name="tkey-random-generator"

if [ -e buildall ]; then
  printf "./buildall already exists, from a failed run?\n"
  exit 1
fi

cat >buildall <<EOF
#!/bin/sh -e
EOF
chmod +x buildall

for os in $targets; do
  outos="$os"
  archs="amd64"
  if [ "$os" = "darwin" ]; then
    outos="macos"
    archs="amd64 arm64"
    cat >>buildall <<EOF
export CGO_ENABLED=1
EOF
  else
      cat >>buildall <<EOF
export CGO_ENABLED=0
EOF
  fi
  suffix=""
  [ "$os" = "windows" ] && suffix=".exe"

  for arch in $archs; do
    cat >>buildall <<EOF
printf "Building $version for $os $arch\n"
export GOOS=$os GOARCH=$arch
go build -trimpath -buildvcs=false -ldflags="-X=main.version=$version" \
   -o "$outd/${exec_name}_${version}_$outos-$arch$suffix" ./$cmd
EOF
  done
done

podman run --rm -it --mount "type=bind,source=$(pwd),target=/build" -w /build \
       ghcr.io/tillitis/tkey-builder:2 ./buildall
rm -f buildall

cd "$outd"
printf "Create hashes:\n"
for binf in "$exec_name"*; do
  if [ ! -e "$binf.sha512" ]; then
    printf "Hash file doesn't exist. Creating %s\n" "$binf.sha512"
    sha512sum >"$binf.sha512" "$binf"
  fi
done
cd ..

set -x
ls -l "$outd"