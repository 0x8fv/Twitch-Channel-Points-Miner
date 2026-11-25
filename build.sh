#!/usr/bin/env bash
set -e

# builds into ./dist.
# run this script without any arguments to build default list of binaries
# usage: ./build.sh [target1 target2 ...]
# example: ./build.sh linux/amd64/ linux/arm64/ windows/amd64/

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)" # get script path ($PSScriptRoot) 
cd "$REPO_ROOT"

DIST_DIR="$REPO_ROOT/dist"
mkdir -p "$DIST_DIR"

ICON_PATH="$REPO_ROOT/assets/gopher.ico"
WINDOWS_RESOURCE="$REPO_ROOT/build_icon_windows_amd64.syso"

# cleanup of temporary resource file on exit
trap 'if [ -f "$WINDOWS_RESOURCE" ]; then rm -f "$WINDOWS_RESOURCE"; fi' EXIT

# use command-line arguments if provided, otherwise use default targets
# $# is amount of arguments
if [ $# -gt 0 ]; then
  TARGETS=("$@")
else
  TARGETS=(
    "windows/amd64/.exe"
    "linux/amd64/"
    "darwin/amd64/"
    "darwin/arm64/"
  )
fi

for TARGET in "${TARGETS[@]}"; do
  NEEDS_WINDOWS_RESOURCE=false
  IFS='/' read -r TARGET_OS TARGET_ARCH TARGET_EXT <<< "$TARGET"
  
  # auto-add .exe for Windows if extension wasn't explicitly provided
  if [ "$TARGET_OS" == "windows" ] && [ -z "$TARGET_EXT" ]; then
    TARGET_EXT=".exe"
  fi

  # check if we need to generate the Windows resource file
  if [ "$TARGET_OS" == "windows" ]; then
    echo "detected windows target"
    NEEDS_WINDOWS_RESOURCE=true
  fi

  BINARY_NAME="TwitchChannelPointsMiner-${TARGET_OS}-${TARGET_ARCH}${TARGET_EXT}"
  OUTPUT_PATH="$DIST_DIR/$BINARY_NAME"

  # generate Windows resource only when building a Windows target
  if $NEEDS_WINDOWS_RESOURCE; then
    echo "generating windows resource"
    if [ ! -f "$ICON_PATH" ]; then
      echo "Windows icon not found at $ICON_PATH" >&2
      exit 1
    fi
    echo "generating windows icon resource"
    go run github.com/akavel/rsrc@v0.10.2 -arch "$TARGET_ARCH" -ico "$ICON_PATH" -o "$WINDOWS_RESOURCE"
  fi

  echo "Building $BINARY_NAME..."
  
  GOOS="$TARGET_OS" GOARCH="$TARGET_ARCH" CGO_ENABLED="0" \
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o "$OUTPUT_PATH" "$REPO_ROOT"
  
  if [[ $? && -f "$OUTPUT_PATH" ]]; then
    echo "DONE: $BINARY_NAME built successfully."
  else
    echo "ERRO: $BINARY_NAME was not successfully built! continuinng anyways..." >&2
  fi
done

echo "build(s) complete -> $DIST_DIR"