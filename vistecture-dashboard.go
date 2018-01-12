package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"path"
	"sync"
	"time"

	"io/ioutil"

	"github.com/AOEpeople/vistecture/model/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"sort"
)

type (
	// dasboard is a general info object for the dasboard application
	dashboard struct {
		projectPath string
		listen      string
		templates   string
		demo        bool
		ingresses   map[string][]ingress
		deployments map[string]apps.Deployment
	}

	// deployment wraps Info on any Deployment's Data
	deployment struct {
		Name        string
		State       uint
		Alive       string
		Ingress     []ingress
		Version     []string
		K8s         apps.Deployment
		Healthcheck string
	}

	// ingress holds Kubernetes Ingress Info
	ingress struct {
		URL    string
		Alive  bool
		Status string
	}

	// templateData holds info for Dashboard Rendering
	templateData struct {
		Failed, Unhealthy, Healthy, Unknown []deployment
		Now                                 time.Time
	}

	ByName []deployment

	// Response Wraps a list of Services
	Response struct {
		Services []Service `json:"services"`
	}

	// Service describes a Service the Application is dependent of, its Liveness and Details on its Status
	Service struct {
		Name    string `json:"name"`
		Alive   bool   `json:"alive"`
		Details string `json:"details"`
	}

	// StatusManager controls R/W Mutex for result fetching
	StatusManager struct {
		mu   *sync.RWMutex
		apps map[string]deployment
	}
)

const (
	unknown   = iota
	failed
	unhealthy
	healthy
	// This is the Interval for goroutine polling of kubernetes
	refreshInterval = 15
)

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }

// loadProject loads the json file from a project folder
func loadProject(path string) *core.Project {
	project, err := core.CreateProject(path)

	if err != nil {
		log.Fatal("Project JSON is not valid:", err)
	}

	err = project.Validate()

	if err != nil {
		log.Fatal("Validation Errors:", err)
	}

	return project
}

// checkAlive calls the healthcheck of an application and returns the result
func checkAlive(d *deployment) {
	for i, ing := range d.Ingress {
		statusText := fmt.Sprintf("Replica #%d //  ", i+1)

		checkUrl := "https://" + ing.URL + d.Healthcheck

		r, httpErr := http.Get(checkUrl)

		if httpErr != nil {
			d.State = unhealthy
			d.Ingress[i].Status = httpErr.Error()
			continue
		}

		statusCode := r.StatusCode

		jsonMap := &Response{
			Services: []Service{},
		}

		responseBody, bodyErr := ioutil.ReadAll(r.Body)

		if bodyErr != nil {
			d.State = unhealthy
			d.Ingress[i].Status = statusText + "Could not read from Healthcheck"
			continue
		}

		jsonError := json.Unmarshal(responseBody, jsonMap)

		// Check if Response is valid
		if jsonError != nil {
			d.State = unhealthy
			d.Ingress[i].Status = statusText + fmt.Sprintf("Healthcheck Format Error from from %s", checkUrl)
			continue
		}

		if statusCode >= 500 {
			d.State = unhealthy
			for _, service := range jsonMap.Services {
				if !service.Alive {
					statusText = statusText + fmt.Sprintf("Application \"%s\" reports Error: \"%s\" // ", service.Name, service.Details)
				}
			}
		} else {
			d.Ingress[i].Alive = true
			statusText = string(r.Status)
		}
		d.Ingress[i].Status = statusText
	}
}

// e is the Error Handler
func e(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Header().Set("content-type", "text/plain")
	fmt.Fprintf(rw, "%+v", err)
}

// getDeployments fetches from Config or Demo Data
func (d *dashboard) getDeployments() (map[string]apps.Deployment, error) {
	var deployments *apps.DeploymentList

	if d.demo {
		deployments = demoDeployments()
	} else {
		client, err := kubeClientFromConfig()
		if err != nil {
			return nil, err
		}

		deploymentClient := client.clientset.AppsV1beta1().Deployments(client.namespace)
		deployments, err = deploymentClient.List(metav1.ListOptions{})

		if err != nil {
			return nil, err
		}
	}

	deploymentIndex := make(map[string]apps.Deployment, len(deployments.Items))

	for _, deployment := range deployments.Items {
		deploymentIndex[deployment.Name] = deployment
	}

	return deploymentIndex, nil
}

// getIngresses fetches from Config or Demo Data
func (d *dashboard) getIngresses() (map[string][]ingress, error) {
	var ingresses *extensions.IngressList

	if d.demo {
		ingresses = demoIngresses()
	} else {
		client, err := kubeClientFromConfig()

		if err != nil {
			return nil, err
		}

		ingressClient := client.clientset.ExtensionsV1beta1().Ingresses(client.namespace)
		ingresses, err = ingressClient.List(metav1.ListOptions{})

		if err != nil {
			return nil, err
		}
	}

	ingressIndex := make(map[string][]ingress)

	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			for _, p := range rule.HTTP.Paths {
				name := p.Backend.ServiceName
				ingressIndex[name] = append(ingressIndex[name], ingress{URL: rule.Host + p.Path})
			}
		}
	}

	return ingressIndex, nil
}

// checkAppStatus iterates through deployments and controls the result channel
func (stm *StatusManager) checkAppStatus(app *core.Application, dash *dashboard, wg *sync.WaitGroup) chan deployment {
	// result (like a futures)
	res := make(chan deployment, 1)

	// start fetcher routing
	go func(res chan deployment, wg *sync.WaitGroup) {

		// simulate status fetch
		time.Sleep(time.Second * 1)

		name := app.Name

		// Replace Name by configured Kubernetes Name
		if n, ok := app.Properties["kubernetes-name"]; ok && n != "" {
			name = n
		}

		depl, exists := dash.deployments[name]

		d := deployment{
			Name:  name,
			State: unknown,
		}

		if exists {
			d.K8s = depl
			d.State = failed

			if h, ok := app.Properties["healthcheck"]; ok {
				d.Healthcheck = h
			}

			for _, c := range depl.Status.Conditions {
				if c.Type == apps.DeploymentAvailable && c.Status == v1.ConditionTrue {
					d.State = healthy
				}
			}

			for _, c := range depl.Spec.Template.Spec.Containers {
				d.Version = append(d.Version, c.Image)
			}

			if len(dash.ingresses[name]) > 0 && d.State != failed {
				d.Ingress = dash.ingresses[name]
				checkAlive(&d)
			}
		}

		res <- d

		// mark this routine as done
		wg.Done()
	}(res, wg)

	return res
}

// fetchStatus controls the interval in which new info is fetched and loops over configured applications
func (stm *StatusManager) fetchStatus(d *dashboard) {
	project := loadProject(d.projectPath)

	// Add Deployments to Dashboard
	deployments, err := d.getDeployments()

	if err != nil {
		panic("Could not get Deployment Config, check Configuration and Kubernetes Connection")
	}

	d.deployments = deployments

	// Add Ingresses to Dasboard
	ingresses, err := d.getIngresses()

	if err != nil {
		panic("Could not get Ingress Config, check Configuration and Kubernetes Connection")
	}

	d.ingresses = ingresses

	var tickIteration = 0

	for range time.Tick(refreshInterval * time.Second) {
		log.Printf("Check run #%d", tickIteration)
		tickIteration++

		applications := project.Applications

		// waitgroup to wait for all sub-fetcher
		wg := new(sync.WaitGroup)

		// results is a list of channels, which get filled by the fetcher
		var results []chan deployment

		for _, app := range applications {
			// Deployment is not on Kubernetes
			if d, ok := app.Properties["deployment"]; !ok || d != "kubernetes" {
				continue
			}

			wg.Add(1)

			log.Printf("Checking: %s", app.Name)

			results = append(results, stm.checkAppStatus(app, d, wg))
		}

		// wait for all status updates to be available
		wg.Wait()

		// exclusive lock map for write access
		stm.mu.Lock()

		// read all results in to map
		for _, result := range results {
			// get result from future
			status := <-result
			stm.apps[status.Name] = status
		}

		// unlock map
		stm.mu.Unlock()
	}
}

// renderStatus passes Viewdata to Template
func (d *dashboard) renderStatus(rw http.ResponseWriter, viewdata templateData) {
	tpl := template.New("dashboard")

	tpl.Funcs(template.FuncMap{
		"unknown":   func() uint { return unknown },
		"unhealthy": func() uint { return unhealthy },
		"failed":    func() uint { return failed },
		"healthy":   func() uint { return healthy },
	})

	b, err := ioutil.ReadFile(path.Join(d.templates, "dashboard.html"))

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

// handleStatus handles the view Request
func (stm *StatusManager) handleStatus(rw http.ResponseWriter, r *http.Request, d *dashboard) {
	viewdata := templateData{
		Now: time.Now(),
	}

	stm.mu.RLock()

	// copy results to not leak a reference to the statusManager's map
	result := make(map[string]deployment, len(stm.apps))

	for k, v := range stm.apps {
		result[k] = v
	}

	stm.mu.RUnlock()

	for _, deployment := range result {
		switch deployment.State {
		case unknown:
			viewdata.Unknown = append(viewdata.Unknown, deployment)
		case unhealthy:
			viewdata.Unhealthy = append(viewdata.Unhealthy, deployment)
		case failed:
			viewdata.Failed = append(viewdata.Failed, deployment)
		case healthy:
			viewdata.Healthy = append(viewdata.Healthy, deployment)
		}
	}

	sort.Sort(ByName(viewdata.Unknown))
	sort.Sort(ByName(viewdata.Unhealthy))
	sort.Sort(ByName(viewdata.Failed))
	sort.Sort(ByName(viewdata.Healthy))

	d.renderStatus(rw, viewdata)
}

// Server defines controller actions
func (d *dashboard) Server() error {
	statusManager := new(StatusManager)
	statusManager.mu = new(sync.RWMutex)
	statusManager.apps = make(map[string]deployment)

	go statusManager.fetchStatus(d)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(d.templates, "static")))))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		statusManager.handleStatus(w, r, d)
	})

	log.Println("Listening on http://" + d.listen + "/")
	return http.ListenAndServe(d.listen, nil)
}

func main() {
	flag.Set("alsologtostderr", "true")

	d := &dashboard{}
	flag.StringVar(&d.projectPath, "config", "../project", "Path to project config")
	flag.StringVar(&d.templates, "templates", "templates/dashboard", "Path to dashboard.html and static/ folder")
	flag.StringVar(&d.listen, "listen", ":8080", "server listen address")
	flag.BoolVar(&d.demo, "demo", false, "demo mode (for templating)")

	flag.Parse()

	loadProject(d.projectPath)

	http.DefaultClient.Timeout = 2 * time.Second

	d.Server()
}
