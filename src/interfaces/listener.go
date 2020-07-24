package interfaces

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/AOEpeople/vistecture-dashboard/v2/src/model/kube"
	"github.com/AOEpeople/vistecture-dashboard/v2/src/model/vistecture"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type (
	DashboardController struct {
		ProjectPath string
		Templates   string
		Listen      string
		DemoMode    bool
		LogFormat   string
		LogLevel    string
	}

	ByName []kube.AppDeploymentInfo

	// templateData holds info for Dashboard Rendering
	templateData struct {
		Failed, Unhealthy, Healthy, Unknown, Unstable []kube.AppDeploymentInfo
		Now                                           time.Time
	}
)

func (d *DashboardController) initLogger() {
	if d.LogFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if d.LogLevel == "fatal" {
		log.SetLevel(log.FatalLevel)
	}
	if d.LogLevel == "error" {
		log.SetLevel(log.ErrorLevel)
	}
	if d.LogLevel == "warn" {
		log.SetLevel(log.WarnLevel)
	}
	if d.LogLevel == "info" {
		log.SetLevel(log.InfoLevel)
	}
	if d.LogLevel == "debug" {
		log.SetLevel(log.DebugLevel)
	}
	if d.LogLevel == "trace" {
		log.SetLevel(log.TraceLevel)
	}
}

// Server defines controller actions
func (d *DashboardController) Server() error {
	d.initLogger()
	//load once (will panic before we start listen)
	project := vistecture.LoadProject(d.ProjectPath)

	//Prepare the status fetcher (will run in background and starts regual checks)
	statusFetcher := kube.NewStatusFetcher(project.Applications, d.DemoMode)
	go statusFetcher.FetchStatusInRegularInterval()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(d.Templates, "static")))))
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		d.dashBoardHandler(w, r, statusFetcher)
	})

	log.Infoln("Listening on http://" + d.Listen + "/")
	return http.ListenAndServe(d.Listen, nil)
}

// dashBoardHandler handles the view Request
func (d *DashboardController) dashBoardHandler(rw http.ResponseWriter, r *http.Request, statusFetcher *kube.StatusFetcher) {
	viewdata := templateData{
		Now: time.Now(),
	}
	result := statusFetcher.GetCurrentResult()
	for _, deployment := range result {
		switch deployment.AppStateInfo.State {
		case kube.State_unknown:
			viewdata.Unknown = append(viewdata.Unknown, deployment)
		case kube.State_unhealthy:
			viewdata.Unhealthy = append(viewdata.Unhealthy, deployment)
		case kube.State_failed:
			viewdata.Failed = append(viewdata.Failed, deployment)
		case kube.State_healthy:
			viewdata.Healthy = append(viewdata.Healthy, deployment)
		case kube.State_unstable:
			viewdata.Unstable = append(viewdata.Unstable, deployment)
		}
	}

	sort.Sort(ByName(viewdata.Unknown))
	sort.Sort(ByName(viewdata.Unhealthy))
	sort.Sort(ByName(viewdata.Failed))
	sort.Sort(ByName(viewdata.Healthy))

	d.renderDashboardStatus(rw, viewdata)
}

// renderDashboardStatus passes Viewdata to Template
func (d *DashboardController) renderDashboardStatus(rw http.ResponseWriter, viewdata templateData) {
	tpl := template.New("dashboard")

	tpl.Funcs(template.FuncMap{
		"unknown":   func() uint { return kube.State_unknown },
		"unhealthy": func() uint { return kube.State_unhealthy },
		"failed":    func() uint { return kube.State_failed },
		"healthy":   func() uint { return kube.State_healthy },
		"unstable":  func() uint { return kube.State_unstable },
	})

	b, err := ioutil.ReadFile(path.Join(d.Templates, "dashboard.html"))

	if err != nil {
		e(rw, err)
		return
	}

	tpl, err = tpl.Parse(string(b))

	if err != nil {
		e(rw, err)
		return
	}

	buf := new(bytes.Buffer)
	err = tpl.ExecuteTemplate(buf, "dashboard", viewdata)

	if err != nil {
		e(rw, err)
		return
	}

	rw.Header().Set("content-type", "text/html")
	rw.WriteHeader(http.StatusOK)
	io.Copy(rw, buf)
}

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// e is the Error Handler
func e(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Header().Set("content-type", "text/plain")
	fmt.Fprintf(rw, "%+v", err)
}
