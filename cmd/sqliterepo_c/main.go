// Package main provides the sqliterepo_c command.
package main

import "github.com/define42/createrepo_go/internal/cli"

func main() {
	cli.Main(cli.RunSQLite)
}
