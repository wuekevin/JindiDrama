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
    if [[ ! $team_id =~ ^[A-Z0-9]{10}$ ]]; then
      echo "Invalid Apple Team ID" >&2
      exit 1
    fi
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
  if [[ -n $work ]] && { ! rm -f -- "$work/certificate.p12" "$work/notarize.zip" "$work/notary-response.json" "$work/release.keychain-db" ||
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

run_notary_step() {
  local step=$1 status=0
  shift
  run_interruptible "$@" >/dev/null 2>&1 || status=$?
  if (( status != 0 )); then
    printf 'macOS notarization %s failed (exit %d)\n' "$step" "$status" >&2
  fi
  return "$status"
}

read_notary_string() {
  # Preserve invalid control bytes as '?' before command substitution can strip them.
  /usr/bin/plutil -extract "$1" raw -expect string -n -o - "$2" 2>/dev/null |
    LC_ALL=C /usr/bin/tr -c '[:alnum:] -' '?'
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
  run_interruptible codesign --verify --deep --strict "$app"
else
  requirement='anchor apple generic and certificate 1[field.1.2.840.113635.100.6.2.6] exists'
  requirement+=' and certificate leaf[field.1.2.840.113635.100.6.1.13] exists'
  requirement+=" and certificate leaf[subject.OU] = \"$team_id\""
  run_notary_step 'Developer ID check' codesign --verify --deep --strict -R="$requirement" "$app"
  run_notary_step 'archive creation' ditto -c -k --keepParent "$app" "$archive"
  response="$work/notary-response.json"
  submit_status=0
  # The hosted log only gets the validated response fields and the exit status below.
  run_interruptible xcrun notarytool submit "$archive" --apple-id "$apple_id" --team-id "$team_id" \
    --password "$apple_password" --wait --output-format json > "$response" 2>/dev/null || submit_status=$?
  unset apple_password
  if submission_id=$(read_notary_string id "$response") &&
     [[ $submission_id =~ ^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$ ]]; then
    printf 'Apple notarization submission ID: %s\n' "$submission_id"
  fi
  notary_status=$(read_notary_string status "$response") || notary_status=
  case "$notary_status" in
    Invalid|Rejected|"In Progress") printf 'Apple notarization status: %s\n' "$notary_status" >&2 ;;
  esac
  if (( submit_status != 0 )); then
    printf 'Apple notarization request failed (notarytool exit %d)\n' "$submit_status" >&2
    exit "$submit_status"
  fi
  if [[ $notary_status != Accepted ]]; then
    echo "Apple did not accept this notarization submission" >&2
    exit 1
  fi
  echo "Apple accepted this notarization submission"
  run_notary_step 'ticket stapling' xcrun stapler staple "$app"
  run_notary_step 'ticket validation' xcrun stapler validate "$app"
  run_notary_step 'final signature verification' codesign --verify --deep --strict "$app"
fi
