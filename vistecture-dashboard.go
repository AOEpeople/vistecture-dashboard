package main

import (
	"flag"
	"net/http"
	"strings"
	"time"

	"github.com/AOEpeople/vistecture-dashboard/v2/src/interfaces"
)

type (
	listFlag []string
)

func (l *listFlag) String() string {
	return strings.Join(*l, ",")
}

func (l *listFlag) Set(s string) error {
	*l = append(*l, s)
	return nil
}

func main() {
	_ = flag.Set("alsologtostderr", "true")

	var ignoredServices listFlag

	d := &interfaces.DashboardController{}
	flag.StringVar(&d.ProjectPath, "config", "example/project.yml", "Path to project config")
	flag.StringVar(&d.Templates, "Templates", "templates/dashboard", "Path to dashboard.html and static/ folder")
	flag.StringVar(&d.Listen, "Listen", ":8080", "server Listen address")
	flag.Var(&ignoredServices, "ignore", "services to exclude from checks")
	flag.BoolVar(&d.DemoMode, "Demo", false, "Demo mode (for templating, demo)")

	flag.Parse()

	d.IgnoredServices = ignoredServices

	http.DefaultClient.Timeout = 10 * time.Second

	err := d.Server()
	if err != nil {
		panic("Error while starting server: " + err.Error())
	}
}
