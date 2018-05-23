package kube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"strings"

	"github.com/AOEpeople/vistecture-dashboard/src/model/vistecture"
	vistectureCore "github.com/AOEpeople/vistecture/model/core"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	v1Batch "k8s.io/client-go/pkg/apis/batch/v1"
)

type (
	StatusFetcher struct {
		mu                    *sync.RWMutex
		apps                  map[string]AppDeploymentInfo
		vistectureProjectPath string
		KubeInfoService       KubeInfoServiceInterface
	}

	// AppDeploymentInfo wraps Info on any Deployment's Data
	AppDeploymentInfo struct {
		Name          string
		State         uint
		StateReason   string
		Ingress       []K8sIngressInfo
		Images        []Image
		K8sDeployment apps.Deployment

		Healthcheck            string
		HealthyAlsoFromIngress bool
	}

	Image struct {
		Version  string
		FullPath string
	}

	// K8sIngressInfo holds Kubernetes Ingress Info
	K8sIngressInfo struct {
		URL   string
		Alive bool
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

func NewStatusFetcher(vistectureProjectPath string, demoMode bool) *StatusFetcher {
	statusManager := new(StatusFetcher)
	statusManager.mu = new(sync.RWMutex)
	statusManager.apps = make(map[string]AppDeploymentInfo)
	statusManager.vistectureProjectPath = vistectureProjectPath
	if demoMode {
		statusManager.KubeInfoService = &DemoService{}
	} else {
		statusManager.KubeInfoService = &KubeInfoService{}
	}

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
	log.Printf("Starting status fetcher for #%v apps (every %v sec)", len(definedApplications), refreshInterval)
	for range time.Tick(refreshInterval * time.Second) {
		// Add Deployments to Dashboard
		k8sDeployments, err := stm.KubeInfoService.GetKubernetesDeployments()

		if err != nil {
			panic("Could not get Deployment Config, check Configuration and Kubernetes Connection: " + err.Error())
		}

		// Add Ingresses
		ingresses, err := stm.KubeInfoService.GetIngressesByService()
		if err != nil {
			panic("Could not get Ingress Config, check Configuration and Kubernetes Connection" + err.Error())
		}

		// Add Services
		services, err := stm.KubeInfoService.GetServices()
		if err != nil {
			panic("Could not get Ingress Config, check Configuration and Kubernetes Connection" + err.Error())
		}

		// Add Jobs
		jobs, err := stm.KubeInfoService.GetJobs()
		if err != nil {
			panic("Could not get jobs Config, check Configuration and Kubernetes Connection" + err.Error())
		}

		log.Printf("Check run #%d", tickIteration)
		tickIteration++

		// results is a list of channels, which get filled by the fetcher
		var results []chan AppDeploymentInfo

		for _, app := range definedApplications {
			// Deployment is not on Kubernetes
			if di, ok := app.Properties["deployment"]; !ok || di != "kubernetes" {
				log.Printf("Skipping check for: %s (not configured as kubernetes service)", app.Name)
				continue
			}
			log.Printf("Checking: %s", app.Name)
			results = append(results, checkAppStatusInKubernetes(app, k8sDeployments, services, ingresses, jobs))
		}

		// exclusive lock map for write access
		stm.mu.Lock()

		// read all results in to map
		for _, result := range results {
			// get result from future
			status := <-result
			log.Printf(".. Result: %v %v %v", status.Name, status.State, status.StateReason)
			stm.apps[status.Name] = status
		}

		// unlock map
		stm.mu.Unlock()
	}
}

// checkAppStatusInKubernetes iterates through k8sDeployments and controls the result channel
func checkAppStatusInKubernetes(app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sServices map[string]v1.Service, k8sIngresses map[string][]K8sIngressInfo, k8sJobs map[string]v1Batch.Job) chan AppDeploymentInfo {
	// result (like a futures)
	res := make(chan AppDeploymentInfo, 1)

	// start fetcher routing
	go func(res chan AppDeploymentInfo) {

		// simulate status fetch
		time.Sleep(time.Second * 1)

		name := app.Name

		// Replace Name by configured Kubernetes Name
		if n, ok := app.Properties["k8sType"]; ok && n == "job" {
			d := checkJob(name, app, k8sJobs)
			res <- d
		} else {
			d := checkDeploymentWithHealthCheck(name, app, k8sDeployments, k8sServices, k8sIngresses)
			res <- d
		}

	}(res)

	return res
}

func checkJob(name string, app *vistectureCore.Application, k8sJobs map[string]v1Batch.Job) AppDeploymentInfo {
	_, exists := k8sJobs[name]

	d := AppDeploymentInfo{
		Name: name,
	}

	if !exists {
		d.State = State_unknown
		d.StateReason = "No job found"
		return d
	}
	//TODO - do real heath check of the job (e.g. check last run?)
	d.State = State_healthy
	return d
}

func checkDeploymentWithHealthCheck(name string, app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sServices map[string]v1.Service, k8sIngresses map[string][]K8sIngressInfo) AppDeploymentInfo {

	// Replace Name by configured Kubernetes Name
	if n, ok := app.Properties["k8sDeploymentName"]; ok && n != "" {
		name = n
	}

	depl, exists := k8sDeployments[name]

	d := AppDeploymentInfo{
		Name: name,
	}

	if !exists {
		d.State = State_unknown
		d.StateReason = "No deployment found"

		return d
	}

	//add ingresses found for kubernetes Name
	d.Ingress = k8sIngresses[name]

	d.K8sDeployment = depl

	for _, c := range depl.Spec.Template.Spec.Containers {
		d.Images = append(d.Images, buildImageStruct(c.Image))
	}

	if !activeDeploymentExists(depl) {
		d.State = State_failed
		d.StateReason = "No active deployment"
		return d
	}

	//Now check the service
	k8sHealthCheckServiceName := name
	if h, ok := app.Properties["k8sHealthCheckServiceName"]; ok {
		k8sHealthCheckServiceName = h
		//Add ingresses that might exists for seperate k8sHealthCheckServiceName
		d.Ingress = append(d.Ingress, k8sIngresses[k8sHealthCheckServiceName]...)
	}
	service, serviceExists := k8sServices[k8sHealthCheckServiceName]

	if !serviceExists {
		d.State = State_failed
		d.StateReason = "Deployment has no service for healthcheck that matches the config / " + k8sHealthCheckServiceName
		return d
	}
	if len(service.Spec.Ports) < 1 {
		d.State = State_failed
		d.StateReason = "Service has no port.. cannot check " + k8sHealthCheckServiceName
		return d
	}
	if h, ok := app.Properties["healthCheckPath"]; ok {
		d.Healthcheck = h
	}

	domain := fmt.Sprintf("%v:%v", k8sHealthCheckServiceName, service.Spec.Ports[0].Port)
	healthStatusOfService, reason := checkHealth("http://"+domain, app.Properties["healthCheckPath"])
	if !healthStatusOfService {
		d.State = State_failed
		d.StateReason = "Service Unhealthy: " + reason
		return d
	}

	if len(k8sIngresses[k8sHealthCheckServiceName]) > 0 && d.State != State_failed {
		d.HealthyAlsoFromIngress = checkPublicHealth(k8sIngresses[k8sHealthCheckServiceName], app.Properties["healthCheckPath"])
	}
	//In case the application need to be checked from outside:
	if _, ok := app.Properties["k8sHealthCheckThroughIngress"]; ok {
		if !d.HealthyAlsoFromIngress {
			d.State = State_failed
			if len(k8sIngresses[k8sHealthCheckServiceName]) == 0 {
				d.StateReason = "No Ingress for service " + k8sHealthCheckServiceName
			} else {
				d.StateReason = "Healthcheck from public ingress failed"
			}
			return d
		}
	}

	d.State = State_healthy
	return d
}

func activeDeploymentExists(deployment apps.Deployment) bool {
	for _, c := range deployment.Status.Conditions {
		if c.Type == apps.DeploymentAvailable && c.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// checkAlive calls the healthcheck of an application and returns the result
func checkPublicHealth(ingresses []K8sIngressInfo, healtcheckPath string) bool {
	for _, ing := range ingresses {
		//At least one ingress should succeed
		ok, _ := checkHealth("https://"+ing.URL, healtcheckPath)
		if ok {
			return true
		}
	}
	return false
}

func checkHealth(checkBaseUrl string, healtcheckPath string) (bool, string) {
	checkUrl := checkBaseUrl + healtcheckPath
	r, httpErr := http.Get(checkUrl)

	if httpErr != nil {
		return false, httpErr.Error()
	}

	statusCode := r.StatusCode

	if healtcheckPath != "" {
		//Parse healthcheck

		jsonMap := &HealthCheckResponse{
			Services: []HealthCheckService{},
		}

		responseBody, bodyErr := ioutil.ReadAll(r.Body)
		if bodyErr != nil {
			return false, "Could not read from Healthcheck"
		}
		jsonError := json.Unmarshal(responseBody, jsonMap)
		// Check if Response is valid
		if jsonError != nil {
			return false, fmt.Sprintf("Healthcheck Format Error from %s", checkUrl)
		}

		if statusCode != 200 {
			statusText := fmt.Sprintf("Status  %v for %v", statusCode, checkUrl)
			for _, service := range jsonMap.Services {
				if !service.Alive {
					statusText = statusText + fmt.Sprintf("%v (%v) \n", service.Name, service.Details)
				}
			}
			return false, statusText
		}
		return true, ""
	}

	//Fallback if no healthcheck is configured

	if statusCode > 500 {
		return false, fmt.Sprintf("Fallbackcheck returns error status %v ", statusCode)
	}

	return true, ""

}

func buildImageStruct(imageUrl string) Image {

	imageUrlInfos := strings.Split(imageUrl, ":")
	version := ""
	if len(imageUrlInfos) > 1 {
		version = imageUrlInfos[1]
	}

	return Image{
		FullPath: imageUrl,

		Version: version,
	}
}
