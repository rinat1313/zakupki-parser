DataCode — код и рабочие данные парсера на Go
==============================================
Весь код проекта пишется ТОЛЬКО здесь.
Полные руководства по парсингу — в ../result/
Образцы HTML (только чтение) — в ../html/ (не изменять).


СТРУКТУРА
---------
DataCode/
  README.txt              — этот файл
  AI_AGENT.txt            — краткая инструкция для AI-агента
  go.mod / go.sum         — модуль eisparser
  models/                 — структуры данных
  html_map/               — где в HTML лежат поля
  docs/                   — как работать со структурами
  internal/
    parser/fz44/          — парсер извещений 44-ФЗ
    parser/fz223/         — парсер извещений 223-ФЗ
    parser/pricereq/      — парсер «Запросы цен» (/epz/pricereq)
    csvinput/             — чтение data/tenders*.csv
    store/                — выгрузка в result/{id}/
    extract/              — LibreOffice / pdftotext → .txt
    textutil/             — деньги, даты, нормализация текста
  pkg/eis/                — HTTP-клиент ЕИС
  cmd/parser/             — CLI: CSV → парсинг → result/
  cmd/extract/            — пересборка .txt из уже скачанных files/
  data/
    tenders.csv           — общий список (44+223+pricereq, law авто)
    tenders_223.csv       — опционально только 223
    README.txt
  scripts/eis_curl/       — smoke curl к живому сайту


С ЧЕГО НАЧАТЬ
-------------
cd DataCode

# curl-тесты
bash scripts/eis_curl/02_notice44.sh
bash scripts/eis_curl/03_notice223.sh

# парсер (смешанный CSV 44+223, автодетект закона → result/{id}/)
go run ./cmd/parser
go run ./cmd/parser -csv data/tenders.csv
go run ./cmd/parser -law auto -limit 1

# принудительно один закон
go run ./cmd/parser -law 44 -csv data/tenders.csv
go run ./cmd/parser -law 223 -csv data/tenders_223.csv

# только текст из уже скачанных файлов (LibreOffice + OCR для сканов):
go run ./cmd/extract
go run ./cmd/extract -id 32312323655
# нечитаемые PDF/сканы → result/failed_texts.json и result/{id}/failed_texts.json

go run ./cmd/parser -fixture "../html/44 фз/Сведения закупки 44 фз.html"
go run ./cmd/parser -law 223 -fixture "../html/223 фз/Карточка закупки 223 фз.html"

go test ./internal/parser/fz44/ ./internal/parser/fz223/ ./internal/extract/ ./internal/detect/



НЕ ДЕЛАТЬ
---------
- Писать Go/код вне DataCode/
- Один парсер на 44 и 223 с общими CSS-селекторами
- Менять файлы в html/ и over/
- Хардкодить только ea20 — брать href из поиска
