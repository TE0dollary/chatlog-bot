#!/bin/bash

set -eu
set -o pipefail
[ "$#" = "1" ] && [ "$1" = '-v' ] && set -x

OUTPUT_DIR="bin"
PACKAGES_DIR="packages"
TEMP_DIR="temp_package"
VERSION=$(git describe --tags --always --dirty="-dev")
CHECKSUMS_FILE="$PACKAGES_DIR/checksums.txt"

make -f Makefile crossbuild

rm -rf $PACKAGES_DIR $TEMP_DIR

mkdir -p $PACKAGES_DIR $TEMP_DIR

echo "" > $CHECKSUMS_FILE

for binary in $OUTPUT_DIR/chatlog_darwin_*; do
    binary_name=$(basename $binary)
    os_arch=$(echo $binary_name | cut -d'_' -f 2-)

    # quick start binary
    if [[ $binary_name == "chatlog_darwin_amd64" ]]; then
        cp "$binary" "$PACKAGES_DIR/chatlog_macos_amd64"
        echo "$(sha256sum $PACKAGES_DIR/chatlog_macos_amd64 | sed "s|$PACKAGES_DIR/||")" >> $CHECKSUMS_FILE
    elif [[ $binary_name == "chatlog_darwin_arm64" ]]; then
        cp "$binary" "$PACKAGES_DIR/chatlog_macos_arm64"
        echo "$(sha256sum $PACKAGES_DIR/chatlog_macos_arm64 | sed "s|$PACKAGES_DIR/||")" >> $CHECKSUMS_FILE
    fi

    cp "README.md" "LICENSE" $TEMP_DIR
    cp "$binary" "$TEMP_DIR/chatlog"

    package_name="chatlog_${VERSION}_${os_arch}.tar.gz"
    tar -czf "$PACKAGES_DIR/$package_name" -C $TEMP_DIR .

    rm -rf $TEMP_DIR/*

    echo "$(sha256sum $PACKAGES_DIR/$package_name | sed "s|$PACKAGES_DIR/||")" >> $CHECKSUMS_FILE

done

rm -rf $TEMP_DIR

echo "📦 All packages and their sha256 checksums have been created in $PACKAGES_DIR/"