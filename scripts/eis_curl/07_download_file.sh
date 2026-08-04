#!/usr/bin/env bash
# 07 — скачать один файл 44 и один 223
# ВАЖНО: не использовать имя переменной UID (в zsh это служебная переменная).
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

LOCAL_FILE_UID_44="${1:-$FILE_UID_44}"
LOCAL_FILE_UID_223="${2:-$FILE_UID_223}"

eis_curl "${OUT_DIR}/07_file44.bin" \
  "${BASE}/44fz/filestore/public/1.0/download/priz/file.html?uid=${LOCAL_FILE_UID_44}"

eis_curl "${OUT_DIR}/07_file223.bin" \
  "${BASE}/223/filestore/public/1.0/download/fz223/file.html?uid=${LOCAL_FILE_UID_223}"

echo "--- file(1) ---"
file "${OUT_DIR}/07_file44.bin" "${OUT_DIR}/07_file223.bin" || true
ls -la "${OUT_DIR}/07_file44.bin" "${OUT_DIR}/07_file223.bin"
