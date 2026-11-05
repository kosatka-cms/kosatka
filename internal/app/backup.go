package app

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/config"
)

// runBackup is "app backup <file>": the site's database dumped into file
// (gzip when it ends in .gz) with the database's own tool - mysqldump or
// pg_dump, installed with the database client - which reads the password
// from its environment, never its command line.
func runBackup(args []string) int {
	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: app backup <file>   (a .gz file is compressed)")
		return 2
	}
	if err := backupDatabase(loadConfig().DB, fs.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, "backup:", err)
		return 1
	}
	fmt.Printf("backup: %s\n", fs.Arg(0))
	return 0
}

// dumpCommand - the tool dumping db, its arguments and the environment
// it takes the password from.
func dumpCommand(db config.DBConfig) (name string, args []string, env []string) {
	port := strconv.Itoa(db.Port)
	if db.IsPostgres() {
		return "pg_dump", []string{"--no-owner", "-h", db.Host, "-p", port, "-U", db.Username, "-d", db.Scheme}, []string{"PGPASSWORD=" + db.Password}
	}
	return "mysqldump", []string{"--single-transaction", "--routines", "--triggers", "--no-tablespaces",
		"-h", db.Host, "-P", port, "-u", db.Username, db.Scheme}, []string{"MYSQL_PWD=" + db.Password}
}

// backupDatabase writes db's dump to file; a failed dump leaves no file.
func backupDatabase(db config.DBConfig, file string) error {
	name, args, env := dumpCommand(db)
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("%s not found: install the database client", name)
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	var w io.Writer = f
	var gz *gzip.Writer
	if strings.HasSuffix(file, ".gz") {
		gz = gzip.NewWriter(f)
		w = gz
	}
	var stderr strings.Builder
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = w, &stderr
	err = cmd.Run()
	if gz != nil {
		err = errors.Join(err, gz.Close())
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		_ = os.Remove(file)
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
