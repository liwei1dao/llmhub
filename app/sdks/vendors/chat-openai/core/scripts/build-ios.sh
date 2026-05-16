#!/usr/bin/env bash
# Build a single binary-only XCFramework for the chat-openai capability.
#
# Inputs:
#   * Rust capability + ffi-ios crates  (vendors/chat-openai/core/{capability,ffi-ios})
#   * Swift facade source               (vendors/chat-openai/core/facade-ios/Sources/*.swift)
#
# Output (single artefact):
#   <out>/LLMHubChat.xcframework
#
#       LLMHubChat.xcframework/
#       ├── ios-arm64/
#       │   └── LLMHubChat.framework/
#       │       ├── LLMHubChat                       (merged static .a — Rust + Swift)
#       │       ├── Modules/
#       │       │   ├── module.modulemap
#       │       │   └── LLMHubChat.swiftmodule/      (compiled module, no source)
#       │       └── Info.plist
#       └── ios-arm64_x86_64-simulator/LLMHubChat.framework/
#
# Consumers depend on the SwiftPM package at
#   vendors/chat-openai/ios/LLMHubChat/Package.swift
# which exposes a single `.binaryTarget` pointing at this XCFramework.
#
# Requires:
#   * Rust 1.86 + ios targets (rustup target add aarch64-apple-ios
#     aarch64-apple-ios-sim x86_64-apple-ios)
#   * cbindgen   (cargo install cbindgen)
#   * Xcode (xcodebuild, swiftc, lipo, libtool)

set -euo pipefail

OUT_DIR=""
PROFILE="release"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --out)     OUT_DIR="$2"; shift 2 ;;
        --profile) PROFILE="$2"; shift 2 ;;
        *) echo "unknown arg: $1" >&2; exit 1 ;;
    esac
done

if [[ -z "$OUT_DIR" ]]; then
    echo "usage: $0 --out <xcframework_parent_dir> [--profile release|debug]" >&2
    exit 2
fi

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
WORKSPACE_DIR="$(cd -- "${SCRIPT_DIR}/../../../.." &> /dev/null && pwd)"  # app/sdks
FFI_DIR="${WORKSPACE_DIR}/vendors/chat-openai/core/ffi-ios"
FACADE_DIR="${WORKSPACE_DIR}/vendors/chat-openai/core/facade-ios"
TARGET_DIR="${CARGO_TARGET_DIR:-${WORKSPACE_DIR}/core/target}"

# `LIB_NAME` is the Swift module / framework / xcframework name.
# Deliberately != the main public class (LLMHubChat) so that
# `.swiftinterface` doesn't shadow module-level types with class-level
# ones (Swift parses `LLMHubChat.ChatRole` as "ChatRole nested in
# class LLMHubChat" otherwise).
LIB_NAME="LLMHubChatKit"
VERSION="0.1.0"
MIN_IOS="15.0"

PROFILE_DIR=$([[ "$PROFILE" == "release" ]] && echo "release" || echo "debug")
CARGO_FLAGS=()
[[ "$PROFILE" == "release" ]] && CARGO_FLAGS+=("--release")
SWIFT_OPT_FLAGS=()
[[ "$PROFILE" == "release" ]] && SWIFT_OPT_FLAGS+=("-O")

export LITCRYPT_ENCRYPT_KEY="${LITCRYPT_ENCRYPT_KEY:-$(openssl rand -hex 32 2>/dev/null || date +%s%N)}"

STAGING="$(mktemp -d)"
trap 'rm -rf "$STAGING"' EXIT

# ---------- 1. cargo build the Rust core per iOS slice ----------------
echo "==> cargo build llmhub-chat-openai-ffi-ios (${PROFILE})"
for t in aarch64-apple-ios aarch64-apple-ios-sim x86_64-apple-ios; do
    (cd "$WORKSPACE_DIR" && cargo build -p llmhub-chat-openai-ffi-ios --target "$t" "${CARGO_FLAGS[@]}")
done

# ---------- 2. cbindgen header + private clang modulemap --------------
# The Swift facade imports the C ABI via `@_implementationOnly import
# LLMHubChatC`. We define a private clang module so swiftc can find
# the header — but the module never ships into the final framework.
HEADERS="$STAGING/private-headers"
mkdir -p "$HEADERS"
echo "==> cbindgen → llmhub_chat_openai.h"
(cd "$FFI_DIR" && cbindgen \
    --config "${FFI_DIR}/cbindgen.toml" \
    --crate llmhub-chat-openai-ffi-ios \
    --output "${HEADERS}/llmhub_chat_openai.h")

cat > "${HEADERS}/module.modulemap" <<'EOF'
module LLMHubChatC {
    header "llmhub_chat_openai.h"
    export *
}
EOF

# ---------- 3. Compile the Swift facade per slice ---------------------
# Args: <module_triple> <swift_target_triple> <rust_triple> <sdk_name>
#
# module_triple        — used as swiftmodule filename, must NOT include
#                        the OS version (Xcode looks up by bare triple).
# swift_target_triple  — what `-target` gets; this DOES include the OS
#                        version so swiftc gates APIs against MIN_IOS.
build_swift_slice() {
    local module_triple="$1"
    local swift_target="$2"
    local rust_triple="$3"
    local sdk="$4"
    local slice="${STAGING}/by-triple/${module_triple}"
    mkdir -p "${slice}/${LIB_NAME}.swiftmodule"

    echo "==> swiftc ${swift_target} (${sdk})"
    xcrun -sdk "$sdk" swiftc \
        -emit-library -static \
        -emit-module -emit-module-interface \
        -enable-library-evolution \
        -module-name "${LIB_NAME}" \
        -target "${swift_target}" \
        "${SWIFT_OPT_FLAGS[@]}" \
        -I "$HEADERS" \
        -Xcc "-fmodule-map-file=${HEADERS}/module.modulemap" \
        -emit-module-path           "${slice}/${LIB_NAME}.swiftmodule/${module_triple}.swiftmodule" \
        -emit-module-interface-path "${slice}/${LIB_NAME}.swiftmodule/${module_triple}.swiftinterface" \
        -o "${slice}/lib${LIB_NAME}-swift.a" \
        "${FACADE_DIR}/Sources/"*.swift

    # Merge Swift .a + Rust .a → one static archive named for the framework.
    libtool -static \
        -o "${slice}/${LIB_NAME}" \
        "${slice}/lib${LIB_NAME}-swift.a" \
        "${TARGET_DIR}/${rust_triple}/${PROFILE_DIR}/libllmhub_chat_openai.a"
}

# module_triple        |  swift_target_triple                    | rust target           | sdk
build_swift_slice  arm64-apple-ios             "arm64-apple-ios${MIN_IOS}"             aarch64-apple-ios      iphoneos
build_swift_slice  arm64-apple-ios-simulator   "arm64-apple-ios${MIN_IOS}-simulator"   aarch64-apple-ios-sim  iphonesimulator
build_swift_slice  x86_64-apple-ios-simulator  "x86_64-apple-ios${MIN_IOS}-simulator"  x86_64-apple-ios       iphonesimulator

# ---------- 4. Assemble static .framework dirs per XCFramework slice --
# Args:
#   --fw   <output framework dir>
#   --plat <iPhoneOS|iPhoneSimulator>
#   --lib  <static archive path>    (repeatable: simulator slice has 2)
#   --mod  <swiftmodule source dir> (repeatable: same idea)
mk_framework() {
    local fw_dir="" platform=""
    local libs=() modules=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --fw)   fw_dir="$2"; shift 2 ;;
            --plat) platform="$2"; shift 2 ;;
            --lib)  libs+=("$2"); shift 2 ;;
            --mod)  modules+=("$2"); shift 2 ;;
            *) echo "mk_framework: unknown arg $1" >&2; exit 1 ;;
        esac
    done

    mkdir -p "${fw_dir}/Modules/${LIB_NAME}.swiftmodule"

    # Framework binary: single .a, or lipo'd universal.
    if [[ ${#libs[@]} -eq 1 ]]; then
        cp "${libs[0]}" "${fw_dir}/${LIB_NAME}"
    else
        lipo -create "${libs[@]}" -output "${fw_dir}/${LIB_NAME}"
    fi

    # Copy each triple's swiftmodule/swiftinterface/swiftdoc into the
    # framework's single Modules/<name>.swiftmodule dir. Different
    # triples end up with different filenames; that's how Xcode picks
    # the right one at consume time.
    for m in "${modules[@]}"; do
        cp -R "$m"/. "${fw_dir}/Modules/${LIB_NAME}.swiftmodule/"
    done

    # Framework-level module map. Pure Swift, so `export *` is enough —
    # consumers only see what's in the .swiftinterface.
    cat > "${fw_dir}/Modules/module.modulemap" <<EOF
framework module ${LIB_NAME} {
    export *
}
EOF

    cat > "${fw_dir}/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>           <string>${LIB_NAME}</string>
  <key>CFBundleIdentifier</key>           <string>io.llmhub.${LIB_NAME}</string>
  <key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
  <key>CFBundleName</key>                 <string>${LIB_NAME}</string>
  <key>CFBundlePackageType</key>          <string>FMWK</string>
  <key>CFBundleShortVersionString</key>   <string>${VERSION}</string>
  <key>CFBundleVersion</key>              <string>1</string>
  <key>MinimumOSVersion</key>             <string>${MIN_IOS}</string>
  <key>CFBundleSupportedPlatforms</key>   <array><string>${platform}</string></array>
</dict>
</plist>
EOF
}

DEVICE_FWK="${STAGING}/fwk/ios-arm64/${LIB_NAME}.framework"
SIM_FWK="${STAGING}/fwk/ios-sim/${LIB_NAME}.framework"

DEVICE_TRIPLE=arm64-apple-ios
SIM_TRIPLES=(arm64-apple-ios-simulator x86_64-apple-ios-simulator)

mk_framework \
    --fw "$DEVICE_FWK" --plat iPhoneOS \
    --lib "${STAGING}/by-triple/${DEVICE_TRIPLE}/${LIB_NAME}" \
    --mod "${STAGING}/by-triple/${DEVICE_TRIPLE}/${LIB_NAME}.swiftmodule"

mk_framework \
    --fw "$SIM_FWK" --plat iPhoneSimulator \
    --lib "${STAGING}/by-triple/${SIM_TRIPLES[0]}/${LIB_NAME}" \
    --lib "${STAGING}/by-triple/${SIM_TRIPLES[1]}/${LIB_NAME}" \
    --mod "${STAGING}/by-triple/${SIM_TRIPLES[0]}/${LIB_NAME}.swiftmodule" \
    --mod "${STAGING}/by-triple/${SIM_TRIPLES[1]}/${LIB_NAME}.swiftmodule"

# ---------- 5. Bundle into the final XCFramework ----------------------
XCF_OUT="${OUT_DIR}/${LIB_NAME}.xcframework"
rm -rf "$XCF_OUT"
mkdir -p "$OUT_DIR"

xcodebuild -create-xcframework \
    -framework "$DEVICE_FWK" \
    -framework "$SIM_FWK" \
    -output "$XCF_OUT"

echo "==> done: $XCF_OUT"
