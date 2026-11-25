#!/usr/bin/env bash
set -euo pipefail
umask 077

if [[ $# -ne 2 || ! -d ${2:-} ]]; then
  echo "Usage: $0 {sign|notarize} path/to/application.app" >&2
  exit 2
fi

case "$1" in
  sign)
    certificate=${MACOS_CERT_P12:?Missing MACOS_CERT_P12}
    certificate_password=${MACOS_CERT_PASSWORD:?Missing MACOS_CERT_PASSWORD}
    identity=${MACOS_SIGN_IDENTITY:?Missing MACOS_SIGN_IDENTITY}
    unset MACOS_CERT_P12 MACOS_CERT_PASSWORD MACOS_SIGN_IDENTITY
    ;;
  notarize)
    apple_id=${APPLE_ID:?Missing APPLE_ID}
    team_id=${APPLE_TEAM_ID:?Missing APPLE_TEAM_ID}
    apple_password=${APPLE_APP_PASSWORD:?Missing APPLE_APP_PASSWORD}
    unset APPLE_ID APPLE_TEAM_ID APPLE_APP_PASSWORD
    ;;
  *)
    echo "Unknown macOS release operation: $1" >&2
    exit 2
    ;;
esac

app=$2
work=
child=
keychain=

cleanup() {
  local status=$?
  trap - EXIT
  trap '' INT TERM
  if [[ -n $child ]]; then
    kill -KILL "$child" 2>/dev/null || true
    wait "$child" 2>/dev/null || true
  fi
  if [[ -f $keychain ]] && ! security delete-keychain "$keychain"; then
    echo "Could not unregister the temporary signing keychain" >&2
    if (( status == 0 )); then status=1; fi
  fi
  if [[ -n $work ]] && { ! rm -f -- "$work/certificate.p12" "$work/notarize.zip" "$work/release.keychain-db" ||
                         ! rmdir -- "$work"; }; then
    echo "Could not remove temporary macOS release files" >&2
    if (( status == 0 )); then status=1; fi
  fi
  exit "$status"
}

# Waiting for a background command lets Bash handle cancellation immediately.
run_interruptible() {
  local status=0
  "$@" &
  child=$!
  wait "$child" || status=$?
  child=
  return "$status"
}

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

work=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/jindi-macos-release.XXXXXX")
p12="$work/certificate.p12"
keychain="$work/release.keychain-db"
archive="$work/notarize.zip"

if [[ $1 == sign ]]; then
  printf '%s' "$certificate" | base64 --decode > "$p12"
  unset certificate
  keychain_password=$(openssl rand -hex 24)
  run_interruptible security create-keychain -p "$keychain_password" "$keychain"
  run_interruptible security unlock-keychain -p "$keychain_password" "$keychain"
  run_interruptible security import "$p12" -k "$keychain" -P "$certificate_password" -T /usr/bin/codesign
  unset certificate_password
  rm -f -- "$p12"
  run_interruptible security set-key-partition-list -S apple-tool:,apple: -s -k "$keychain_password" "$keychain" >/dev/null
  unset keychain_password
  run_interruptible codesign --force --deep --options runtime --timestamp --keychain "$keychain" --sign "$identity" "$app"
else
  run_interruptible ditto -c -k --keepParent "$app" "$archive"
  run_interruptible xcrun notarytool submit "$archive" --apple-id "$apple_id" --team-id "$team_id" --password "$apple_password" --wait
  unset apple_password
  run_interruptible xcrun stapler staple "$app"
  run_interruptible xcrun stapler validate "$app"
fi

run_interruptible codesign --verify --deep --strict "$app"
