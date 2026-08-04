#!/usr/bin/env bash
# Запуск всех тестовых запросов к ЕИС по порядку.
set -euo pipefail
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== EIS curl smoke tests ==="
echo "OUT: ${DIR}/out"
echo

for s in \
  01_home.sh \
  02_notice44.sh \
  03_notice223.sh \
  04_search.sh \
  05_organization.sh \
  06_documents_list.sh \
  07_download_file.sh
do
  echo "======== ${s} ========"
  bash "${DIR}/${s}"
  echo
  sleep 1
done

echo "=== done ==="
ls -la "${DIR}/out"
