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
)

type (
	dashboard struct {
		projectPath string
		listen      string
		templates   string
		demo        bool
	}

	deployment struct {
		Name        string
		State       uint
		Alive       string
		Ingress     []ingress
		Version     []string
		K8s         apps.Deployment
		Healthcheck string
	}

	ingress struct {
		URL    string
		Alive  bool
		Status string
	}

	templateData struct {
		Failed, Unhealthy, Healthy, Unknown []deployment
		Now                                 time.Time
	}
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
)

const (
	unknown = iota
	failed
	unhealthy
	healthy
)

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

func checkAlive(wg *sync.WaitGroup, d *deployment) {
	for i, ing := range d.Ingress {
		statusText := fmt.Sprintf("Replica #%d //  ", i+1)

		r, httpErr := http.Get("https://" + ing.URL + d.Healthcheck)

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
			d.Ingress[i].Status = "Healthcheck not readable"
			continue
		}

		jsonError := json.Unmarshal(responseBody, jsonMap)

		// Check if Response is valid
		if jsonError != nil {
			d.State = unhealthy
			d.Ingress[i].Status = "Healthcheck Format Error"
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
			statusText = fmt.Sprintf("%s %s", r.StatusCode, r.Status)
		}
		d.Ingress[i].Status = statusText
	}
	wg.Done()
}

func (d *dashboard) load() ([]*deployment, error) {
	project := loadProject(d.projectPath)

	var deployments *apps.DeploymentList
	var ingresses *extensions.IngressList

	if d.demo {
		deployments = demoDeployments()
		ingresses = demoIngresses()
	} else {
		client, err := KubeClientFromConfig()
		if err != nil {
			return nil, err
		}

		deploymentClient := client.clientset.AppsV1beta1().Deployments(client.namespace)
		deployments, err = deploymentClient.List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}

		ingressClient := client.clientset.ExtensionsV1beta1().Ingresses(client.namespace)
		ingresses, err = ingressClient.List(metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
	}

	deploymentIndex := make(map[string]apps.Deployment, len(deployments.Items))
	for _, deployment := range deployments.Items {
		deploymentIndex[deployment.Name] = deployment
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

	var deploymentlist []*deployment
	var wg = new(sync.WaitGroup)

	for _, application := range project.Applications {
		name := application.Name
		if d, ok := application.Properties["deployment"]; !ok || d != "kubernetes" {
			continue
		}
		if n, ok := application.Properties["kubernetes-name"]; ok && n != "" {
			name = n
		}

		depl, exists := deploymentIndex[name]

		d := deployment{
			Name:  name,
			State: unknown,
		}

		if exists {
			d.K8s = depl
			d.State = failed

			if h, ok := application.Properties["healthcheck"]; ok {
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

			if len(ingressIndex[name]) > 0 && d.State != failed {
				d.Ingress = ingressIndex[name]
				wg.Add(1)
				go checkAlive(wg, &d)
			}
		}

		deploymentlist = append(deploymentlist, &d)
	}

	wg.Wait()

	return deploymentlist, nil
}

func e(rw http.ResponseWriter, err error) {
	rw.WriteHeader(http.StatusInternalServerError)
	rw.Header().Set("content-type", "text/plain")
	fmt.Fprintf(rw, "%+v", err)
}

func (d *dashboard) handler(rw http.ResponseWriter, r *http.Request) {
	deployments, err := d.load()

	if err != nil {
		e(rw, err)
		return
	}

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

	viewdata := templateData{
		Now: time.Now(),
	}

	for _, d := range deployments {
		switch d.State {
		case unknown:
			viewdata.Unknown = append(viewdata.Unknown, *d)
		case unhealthy:
			viewdata.Unhealthy = append(viewdata.Unhealthy, *d)
		case failed:
			viewdata.Failed = append(viewdata.Failed, *d)
		case healthy:
			viewdata.Healthy = append(viewdata.Healthy, *d)
		}
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

// AnalyzeAction controller action
func (d *dashboard) Server() error {
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(path.Join(d.templates, "static")))))
	http.HandleFunc("/", d.handler)

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
