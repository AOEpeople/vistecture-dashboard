package main

import (
	"flag"
	"net/http"
	"time"

	"github.com/AOEpeople/vistecture-dashboard/v2/src/interfaces"
)

func main() {
	flag.Set("alsologtostderr", "true")

	d := &interfaces.DashboardController{}
	flag.StringVar(&d.ProjectPath, "config", "example/project.yml", "Path to project config")
	flag.StringVar(&d.Templates, "Templates", "templates/dashboard", "Path to dashboard.html and static/ folder")
	flag.StringVar(&d.Listen, "Listen", ":8080", "server Listen address")
	flag.StringVar(&d.LogFormat, "logformat", "plain", "logformat plain|json")
	flag.StringVar(&d.LogLevel, "loglevel", "warn", "fatal|error|warn|info|debug|trace")
	flag.BoolVar(&d.DemoMode, "Demo", false, "Demo mode (for templating, demo)")

	flag.Parse()

	http.DefaultClient.Timeout = 10 * time.Second

	err := d.Server()
	if err != nil {
		panic("Error while starting server: " + err.Error())
	}
}
