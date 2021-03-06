#!/usr/bin/env bash

#
# Commands
#

FIND="${FIND:-find}"
GIT="${GIT:-git}"
SHA256SUM="${SHA256SUM:-shasum -a 256}"

#
#
#

nebula::sdk::default_programs() {
  local DEFAULT_PROGRAMS
  DEFAULT_PROGRAMS=( ni spindle )

  for DEFAULT_PROGRAM in ${DEFAULT_PROGRAMS[@]}; do
    printf "%s\n" "${DEFAULT_PROGRAM}"
  done
}

nebula::sdk::git_tag() {
  printf "%s\n" "${GIT_TAG_OVERRIDE:-$( $GIT tag --points-at HEAD 'v*.*.*' )}"
}

nebula::sdk::sha256sum() {
  $SHA256SUM | cut -d' ' -f1
}

nebula::sdk::escape_shell() {
  printf '%s\n' "'${*//\'/\'\"\'\"\'}'"
}

nebula::sdk::release_version() {
  local GIT_TAG GIT_CHANGED_FILES
  GIT_TAG="$( nebula::sdk::git_tag )"
  GIT_CHANGED_FILES="$( $GIT status --short )"

  # Check for releasable version: if we have no tags or any changed files, we
  # can't release.
  if [ -z "${GIT_TAG}" ] || [ -n "${GIT_CHANGED_FILES}" ]; then
    return 1
  fi

  # Arbitrarily pick the first line.
  read GIT_TAG_A <<<"${GIT_TAG}"

  printf "%s\n" "${GIT_TAG_A#v}"
}

nebula::sdk::release_check() {
  if ! nebula::sdk::release_version >/dev/null; then
    echo "$0: no release tag (this commit must be tagged with the format vX.Y.Z)" >&2
    return 2
  fi
}

nebula::sdk::release_vars() {
  RELEASE_VERSION="$( nebula::sdk::release_version || true )"
  if [ -z "${RELEASE_VERSION}" ]; then
    printf 'RELEASE_VERSION=\n'
    return
  fi

  # Parse the version information.
  IFS='.' read RELEASE_VERSION_MAJOR RELEASE_VERSION_MINOR RELEASE_VERSION_PATCH <<<"${RELEASE_VERSION}"

  printf 'RELEASE_VERSION=%s\n' "$( nebula::sdk::escape_shell "${RELEASE_VERSION}" )"
  printf 'RELEASE_VERSION_MAJOR=%s\n' "$( nebula::sdk::escape_shell "${RELEASE_VERSION_MAJOR}" )"
  printf 'RELEASE_VERSION_MINOR=%s\n' "$( nebula::sdk::escape_shell "${RELEASE_VERSION_MINOR}" )"
  printf 'RELEASE_VERSION_PATCH=%s\n' "$( nebula::sdk::escape_shell "${RELEASE_VERSION_PATCH}" )"
}

nebula::sdk::release_vars_local() {
  printf 'local RELEASE_VERSION RELEASE_VERSION_MAJOR RELEASE_VERSION_MINOR RELEASE_VERSION_PATCH\n'
  nebula::sdk::release_vars "$@"
}

nebula::sdk::version() {
  eval "$( nebula::sdk::release_vars )"

  if [ -n "${RELEASE_VERSION}" ]; then
    printf "%s\n" "v${RELEASE_VERSION}"
  else
    $GIT describe --tags --always --dirty
  fi
}

nebula::sdk::cli_vars() {
  if [[ "$#" -ne 1 ]]; then
    echo "usage: ${FUNCNAME[0]} <program>" >&2
    return 1
  fi

  local GO GOOS GOARCH
  GO="${GO:-go}"
  GOOS="$( $GO env GOOS )"
  GOARCH="$( $GO env GOARCH )"

  local EXT=
  [[ "${GOOS}" == "windows" ]] && EXT=.exe

  printf 'CLI_NAME=%s\n' "$( nebula::sdk::escape_shell "$1" )"
  printf 'CLI_VERSION=%s\n' "$( nebula::sdk::version )"
  printf 'CLI_FILE_PREFIX="${CLI_NAME}-${CLI_VERSION}"-%s-%s\n' \
    "$( nebula::sdk::escape_shell "${GOOS}" )" \
    "$( nebula::sdk::escape_shell "${GOARCH}" )"
  printf 'CLI_FILE_BIN="${CLI_FILE_PREFIX}%s"\n' "${EXT}"
}

nebula::sdk::cli_vars_local() {
  printf 'local CLI_NAME CLI_FILE_PREFIX CLI_FILE_BIN\n'
  nebula::sdk::cli_vars "$@"
}

nebula::sdk::cli_artifacts() {
  if [[ "$#" -ne 2 ]]; then
    echo "usage: ${FUNCNAME[0]} <program> <directory>" >&2
    return 1
  fi

  eval "$( nebula::sdk::cli_vars_local "$1" )"

  local CLI_MATCH
  CLI_MATCH="${CLI_NAME}-${CLI_VERSION}-"

  $FIND "$2" -type f -name "${CLI_MATCH}"'*.tar.xz' -or -name "${CLI_MATCH}"'*.zip'
}

nebula::sdk::cli_platform_ext() {
  if [[ "$#" -ne 2 ]]; then
    echo "usage: ${FUNCNAME[0]} <program> <package-file>" >&2
    return 1
  fi

  eval "$( nebula::sdk::cli_vars_local "$1" )"

  local CLI_FILE
  CLI_FILE="$( basename "$2" )"

  printf "%s\n" "${CLI_FILE##${CLI_NAME}-${CLI_VERSION}-}"
}

nebula::sdk::usage() {
  echo "usage: $*" >&2
  exit 1
}
