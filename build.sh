#! /bin/sh

set -eu

tkey_libs_version="v0.0.2"

if [ -d ../tkey-libs ]
then
    (cd ../tkey-libs; git checkout "$tkey_libs_version")
else
    git clone -b "$tkey_libs_version" https://github.com/tillitis/tkey-libs.git ../tkey-libs
fi

make -j -C ../tkey-libs

make -j tkey-random-generator tkey-random-generator-dev
