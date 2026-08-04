#!/usr/bin/env bash
# 06 — список документов (ссылки filestore) 44 и 223
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

eis_curl "${OUT_DIR}/06_docs44.html" \
  "${BASE}/epz/order/notice/${NOTICE_44_TYPE}/view/documents.html?regNumber=${NOTICE_44}"

echo "--- filestore 44 ---"
grep -oE "${BASE}/44fz/filestore/public/1.0/download/priz/file.html\?uid=[A-F0-9]+" \
  "${OUT_DIR}/06_docs44.html" | sort -u | tee "${OUT_DIR}/06_docs44_urls.txt"

eis_curl "${OUT_DIR}/06_docs223.html" \
  "${BASE}/epz/order/notice/notice223/documents.html?purchaseNoticeNumber=${NOTICE_223}&noticeGuid=${NOTICE_223_GUID}"

echo "--- filestore 223 ---"
grep -oE "${BASE}/223/filestore/public/1.0/download/fz223/file.html\?uid=[A-F0-9]+" \
  "${OUT_DIR}/06_docs223.html" | sort -u | tee "${OUT_DIR}/06_docs223_urls.txt"
