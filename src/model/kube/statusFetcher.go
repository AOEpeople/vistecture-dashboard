package kube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/AOEpeople/vistecture-dashboard/src/model/vistecture"
	vistectureCore "github.com/AOEpeople/vistecture/model/core"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type (
	StatusFetcher struct {
		mu                    *sync.RWMutex
		apps                  map[string]AppDeploymentInfo
		vistectureProjectPath string
	}

	// AppDeploymentInfo wraps Info on any Deployment's Data
	AppDeploymentInfo struct {
		Name        string
		State       uint
		Alive       string
		Ingress     []K8sIngressInfo
		Version     []string
		K8s         apps.Deployment
		Healthcheck string
	}

	// K8sIngressInfo holds Kubernetes Ingress Info
	K8sIngressInfo struct {
		URL    string
		Alive  bool
		Status string
	}

	// Response Wraps a list of Services
	HealthCheckResponse struct {
		Services []HealthCheckService `json:"services"`
	}

	// Service describes a Service the Application is dependent of, its Liveness and Details on its Status
	HealthCheckService struct {
		Name    string `json:"name"`
		Alive   bool   `json:"alive"`
		Details string `json:"details"`
	}
)

const (
	State_unknown = iota
	State_failed
	State_unhealthy
	State_healthy
	// This is the Interval for goroutine polling of kubernetes
	refreshInterval = 15
)

func NewStatusFetcher(vistectureProjectPath string) *StatusFetcher {
	statusManager := new(StatusFetcher)
	statusManager.mu = new(sync.RWMutex)
	statusManager.apps = make(map[string]AppDeploymentInfo)
	statusManager.vistectureProjectPath = vistectureProjectPath
	return statusManager
}

func (stm *StatusFetcher) GetCurrentResult() map[string]AppDeploymentInfo {
	stm.mu.RLock()

	// copy results to not leak a reference to the statusManager's map
	result := make(map[string]AppDeploymentInfo, len(stm.apps))

	for k, v := range stm.apps {
		result[k] = v
	}

	stm.mu.RUnlock()
	return result
}

// fetchStatusInRegularInterval controls the interval in which new info is fetched and loops over configured applications
func (stm *StatusFetcher) FetchStatusInRegularInterval() {
	var tickIteration = 0
	project := vistecture.LoadProject(stm.vistectureProjectPath)
	definedApplications := project.Applications

	for range time.Tick(refreshInterval * time.Second) {

		// Add Deployments to Dashboard
		k8sDeployments, err := getKubernetesDeployments()

		if err != nil {
			panic("Could not get Deployment Config, check Configuration and Kubernetes Connection: " + err.Error())
		}

		// Add Ingresses to Dasboard
		ingresses, err := getIngresses()

		if err != nil {
			panic("Could not get Ingress Config, check Configuration and Kubernetes Connection" + err.Error())
		}

		log.Printf("Check run #%d", tickIteration)
		tickIteration++

		// results is a list of channels, which get filled by the fetcher
		var results []chan AppDeploymentInfo

		for _, app := range definedApplications {
			// Deployment is not on Kubernetes
			if di, ok := app.Properties["deployment"]; !ok || di != "kubernetes" {
				log.Printf("Skipping check for: %s", app.Name)
				continue
			}

			log.Printf("Checking: %s", app.Name)

			results = append(results, checkAppStatusInKubernetes(app, k8sDeployments, ingresses))
		}

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

// checkAppStatusInKubernetes iterates through k8sDeployments and controls the result channel
func checkAppStatusInKubernetes(app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sIngresses map[string][]K8sIngressInfo) chan AppDeploymentInfo {
	// result (like a futures)
	res := make(chan AppDeploymentInfo, 1)

	// start fetcher routing
	go func(res chan AppDeploymentInfo) {

		// simulate status fetch
		time.Sleep(time.Second * 1)

		name := app.Name

		// Replace Name by configured Kubernetes Name
		if n, ok := app.Properties["kubernetes-name"]; ok && n != "" {
			name = n
		}

		depl, exists := k8sDeployments[name]

		d := AppDeploymentInfo{
			Name:  name,
			State: State_unknown,
		}

		if exists {
			d.K8s = depl
			d.State = State_failed

			if h, ok := app.Properties["healthcheck"]; ok {
				d.Healthcheck = h
			}

			for _, c := range depl.Status.Conditions {
				if c.Type == apps.DeploymentAvailable && c.Status == v1.ConditionTrue {
					d.State = State_healthy
				}
			}

			for _, c := range depl.Spec.Template.Spec.Containers {
				d.Version = append(d.Version, c.Image)
			}

			if len(k8sIngresses[name]) > 0 && d.State != State_failed {
				d.Ingress = k8sIngresses[name]
				checkAlive(&d)
			}
		}

		res <- d

	}(res)

	return res
}

// getKubernetesDeployments fetches from Config or Demo Data
func getKubernetesDeployments() (map[string]apps.Deployment, error) {
	var deployments *apps.DeploymentList

	client, err := KubeClientFromConfig()
	if err != nil {
		return nil, err
	}

	deploymentClient := client.Clientset.AppsV1beta1().Deployments(client.Namespace)
	deployments, err = deploymentClient.List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	deploymentIndex := make(map[string]apps.Deployment, len(deployments.Items))

	for _, deployment := range deployments.Items {
		deploymentIndex[deployment.Name] = deployment
	}

	return deploymentIndex, nil
}

// getIngresses fetches from Config or Demo Data
func getIngresses() (map[string][]K8sIngressInfo, error) {
	var ingresses *extensions.IngressList

	client, err := KubeClientFromConfig()

	if err != nil {
		return nil, err
	}

	ingressClient := client.Clientset.ExtensionsV1beta1().Ingresses(client.Namespace)
	ingresses, err = ingressClient.List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	ingressIndex := make(map[string][]K8sIngressInfo)

	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			for _, p := range rule.HTTP.Paths {
				name := p.Backend.ServiceName
				ingressIndex[name] = append(ingressIndex[name], K8sIngressInfo{URL: rule.Host + p.Path})
			}
		}
	}

	return ingressIndex, nil
}

// checkAlive calls the healthcheck of an application and returns the result
func checkAlive(d *AppDeploymentInfo) {
	for i, ing := range d.Ingress {
		statusText := fmt.Sprintf("Replica #%d //  ", i+1)

		checkUrl := "https://" + ing.URL + d.Healthcheck

		r, httpErr := http.Get(checkUrl)

		if httpErr != nil {
			d.State = State_unhealthy
			d.Ingress[i].Status = httpErr.Error()
			continue
		}

		statusCode := r.StatusCode

		jsonMap := &HealthCheckResponse{
			Services: []HealthCheckService{},
		}

		responseBody, bodyErr := ioutil.ReadAll(r.Body)

		if bodyErr != nil {
			d.State = State_unhealthy
			d.Ingress[i].Status = statusText + "Could not read from Healthcheck"
			continue
		}

		jsonError := json.Unmarshal(responseBody, jsonMap)

		// Check if Response is valid
		if jsonError != nil {
			d.State = State_unhealthy
			d.Ingress[i].Status = statusText + fmt.Sprintf("Healthcheck Format Error from %s", checkUrl)
			continue
		}

		if statusCode >= 500 {
			d.State = State_unhealthy
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
