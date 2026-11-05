package app

import (
	"fmt"
	"io"
	"strings"
)

// commands - "app <command> …", each run instead of the server:
// migrate - the schema migrations (migrate.go); install - a new site on
// an empty database (install.go); theme - the themes kept in their own
// repositories (theme.go); module - the modules from outside the engine
// (module.go); assets - the modules' static files for the web server
// (assets.go); config - the site's settings (config_cmd.go); sites - the
// sites sharing this engine on a server (sites.go); backup - the site's
// database dumped (backup.go); files - the public files copied between
// the directory and a bucket (files_cmd.go); search - the search index
// (search_cmd.go); translations - the interface's languages downloaded
// from the translations catalog (translations_cmd.go).
var commands = map[string]func(args []string) int{
	"migrate":      runMigrate,
	"install":      runInstall,
	"theme":        runTheme,
	"module":       runModule,
	"assets":       runAssets,
	"config":       runConfig,
	"sites":        runSites,
	"backup":       runBackup,
	"files":        runFiles,
	"search":       runSearch,
	"translations": runTranslations,
}

// appUsage - "app help", and what an unknown command prints.
const appUsage = `usage (-config <file> goes before a command; without it - CONFIG_PATH):
  app [-config <file>]             serve the site
  app migrate [-apply [-additive]|-baseline]
                                   the schema migrations and data steps
  app install -site-name … -admin-name … -admin-email … [-language ru] [-demo | -demo-set …] [-theme …] [-modules …]
                                   a new site on an empty database
  app theme <command> …            the themes from their own repositories ("app theme" lists its commands)
  app module list|add|remove …     the modules: the binary's, and adding one from its own repository ("app module")
  app assets export <dir>          the modules' static files, for a web server serving /static/ itself
  app config [-json]               the site's settings, without secrets
  app backup <file>                the site's database dumped to file (.gz - compressed; mysqldump or pg_dump)
  app files migrate [-private] [-to-local]
                                   the public (or private) files copied from their directory to the bucket, or back
  app search reindex               the search index written again: the words of all the content
  app translations list|add|update|remove …
                                   the interface's languages from the translations catalog ("app translations")
  app sites list|add|update|rollback …
                                   the sites sharing this engine on a server ("app sites" lists its commands)
  app help                         this list`

// dispatch runs the command args name: its exit code, true. False - no
// command (no arguments, or only the server's flags: -config …): serve.
// The server's flags may come before a command ("app -config x migrate":
// config.Load reads -config). An unknown command is an error, never
// the server: a binary older than a command would otherwise start
// serving instead of running it.
func dispatch(args []string, stderr io.Writer) (int, bool) {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if args[i] == "-config" || args[i] == "--config" {
			i++ // its value
		}
		i++
	}
	if i >= len(args) {
		return 0, false
	}
	switch name := args[i]; {
	case name == "help":
		_, _ = fmt.Fprintln(stderr, appUsage)
		return 0, true
	case commands[name] != nil:
		return commands[name](args[i+1:]), true
	default:
		_, _ = fmt.Fprintf(stderr, "app: unknown command %q\n%s\n", name, appUsage)
		return 2, true
	}
}
