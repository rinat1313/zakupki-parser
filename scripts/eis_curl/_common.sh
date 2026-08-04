#!/usr/bin/env bash
# Общие настройки для тестовых curl к ЕИС.
# Подключается через: source "$(dirname "$0")/_common.sh"

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_DIR="${SCRIPT_DIR}/out"
mkdir -p "$OUT_DIR"

# -k: на части Mac curl не видит цепочку CA ЕИС (curl error 60).
# Для продакшена в Go лучше настроить CA; для локальных тестов -k ок.
CURL_OPTS=(-sS -L -k -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

BASE="https://zakupki.gov.ru"

# Примеры из html/44 фз и html/223 фз (проверено 2026-08-02)
NOTICE_44="0432200000826003978"
NOTICE_44_TYPE="ea20"
NOTICE_223="32616257756"
NOTICE_223_GUID="ddfc7c5e-bfbd-463a-894e-4f0f85263e27"
ORG_44_CODE="01622000077"
ORG_223_AGENCY="190768"
# НЕ называть переменную UID — в zsh это служебная readonly-переменная
FILE_UID_44="019FBB86942B7A909ABCE6E631E10720"
FILE_UID_223="45907B865F85415FB3D1A467E638A7A8"

eis_curl() {
  local out="$1"
  shift
  echo "→ GET $*"
  curl "${CURL_OPTS[@]}" -o "$out" -w "HTTP %{http_code}, size %{size_download} → ${out}\n" "$@"
}
