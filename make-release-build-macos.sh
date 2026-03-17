#!/bin/sh -e

cat <<EOF
Note that it is your responsibility that the code that is built as a specific
version comes from a clean version tag.

Specify version of the release to build to use when invoking this script.

Use as "./make-release-build-macos.sh <version_name_of_release>

Use to build macos targets.

EOF

export GOTOOLCHAIN=go1.23.1

if [ "$#" -ne 1 ]
then
  echo "Please supply release_version"
  exit 1
fi

version="$1"
if [ -z "$version" ]
then
    printf "give me a version number\n"
    exit 1
fi

if ! hash 2>/dev/null sha512sum
then
    sha512sum() {
        shasum -a 512 "$@"
    }
fi

targets="darwin"
printf "Will build for: %s\n" "$targets"

outd="release-builds"
mkdir -p "$outd"

exec_name="tkey-random-generator"

os=darwin
for arch in "amd64" "arm64"
do
    printf "Building $version for $os $arch\n"
    make GOOS=$os GOARCH=$arch BUILD_CGO_ENABLED=1 TKEY_RANDOM_GENERATOR_VERSION="$version" tkey-random-generator
    target="${exec_name}_${version}_$os-$arch$suffix"

    cp tkey-random-generator "$outd/$target"
    (cd "$outd" && sha512sum "$target" >"$target.sha512")

done

# Build tool to create universal binaries, then build a universal
# binary
make -s -C gotools lipo

universaltarget="${exec_name}_${version}_darwin-universal"
./gotools/lipo -output "$outd/$universaltarget" -create \
               "$outd/${exec_name}_${version}_darwin-amd64" \
               "$outd/${exec_name}_${version}_darwin-arm64"
(cd "$outd" && sha512sum "$universaltarget" >"$universaltarget.sha512")

set -x
ls -l "$outd"
