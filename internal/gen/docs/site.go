package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// siteCSS is inlined so the published pages need no extra requests, and so the
// site survives a host that only serves files it recognises by extension.
const siteCSS = `
:root { --fg:#1a1a1a; --muted:#666; --bg:#fff; --code-bg:#f5f5f5; --line:#e3e3e3; --link:#2b5eb8; }
@media (prefers-color-scheme: dark) {
  :root { --fg:#e6e6e6; --muted:#9a9a9a; --bg:#151515; --code-bg:#1f1f1f; --line:#333; --link:#79a9f0; }
}
* { box-sizing: border-box; }
body { margin:0; background:var(--bg); color:var(--fg);
       font:16px/1.65 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif; }
.wrap { max-width:52rem; margin:0 auto; padding:2.5rem 1.2rem 5rem; }
header { border-bottom:1px solid var(--line); margin-bottom:2rem; padding-bottom:1rem; }
header nav a { margin-right:1.2rem; }
h1 { font-size:2rem; margin:.2rem 0 .4rem; }
h2 { font-size:1.35rem; margin-top:2.5rem; padding-top:.4rem; border-top:1px solid var(--line); }
h3 { font-size:1.05rem; margin-top:1.8rem; }
a { color:var(--link); text-decoration:none; }
a:hover { text-decoration:underline; }
code { font-family:ui-monospace,SFMono-Regular,Menlo,monospace; font-size:.92em;
       background:var(--code-bg); padding:.12em .35em; border-radius:4px; }
pre { background:var(--code-bg); padding:.9rem 1.1rem; border-radius:8px; overflow-x:auto; }
pre code { background:none; padding:0; }
table { border-collapse:collapse; width:100%; margin:1rem 0; display:block; overflow-x:auto; }
th,td { text-align:left; padding:.45rem .7rem; border-bottom:1px solid var(--line); vertical-align:top; }
th { font-weight:600; }
.muted { color:var(--muted); }
.cmd { margin-bottom:2.5rem; }
.toc { columns:2; column-gap:2rem; padding-left:1.1rem; }
@media (max-width:40rem) { .toc { columns:1; } }
footer { margin-top:4rem; padding-top:1rem; border-top:1px solid var(--line); color:var(--muted); font-size:.9rem; }
`

// writePage wraps body in the shared HTML skeleton.
func writePage(path, title, body string) error {
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"ru\">\n<meta charset=\"utf-8\">\n")
	fmt.Fprintf(&b, "<title>%s</title>\n", html.EscapeString(title))
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<style>%s</style>\n", siteCSS)
	b.WriteString("<body>\n<div class=\"wrap\">\n")
	b.WriteString(`<header><nav><a href="./">gv</a><a href="commands.html">Команды</a>` +
		`<a href="https://gitverse.ru/veryviolet/gv">Исходники</a>` +
		`<a href="https://gitverse.ru/veryviolet/gv/releases">Релизы</a></nav></header>` + "\n")
	b.WriteString(body)
	b.WriteString("\n<footer>gv — консольный клиент GitVerse. Лицензия MIT.</footer>\n")
	b.WriteString("</div>\n")
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// generateSite renders a static documentation site from the command tree.
func generateSite(root *cobra.Command, outDir, version string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := writePage(filepath.Join(outDir, "index.html"), "gv — GitVerse из командной строки", landingPage(version)); err != nil {
		return err
	}
	return writePage(filepath.Join(outDir, "commands.html"), "gv — справочник команд", commandsPage(root))
}

func landingPage(version string) string {
	var b strings.Builder
	b.WriteString(`<h1>gv</h1>
<p class="muted">Консольный клиент <a href="https://gitverse.ru">GitVerse</a>, устроенный по образцу
<a href="https://cli.github.com"><code>gh</code></a>. Репозитории, pull request'ы, задачи и релизы
без ухода из терминала.</p>

<pre><code>gv repo view
gv pr list --state all
gv issue create --title "Падает на старте" --body-file notes.md
gv api /user --jq .login</code></pre>

<h2>Установка</h2>

<h3>Через Go</h3>
<p>Работает без токена и без дополнительных настроек:</p>
<pre><code>go install gitverse.ru/veryviolet/gv/cmd/gv@latest</code></pre>

<h3>Пакеты</h3>
<p>Готовые <code>deb</code>, <code>rpm</code> и <code>apk</code> под amd64 и arm64 лежат в
<a href="https://gitverse.ru/veryviolet/gv/releases">релизах</a>. В пакет входят бинарник,
man-страницы и автодополнения для bash, zsh и fish.</p>
<pre><code># Debian, Ubuntu, Astra
gunzip -c gv_0.1.0-1_amd64.deb.gz &gt; gv.deb &amp;&amp; sudo dpkg -i gv.deb

# Fedora, RHEL, Alma, Rocky, РЕД ОС
gunzip -c gv-0.1.0-1.x86_64.rpm.gz &gt; gv.rpm &amp;&amp; sudo rpm -i gv.rpm

# Alpine
sudo apk add --allow-untrusted gv_0.1.0-r1_x86_64.apk</code></pre>

<h2>Авторизация</h2>
<p>OAuth device flow у GitVerse нет, поэтому токен создаётся в веб-интерфейсе на странице
<a href="https://gitverse.ru/settings/tokens">настроек</a> и вставляется в <code>gv</code>:</p>
<pre><code>gv auth login</code></pre>
<p>Токен хранится в <code>~/.config/gv/hosts.yml</code> с правами <code>0600</code>. Переменные
<code>GV_TOKEN</code> и <code>GITVERSE_TOKEN</code> имеют приоритет над файлом и на диск не пишутся.
Чтобы <code>git</code> ходил по HTTPS с тем же токеном — <code>gv auth setup-git</code>.</p>
<p class="muted">API GitVerse требует токен для всех эндпоинтов, включая чтение публичных
репозиториев: анонимных запросов не бывает.</p>

<h2>Машинный вывод</h2>
<p>У всех списковых команд есть <code>--json</code>, <code>--jq</code> и <code>--template</code>.
В терминале вывод табличный и цветной, в пайпе — TSV без цветов.</p>
<pre><code>gv pr list --json --jq '.[] | "\(.number) \(.title)"'
gv repo list --template '{{range .}}{{tablerow .full_name .visibility}}{{end}}{{tablerender}}'</code></pre>

<h2>Чего нет</h2>
<p>Это ограничения API платформы, а не пробелы клиента:</p>
<table>
<tr><th>Аналог в <code>gh</code></th><th>Почему нет</th></tr>
<tr><td><code>gh pr merge</code></td><td>В API есть только проверка «смержен ли»; эндпоинта на слияние нет</td></tr>
<tr><td><code>gh pr review</code></td><td>Review-эндпоинтов в спецификации нет</td></tr>
<tr><td><code>gh search repos</code></td><td>Из поиска доступен только поиск пользователей</td></tr>
<tr><td><code>gh label</code></td><td>Метки только читаются, CRUD нет</td></tr>
</table>
`)
	if version != "" && version != "dev" {
		fmt.Fprintf(&b, "\n<p class=\"muted\">Документация собрана для версии <code>%s</code>.</p>\n",
			html.EscapeString(version))
	}
	return b.String()
}

// commandsPage renders the whole command tree: usage, flags and examples.
func commandsPage(root *cobra.Command) string {
	var cmds []*cobra.Command
	var collect func(*cobra.Command)
	collect = func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			return
		}
		cmds = append(cmds, c)
		for _, sub := range c.Commands() {
			collect(sub)
		}
	}
	collect(root)

	var b strings.Builder
	b.WriteString("<h1>Справочник команд</h1>\n<ul class=\"toc\">\n")
	for _, c := range cmds[1:] {
		path := c.CommandPath()
		fmt.Fprintf(&b, "<li><a href=\"#%s\"><code>%s</code></a></li>\n", anchor(path), html.EscapeString(path))
	}
	b.WriteString("</ul>\n")

	for _, c := range cmds {
		b.WriteString(commandSection(c))
	}
	return b.String()
}

func commandSection(c *cobra.Command) string {
	path := c.CommandPath()
	var b strings.Builder

	fmt.Fprintf(&b, "\n<div class=\"cmd\" id=\"%s\">\n", anchor(path))
	fmt.Fprintf(&b, "<h2><code>%s</code></h2>\n", html.EscapeString(path))
	if c.Short != "" {
		fmt.Fprintf(&b, "<p>%s</p>\n", html.EscapeString(c.Short))
	}
	if long := strings.TrimSpace(c.Long); long != "" && long != c.Short {
		fmt.Fprintf(&b, "<p>%s</p>\n", paragraphs(long))
	}
	fmt.Fprintf(&b, "<pre><code>%s</code></pre>\n", html.EscapeString(c.UseLine()))

	if flags := flagRows(c.NonInheritedFlags()); flags != "" {
		b.WriteString("<h3>Флаги</h3>\n<table>\n<tr><th>Флаг</th><th>Описание</th></tr>\n")
		b.WriteString(flags)
		b.WriteString("</table>\n")
	}
	if example := strings.TrimSpace(c.Example); example != "" {
		fmt.Fprintf(&b, "<h3>Примеры</h3>\n<pre><code>%s</code></pre>\n", html.EscapeString(example))
	}
	b.WriteString("</div>\n")
	return b.String()
}

func flagRows(fs *pflag.FlagSet) string {
	var b strings.Builder
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", " + name
		}
		if varname, _ := pflag.UnquoteUsage(f); varname != "" && f.Value.Type() != "bool" {
			name += " &lt;" + html.EscapeString(varname) + "&gt;"
		}
		_, usage := pflag.UnquoteUsage(f)
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td></tr>\n", name, html.EscapeString(usage))
	})
	return b.String()
}

// paragraphs turns blank-line-separated text into HTML paragraphs.
func paragraphs(s string) string {
	blocks := strings.Split(s, "\n\n")
	for i, block := range blocks {
		blocks[i] = html.EscapeString(strings.TrimSpace(block))
	}
	return strings.Join(blocks, "</p>\n<p>")
}

func anchor(commandPath string) string {
	return strings.ReplaceAll(commandPath, " ", "-")
}
