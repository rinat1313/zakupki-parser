#!/usr/bin/env bash
# 01 — главная страница ЕИС
source "$(cd "$(dirname "$0")" && pwd)/_common.sh"
eis_curl "${OUT_DIR}/01_home.html" "${BASE}/epz/main/public/home.html"
