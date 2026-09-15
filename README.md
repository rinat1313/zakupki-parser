# zakupki-parser

Go-сервис парсинга закупок с **разных торговых площадок** в одном репозитории/контейнере.

Сейчас:
- **ЕИС** (`zakupki.gov.ru`) — полноценный адаптер (44/223/pricereq)
- остальные ЭТП — stub-адаптеры (tektorg, mos, roseltorg, …), расширяются в `internal/adapter`

## API (parser)

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/health` | liveness |
| POST | `/api/v1/fetch` | `{ "reg_number", "source_site" }` → карточка + документы + тексты |
| POST | `/api/v1/extract` | `multipart/form-data` поле `file` → `{ "status", "filename", "body" }` (обработка до ~5 мин) |

Порт по умолчанию: **8091**.

```bash
go run ./cmd/service
curl -s -X POST http://127.0.0.1:8091/api/v1/fetch \
  -H 'Content-Type: application/json' \
  -d '{"reg_number":"0334500000125000001","source_site":"https://zakupki.gov.ru"}'

# extract: клиентский timeout ≥ 5–6 минут (OCR/LibreOffice)
curl -s -m 360 -X POST http://127.0.0.1:8091/api/v1/extract \
  -F 'file=@./document.pdf'
```

## Search service (`cmd/search`)

UI-совместимый микросервис под Gateway `SEARCH_URL` (контракт из UI «Поисковики»).

Порт по умолчанию: **8093**.

| Метод | Путь | Описание |
|-------|------|----------|
| POST | `/api/v1/auth/login` | `demo` / `demo` → Bearer token |
| GET/POST | `/api/v1/searchers` | список / создание настройки |
| PUT | `/api/v1/searchers/{id}/auto-ai` | AI-анализ **на эту** настройку |
| POST | `/api/v1/searchers/{id}/run` | поиск ЕИС → сохранение хитов → fetch/analyze |
| GET | `/api/v1/searchers/{id}/tenders` | тендеры выбранного поиска |

Поток: сохранить настройку → `run` → новые `reg_number` уходят в `PARSER_URL` (обработка документов); при `auto_ai=true` после появления карточки в core ставится AI-анализ.

```bash
export DATABASE_URL='postgres://zakupki:zakupki@localhost:5432/zakupki_search?sslmode=disable'
export PARSER_URL='http://127.0.0.1:8091'
export CORE_URL='http://127.0.0.1:8080'
export HTTP_ADDR=':8093'
go run ./cmd/search
# Swagger: http://127.0.0.1:8093/swagger/
# Gateway: SEARCH_URL=http://127.0.0.1:8093
```

## Как добавить площадку

1. Реализуйте `adapter.Adapter` (`Hosts`, `Fetch`) в `internal/adapter/<site>/`.
2. Зарегистрируйте в `init()` через `adapter.Register(...)`.
3. Пересоберите сервис — отдельный репозиторий **не нужен**.

## CLI (legacy)

Рядом остаётся исследовательский CLI из бывшего `DataCode` (`cmd/parser`) для файловой выгрузки в `result/`.

## Оркестрация

Полный стек поднимается из [zakupki-platform](https://github.com/rinat1313/zakupki-platform).
