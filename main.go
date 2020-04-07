// Package main contains a CLI to help sizing a Newts cluster
//
// @author Alejandro Galue <agalue@opennms.com>
package main

import (
	"fmt"
	"os"

	"github.com/agalue/newts-sizing/cli/analysis"
	"github.com/agalue/newts-sizing/cli/sizing"
	"github.com/urfave/cli"
)

var (
	version = "v1.0.3"
)

func main() {
	var app = cli.NewApp()
	initCliInfo(app)
	initCliCommands(app)

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

func initCliInfo(app *cli.App) {
	app.Name = "newts-sizing"
	app.Usage = "A CLI to help sizing a Cassandra/ScyllaDB cluster for Newts"
	app.Author = "Alejandro Galue"
	app.Email = "agalue@opennms.com"
	app.Version = version
	app.EnableBashCompletion = true
}

func initCliCommands(app *cli.App) {
	app.Commands = []cli.Command{
		analysis.Command,
		sizing.Command,
	}
}
