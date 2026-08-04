#!/usr/bin/env bash
# 03 — карточка закупки 223-ФЗ
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

eis_curl "${OUT_DIR}/03_notice223.html" \
  "${BASE}/epz/order/notice/notice223/common-info.html?noticeGuid=${NOTICE_223_GUID}&regNumber=${NOTICE_223}"
