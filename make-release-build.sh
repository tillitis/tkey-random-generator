#!/bin/sh -e

set -eu

cat <<EOF
Note that it is your responsibility that the code that is built as a specific
version comes from a clean version tag.

Specify version of the release to build to use when invoking this script.

Use as "./make-release-build.sh <version_name_of_release>

This script only builds for linux and windows. The macos version requires CGO
to compile, hence it cannot be cross-compiled, i.e. use podman.

EOF

export GOTOOLCHAIN=go1.23.1

if [ "$#" -ne 1 ]
then
  echo "Please supply release_version"
  exit 1
fi

version="$1"
if [ -z "$version" ]; then
  printf "give me a version number\n"
  exit 1
fi

if ! hash 2>/dev/null sha512sum
then
    sha512sum() {
        shasum -a 512 "$@"
    }
fi

targets="linux windows"
printf "Will build for: %s\n" "$targets"

outd="release-builds"
mkdir -p "$outd"

exec_name="tkey-random-generator"

for os in "linux" "windows"
do
    for arch in "amd64" "arm64"
    do
        printf "Building $version for $os $arch\n"
        make GOOS=$os GOARCH=$arch BUILD_CGO_ENABLED=0 tkey-random-generator
        if [ $os = "windows" ]
        then
            suffix=".exe"
        else
            suffix=""
        fi
        target="$outd/${exec_name}_${version}_$os-$arch$suffix"
        cp tkey-random-generator $target
        sha512sum "$target" >"$target.sha512"
    done
done

set -x
ls -l "$outd"
