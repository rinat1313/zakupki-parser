#!/usr/bin/env bash
# 02 — карточка закупки 44-ФЗ (HTML + printForm XML)
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

eis_curl "${OUT_DIR}/02_notice44.html" \
  "${BASE}/epz/order/notice/${NOTICE_44_TYPE}/view/common-info.html?regNumber=${NOTICE_44}"

eis_curl "${OUT_DIR}/02_notice44.xml" \
  "${BASE}/epz/order/notice/printForm/viewXml.html?regNumber=${NOTICE_44}"
