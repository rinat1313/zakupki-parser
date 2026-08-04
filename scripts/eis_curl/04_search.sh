#!/usr/bin/env bash
# 04 — поиск закупок: «Разработка ПО»
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"

QUERY="${1:-Разработка ПО}"

echo "→ searchString=${QUERY}"
curl "${CURL_OPTS[@]}" -G -o "${OUT_DIR}/04_search.html" \
  -w "HTTP %{http_code}, size %{size_download} → ${OUT_DIR}/04_search.html\n" \
  --data-urlencode "searchString=${QUERY}" \
  --data-urlencode "morphology=on" \
  --data-urlencode "fz44=on" \
  --data-urlencode "fz223=on" \
  --data-urlencode "af=on" \
  --data-urlencode "ca=on" \
  --data-urlencode "pc=on" \
  --data-urlencode "pa=on" \
  --data-urlencode "sortBy=UPDATE_DATE" \
  --data-urlencode "pageNumber=1" \
  --data-urlencode "recordsPerPage=_10" \
  "${BASE}/epz/order/extendedsearch/results.html"
