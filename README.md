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

### Через Go

Самый короткий путь, работает без токена:

```bash
go install gitverse.ru/veryviolet/gv/cmd/gv@latest
```

### apt — подписанный репозиторий

GitVerse держит родной реестр deb-пакетов и **сам подписывает метаданные**, поэтому
`apt` проверяет подпись обычным образом, без `trusted=yes`. Ключ отдаётся анонимно:

```bash
sudo mkdir -p /etc/apt/keyrings
sudo curl -fsSL https://gitverse.ru/api/packages/veryviolet/debian/repository.key -o /etc/apt/keyrings/gitverse-veryviolet.asc
echo 'deb [signed-by=/etc/apt/keyrings/gitverse-veryviolet.asc] https://gitverse.ru/api/packages/veryviolet/debian stable main' | sudo tee /etc/apt/sources.list.d/gitverse-veryviolet.list
sudo apt update && sudo apt install gv
```

### dnf — подписанный репозиторий

Файл `.repo` генерирует сам сервер, `gpgcheck` в нём уже включён:

```bash
sudo dnf config-manager --add-repo https://gitverse.ru/api/packages/veryviolet/rpm/gv.repo
sudo dnf install gv
```

### Пакеты файлом

Готовые пакеты лежат в [релизах](https://gitverse.ru/veryviolet/gv/releases).
Собираются `deb`, `rpm` и `apk` под `amd64` и `arm64`; в них входят бинарник,
man-страницы и автодополнения для bash, zsh и fish.

```bash
# Debian, Ubuntu, Astra — .deb выложен сжатым, см. примечание ниже
gunzip -c gv_0.2.0-1_amd64.deb.gz > gv.deb && sudo dpkg -i gv.deb

# Fedora, RHEL, Alma, Rocky, РЕД ОС
gunzip -c gv-0.2.0-1.x86_64.rpm.gz > gv.rpm && sudo rpm -i gv.rpm

# Alpine
sudo apk add --allow-untrusted gv_0.2.0-r1_x86_64.apk
```

> **Почему `.deb.gz`, а не `.deb`.** GitVerse проверяет содержимое файла,
> прикрепляемого к релизу, и принимает только архивы: `.tar.gz`, `.zip`, `.apk`
> (это тоже gzip) проходят, а `.deb`, `.rpm` и простой текст отклоняются с 400 и
> 422. Поэтому deb и rpm выкладываются сжатыми. Ограничение серверное, не наше.

### Tarball

Для остальных систем — архив с бинарником, man-страницами и автодополнениями:

```bash
tar xzf gv_0.2.0_linux_amd64.tar.gz
sudo install -m755 gv_0.2.0_linux_amd64/bin/gv /usr/local/bin/gv
```

Контрольные суммы всех артефактов — в `SHA256SUMS.gz` рядом с ними.

### Из исходников

Нужен Go 1.25 или новее.

```bash
make build && sudo install -m755 bin/gv /usr/local/bin/gv
```

Сборка релизного набора целиком — `make release` (нужен
[nfpm](https://github.com/goreleaser/nfpm)), подготовка файлов в том виде,
в каком их принимает GitVerse — `make publish`.

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
| `gv package push\|list\|versions\|delete` | Пакеты и образы: публикация в реестры, просмотр, удаление |
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

## Публикация пакетов

У GitVerse есть родные реестры `deb`, `rpm` и `generic`, и метаданные репозиториев
платформа подписывает сама — то есть подписанный `apt install` получается без своего
сервера и без собственного GPG-ключа.

```bash
gv package push dist/gv_0.2.0-1_amd64.deb dist/gv_0.2.0-1_arm64.deb
gv package push dist/*.rpm --group gv
gv package push notes.txt --kind generic --name gv --version 0.2.0
```

После публикации команда печатает готовые строки для подключения репозитория на клиенте.

Особенности, выясненные на живом API:

- **Загрузка идёт как `PUT` с сырым телом.** Если отправить файл как форму
  (`curl --data-binary -X PUT` ставит `Content-Type: application/x-www-form-urlencoded`),
  сервер отвечает `500 internal server error` без объяснений. Правильно — `curl --upload-file`
  или, что то же самое, `gv package push`.
- **Аутентификация — basic `логин:токен`**, а не `Bearer`. Токену нужен уровень доступа
  «Запись» к реестру пакетов; галочка «Репозитории» — это отдельный git-доступ.
- **Версии неизменяемы**: повторная заливка того же имени, версии и архитектуры даёт
  `409 package file already exists`. Сначала удалить старую версию.
- **Лимит 500 МБ** на всё хранилище пакетов владельца.

## Контейнеры

Реестр образов у GitVerse живёт на том же домене, что и репозитории — отдельного хоста
вроде `ghcr.io` нет. Говорит по OCI Distribution v2 и принимает тот же личный токен:

```bash
gv auth docker-login                  # docker login gitverse.ru токеном из конфига gv
docker build -t gitverse.ru/veryviolet/gv:0.2.0 .
docker push gitverse.ru/veryviolet/gv:0.2.0
```

```bash
gv package list                       # свои образы
gv package list veryviolet --tags     # с тегами
gv package versions gv                # версии пакета
gv package delete gv --version 42     # удалить одну версию
```

Две особенности, выясненные на живом API:

- Тип пакета для образов — **`container`**, не `docker`: последнее API отклоняет как
  невалидное. Ещё принимаются `generic`, `debian`, `rpm`, `maven`, `npm`, `pypi`,
  `helm`, `nuget`, `go`.
- Эндпоинта «перечислить пакеты» в API нет — имя нужно знать заранее. Поэтому
  `gv package list` читает каталог реестра, который общий для всей платформы и
  отсортирован по имени: gv стартует сразу с префикса владельца, а не листает всё
  подряд.

Теги в реестре регистронезависимы: `image:Tag` и `image:tag` — одно и то же.

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
