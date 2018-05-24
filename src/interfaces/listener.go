package interfaces

import (
	"bytes"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"sort"
	"time"

	"fmt"

	"github.com/AOEpeople/vistecture-dashboard/src/model/kube"
	"github.com/AOEpeople/vistecture-dashboard/src/model/vistecture"
)

type (
	DashboardController struct {
		ProjectPath string
		Templates   string
		Listen      string
		DemoMode    bool
	}

	ByName []kube.AppDeploymentInfo

	// templateData holds info for Dashboard Rendering
	templateData struct {
		Failed, Unhealthy, Healthy, Unknown, Unstable []kube.AppDeploymentInfo
		Now                                           time.Time
	}
)

// Server defines controller actions
func (d *DashboardController) Server() error {
	//load once (will panic before we start listen)
	vistecture.LoadProject(d.ProjectPath)

	//Prepare the status fetcher (will run in background and starts regual checks)
	statusFetcher := kube.NewStatusFetcher(d.ProjectPath, d.DemoMode)
	go statusFetcher.FetchStatusInRegularInterval()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(d.Templates, "static")))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		d.dashBoardHandler(w, r, statusFetcher)
	})

	log.Println("Listening on http://" + d.Listen + "/")
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
