# Пакет Cursor: skills, rules, subagents

Это **не ТЗ**. Архив — бэкап конфигурации агента из `rinat1313/NewGeneratorOPCUA` (`origin/main`, 2026-10-22).

Скопируй каталог `.cursor/` в корень другого репозитория (рядом с кодом продукта). Не копируй `.cursor/backup`, если он появится позже.

После копирования закоммить в ветку, которую агент считает основной для skills (часто `main`).

## Как понять, что это не ТЗ

Сообщение «ТЗ.» запускает 8 этапов: первым `lead` через Task. Вопросы, консультации и бэкапы — отвечать самому, код продукта не трогать без пула этапа 4.

## Состав

- **skills (17):** `tz-workflow`, этапы `tz-intake` … `tz-report`, группы `tz-specialists` / `tz-executors` / `tz-testers` / `tz-writers`, `tz-expected-results`, плюс `ui-ux`, `user-docs`, `go-logging`
- **rules (8):** `tz-workflow`, `project`, `commit-to-main`, `merge-to-main`, `cursor-config-on-main`, `update-user-help`, `inspector-editing`, `git-readonly`
- **agents (16):** `lead`; специалисты `analyst`, `architect`, `ui-ux-designer`, `security`, `algorithmist`, `programmer`, `database`; исполнители `backend`, `frontend`, `sql`, `devops`; тестеры `qa-tester`, `manual-tester`, `sql-tester`; `technical-writer`

`analyst` обязан собирать пул ожидаемых результатов `ER-n` (вход → выход). Тестеры берут ТЗ, решения **и** этот пул; пул нельзя пропускать.
