#!/usr/bin/env bash
# Build llmhub-chat-openai-ffi-android for every enabled Android ABI
# and stage the resulting .so files under <jniLibs>/<abi>/. Requires:
#
#   * Rust toolchain + the four Android targets (rust-toolchain.toml
#     installs them on first cargo invocation)
#   * cargo-ndk:   cargo install cargo-ndk
#   * Android NDK installed; either ANDROID_NDK_HOME or
#     ANDROID_NDK_ROOT must point at it.
#
# The script is idempotent — cargo handles incremental rebuilds — and
# it's safe to call from Gradle's `:llmhub-chat:buildNativeCore` task.

set -euo pipefail

OUT_DIR=""
PROFILE="release"
ABIS=("arm64-v8a" "armeabi-v7a" "x86_64")

while [[ $# -gt 0 ]]; do
    case "$1" in
        --out)     OUT_DIR="$2"; shift 2 ;;
        --profile) PROFILE="$2"; shift 2 ;;
        --abis)    IFS=',' read -ra ABIS <<< "$2"; shift 2 ;;
        *) echo "unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$OUT_DIR" ]]; then
    echo "usage: $0 --out <jniLibs_dir> [--profile release|debug] [--abis a,b,c]" >&2
    exit 2
fi

if [[ -z "${ANDROID_NDK_HOME:-}" && -n "${ANDROID_NDK_ROOT:-}" ]]; then
    export ANDROID_NDK_HOME="$ANDROID_NDK_ROOT"
fi
if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
    echo "ANDROID_NDK_HOME (or ANDROID_NDK_ROOT) must be set" >&2
    exit 3
fi

# scripts/         → vendors/chat-openai/core/scripts
# ../../../../     → app/sdks   (the Cargo workspace root)
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
WORKSPACE_DIR="$(cd -- "${SCRIPT_DIR}/../../../.." &> /dev/null && pwd)"

CARGO_FLAGS=()
[[ "$PROFILE" == "release" ]] && CARGO_FLAGS+=("--release")

# Compile-time string-encryption seed. Different per build, so two
# release artifacts never have identical encrypted literals — making
# diffing against an older .so for hard-coded paths harder.
export LITCRYPT_ENCRYPT_KEY="${LITCRYPT_ENCRYPT_KEY:-$(openssl rand -hex 32 2>/dev/null || date +%s%N)}"

mkdir -p "$OUT_DIR"

for abi in "${ABIS[@]}"; do
    echo "==> building llmhub_chat_openai for $abi ($PROFILE)"
    (
        cd "$WORKSPACE_DIR"
        cargo ndk -t "$abi" -o "$OUT_DIR" \
            build -p llmhub-chat-openai-ffi-android "${CARGO_FLAGS[@]}"
    )
done

# cargo-ndk lays out .so under <out>/<abi>/lib<name>.so already — but
# defensive strip in case the host toolchain wasn't picked up by the
# release profile.
if command -v "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$(uname | tr '[:upper:]' '[:lower:]')-x86_64/bin/llvm-strip" >/dev/null 2>&1; then
    STRIP="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$(uname | tr '[:upper:]' '[:lower:]')-x86_64/bin/llvm-strip"
    find "$OUT_DIR" -name "libllmhub_chat_openai.so" -exec "$STRIP" --strip-unneeded {} +
fi

echo "==> done: artefacts under $OUT_DIR"
