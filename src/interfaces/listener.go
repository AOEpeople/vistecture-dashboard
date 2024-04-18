package interfaces

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/AOEpeople/vistecture-dashboard/v2/src/model/kube"
	"github.com/AOEpeople/vistecture-dashboard/v2/src/model/vistecture"
)

type (
	DashboardController struct {
		ProjectPath     string
		Templates       string
		Listen          string
		IgnoredServices []string
		DemoMode        bool
	}

	ByName []kube.AppDeploymentInfo

	// templateData holds info for Dashboard Rendering
	templateData struct {
		Failed, Unhealthy, Healthy, Unknown, Unstable, Ignored []kube.AppDeploymentInfo
		Now                                                    time.Time
	}
)

// Server defines controller actions
func (d *DashboardController) Server() error {
	// load once (will panic before we start listen)
	project := vistecture.LoadProject(d.ProjectPath)

	var fakeHealthcheckPort int32
	if d.DemoMode {
		portReceive := make(chan int32)
		go serveDemoHealthCheck(portReceive)
		fakeHealthcheckPort = <-portReceive
	}

	// Prepare the status fetcher (will run in background and starts regual checks)
	statusFetcher := kube.NewStatusFetcher(project.Applications, d.DemoMode, fakeHealthcheckPort)
	go statusFetcher.FetchStatusInRegularInterval(d.IgnoredServices)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(d.Templates, "static")))))
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		d.dashBoardHandler(w, r, statusFetcher)
	})

	log.Println("Listening on http://" + d.Listen + "/")
	return http.ListenAndServe(d.Listen, nil)
}

// dashBoardHandler handles the view Request
func (d *DashboardController) dashBoardHandler(rw http.ResponseWriter, _ *http.Request, statusFetcher *kube.StatusFetcher) {
	viewdata := templateData{
		Now: time.Now(),
	}
	result := statusFetcher.GetCurrentResult()
	for _, deployment := range result {
		switch deployment.AppStateInfo.State {
		case kube.State_ignored:
			viewdata.Ignored = append(viewdata.Ignored, deployment)
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

	sort.Sort(ByName(viewdata.Ignored))
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
		"ignored":   func() uint { return kube.State_ignored },
		"unknown":   func() uint { return kube.State_unknown },
		"unhealthy": func() uint { return kube.State_unhealthy },
		"failed":    func() uint { return kube.State_failed },
		"healthy":   func() uint { return kube.State_healthy },
		"unstable":  func() uint { return kube.State_unstable },
		"splitLines": func(s string) []string {
			return strings.Split(s, "\n")
		},
	})

	b, err := os.ReadFile(path.Join(d.Templates, "dashboard.html"))

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
	_, _ = io.Copy(rw, buf)
}

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// e is the Error Handler
func e(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Header().Set("content-type", "text/plain")
	_, _ = fmt.Fprintf(rw, "%+v", err)
}

func serveDemoHealthCheck(fakeHealthcheckPort chan<- int32) {
	ln, err := net.Listen("tcp", "")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Demo mode enabled: start fake health check on " + ln.Addr().String())
	_, p, _ := net.SplitHostPort(ln.Addr().String())
	pp, _ := strconv.Atoi(p)
	fakeHealthcheckPort <- int32(pp)

	err = http.Serve(ln, http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	if err != nil {
		log.Fatal("fake health check failed: ", err)
	}
}
