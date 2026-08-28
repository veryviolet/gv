# gv — GitVerse из командной строки

`gv` — консольный клиент для [GitVerse](https://gitverse.ru), устроенный по образцу
[`gh`](https://cli.github.com). Публичный API GitVerse намеренно повторяет GitHub
(`/repos/{owner}/{repo}/pulls`, `/user/repos`, `clone_url`, `html_url`), поэтому привычные
сценарии переносятся почти без изменений: `gv pr create`, `gv repo clone`, `gv issue list`,
`gv api`.

```
gv auth login
gv repo view
gv pr list
gv issue create --title "Падает на старте" --body-file notes.md
gv api /user --jq .login
```

## Установка

Нужен Go 1.25 или новее.

```bash
make build && install -m755 bin/gv /usr/local/bin/gv
```

Кросс-сборка под linux, darwin и windows (amd64/arm64):

```bash
make cross
```

## Авторизация

OAuth device flow у GitVerse нет, поэтому токен создаётся в веб-интерфейсе —
[gitverse.ru/settings/tokens](https://gitverse.ru/settings/tokens) — и вставляется в `gv`:

```bash
gv auth login
```

Неинтерактивно (CI, скрипты):

```bash
gv auth login --with-token < token.txt
```

Токен хранится в `~/.config/gv/hosts.yml` с правами `0600`. Переменные окружения
`GV_TOKEN` и `GITVERSE_TOKEN` имеют приоритет над файлом и на диск не пишутся.

Чтобы `git clone/fetch/push` по HTTPS ходили с этим же токеном:

```bash
gv auth setup-git
```

**Важно:** API GitVerse требует Bearer-токен для всех эндпоинтов, включая чтение публичных
репозиториев. Анонимных запросов не бывает — без `gv auth login` не работает ничего.

## Команды

| Команда | Что делает |
|---|---|
| `gv auth login\|logout\|status\|token\|setup-git` | Токены и настройка git |
| `gv repo view\|list\|create\|clone\|fork\|delete` | Репозитории |
| `gv pr list\|view\|create\|diff\|files\|commits\|checkout\|edit\|close\|reopen` | Pull request'ы |
| `gv issue list\|view\|create\|comment\|edit\|close\|reopen` | Задачи |
| `gv api` | Сырой запрос к любому из 130 эндпоинтов API |
| `gv version`, `gv completion` | Версия и автодополнение shell |

Общие соглашения:

- `-R/--repo OWNER/REPO` у `gv pr` и `gv issue` выбирает репозиторий; без него берётся
  тот, на который указывает git-remote текущего каталога (приоритет `upstream` → `origin`).
- `--json`, `--jq <выражение>`, `--template <шаблон>` — машинный вывод. `--jq` использует
  [gojq](https://github.com/itchyny/gojq), `--template` — Go-шаблоны с функциями
  `truncate`, `timeago`, `timefmt`, `color`, `join`, `pluck`, `tablerow`, `tablerender`.
- В терминале вывод — таблица с цветами, в пайпе — TSV без цветов.
- `--hostname` (глобально) и `GV_HOST` позволяют работать с другой инсталляцией GitVerse.

### `gv api`

Escape-hatch ко всему API, включая то, для чего нет высокоуровневых команд —
Actions, runners, secrets, variables, releases, webhooks, packages, artifacts.

```bash
gv api /user
gv api repos/{owner}/{repo}/pulls --paginate --jq '.[] | "\(.number) \(.title)"'
gv api -X PATCH repos/{owner}/{repo}/issues/12 -f state=closed
gv api repos/{owner}/{repo}/actions/runs --jq '.workflow_runs // .'
```

Плейсхолдеры `{owner}`, `{repo}`, `{hostname}` подставляются из текущего репозитория.
`-f key=value` передаёт строку, `-F key=value` — типизированное значение (`true`, `false`,
`null`, число, `@файл`, `@-` для stdin). Для GET-запросов параметры уходят в query-строку,
иначе — в JSON-тело. `--paginate` собирает все страницы в **один** JSON-массив (в отличие
от `gh`, который печатает страницы подряд).

## Чего нет — и почему

Это ограничения API GitVerse, а не пробелы реализации:

| Аналог в `gh` | Статус | Причина |
|---|---|---|
| `gh pr merge` | нет | В API есть только `GET /pulls/{n}/merge` — проверка «смержен ли». Эндпоинта на слияние нет, мержить нужно из веб-интерфейса |
| `gh pr review` | нет | Review-эндпоинтов в спецификации нет |
| `gh search repos` | нет | Из поиска доступен только `GET /search/users` |
| `gh label` | нет | Есть только `GET /issues/{index}/labels` — без CRUD |
| `gh gist`, `gh notification` | нет | Таких сущностей в API нет |

Отдельные особенности:

- `gv pr create --label-id` принимает **числовые id** меток, а не имена: эндпоинт создания
  PR требует id, а найти метку по имени API не позволяет. У `gv issue` метки задаются
  именами — там эндпоинт принимает строки.
- `gv issue edit --label/--assignee` **заменяют** списки целиком, потому что API принимает
  списки, а не дельты.
- `gv issue list` по умолчанию отфильтровывает pull request'ы: как и в GitHub, они приходят
  из того же эндпоинта. Вернуть их можно флагом `--include-prs`.
- `gv pr checkout` сначала пробует серверный ref `refs/pull/<n>/head`, и если его нет —
  тянет head-ветку напрямую из форка.

## Разработка

```bash
make build      # сборка в bin/gv
make test       # go test ./...
make lint       # go vet + проверка gofmt
make generate   # перегенерация internal/api/types_gen.go из spec/
make cross      # сборка под все платформы в dist/
```

Типы ответов и запросов (`internal/api/types_gen.go`, 110 структур) генерируются из
вендоренной спецификации `spec/openapi-1.8.json` — официального описания
[GitVerse Public API](https://gitverse.ru/gitverse/rest-api-description) в формате
OpenAPI 2.0. Руками этот файл не правят. Методы эндпоинтов написаны вручную в
`internal/api/queries.go`: `gv` использует около четверти операций, и читаемые сигнатуры
здесь важнее полноты генерации.

Раскладка каталогов повторяет `gh`:

```
cmd/gv          точка входа
internal/api    HTTP-клиент, пагинация, ошибки, сгенерированные типы, запросы
internal/config ~/.config/gv/{config,hosts}.yml
internal/git    обёртки над git
internal/gvrepo разбор OWNER/REPO и URL
internal/*      iostreams, tableprinter, text, export, browser, cmdutil
pkg/cmd/*       команды
spec/           спецификация API
```

Диагностика запросов:

```bash
GV_DEBUG=1 gv pr list
```

Переменные окружения: `GV_TOKEN`, `GITVERSE_TOKEN`, `GV_HOST`, `GV_CONFIG_DIR`,
`GV_API_URL`, `GV_API_VERSION`, `GV_DEBUG`, `GV_BROWSER`, `GV_PAGER`, `GV_EDITOR`,
`NO_COLOR`.

## Лицензия

MIT — см. [LICENSE](LICENSE).

`spec/openapi-1.8.json` — официальное описание API GitVerse, © 2025 GitVerse,
тоже под MIT: [spec/LICENSE-GITVERSE](spec/LICENSE-GITVERSE). Зависимости из `go.mod`
здесь не распространяются и остаются под своими лицензиями.
