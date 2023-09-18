#! /bin/sh

git clone https://github.com/tillitis/tkey-libs.git ../tkey-libs

tkey_libs_version="v0.0.2"

printf "Building tkey-libs with version: %s\n" "$tkey_libs_version"

make -j -C ../tkey-libs clean
cd ../tkey-libs && git checkout "$tkey_libs_version" && cd ../tkey-random-generator
make -j -C ../tkey-libs

make -j
