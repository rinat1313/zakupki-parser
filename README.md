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

Порт по умолчанию: **8091**.

```bash
go run ./cmd/service
curl -s -X POST http://127.0.0.1:8091/api/v1/fetch \
  -H 'Content-Type: application/json' \
  -d '{"reg_number":"0334500000125000001","source_site":"https://zakupki.gov.ru"}'
```

## Search service (`cmd/search`)

UI-compatible microservice for Gateway `SEARCH_URL` (port **8093**).

- Auth: `POST /api/v1/auth/login` (`demo`/`demo`), `logout`, `me`
- Searchers CRUD + `.../auto-ai`, `.../run`, `.../tenders`
- Optional legacy aliases: `/api/v1/search-profiles`
- OpenAPI / Swagger: `/swagger/` (spec also in `docs/search-openapi.yaml`)
- Migrations: `migrations/search/` (Postgres via `DATABASE_URL`)

```bash
export DATABASE_URL='postgres://zakupki:zakupki@localhost:5432/zakupki_search?sslmode=disable'
export PARSER_URL='http://127.0.0.1:8091'   # optional deep fetch for new hits
export CORE_URL='http://127.0.0.1:8092'     # optional tender enrichment
go run ./cmd/search
```

## Как добавить площадку

1. Реализуйте `adapter.Adapter` (`Hosts`, `Fetch`) в `internal/adapter/<site>/`.
2. Зарегистрируйте в `init()` через `adapter.Register(...)`.
3. Пересоберите сервис — отдельный репозиторий **не нужен**.

## CLI (legacy)

Рядом остаётся исследовательский CLI из бывшего `DataCode` (`cmd/parser`) для файловой выгрузки в `result/`.

## Оркестрация

Полный стек поднимается из [zakupki-platform](https://github.com/rinat1313/zakupki-platform).
