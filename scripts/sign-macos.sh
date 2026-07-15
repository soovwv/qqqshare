#!/bin/sh
set -eu
: "${APPLE_SIGN_IDENTITY:?Set APPLE_SIGN_IDENTITY}"
: "${APPLE_ID:?Set APPLE_ID}"
: "${APPLE_TEAM_ID:?Set APPLE_TEAM_ID}"
: "${APPLE_APP_PASSWORD:?Set APPLE_APP_PASSWORD}"
APP="${1:?Usage: sign-macos.sh QQQShare.app}"
codesign --force --deep --options runtime --timestamp --sign "$APPLE_SIGN_IDENTITY" "$APP"
ditto -c -k --keepParent "$APP" QQQShare-notarize.zip
xcrun notarytool submit QQQShare-notarize.zip --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_PASSWORD" --wait
xcrun stapler staple "$APP"
spctl --assess --type execute --verbose "$APP"
