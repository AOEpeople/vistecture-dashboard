package main

import (
	"flag"
	"net/http"
	"time"

	"github.com/AOEpeople/vistecture-dashboard/src/interfaces"
)

func main() {
	flag.Set("alsologtostderr", "true")

	d := &interfaces.DashboardController{}
	flag.StringVar(&d.ProjectPath, "config", "../project", "Path to project config")
	flag.StringVar(&d.Templates, "Templates", "Templates/dashboard", "Path to dashboard.html and static/ folder")
	flag.StringVar(&d.Listen, "Listen", ":8080", "server Listen address")
	//flag.BoolVar(&d.Demo, "Demo", false, "Demo mode (for templating)")

	flag.Parse()

	http.DefaultClient.Timeout = 2 * time.Second

	d.Server()

}
