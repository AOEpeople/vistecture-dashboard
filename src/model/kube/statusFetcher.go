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
	"github.com/prometheus/client_golang/prometheus"
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
		Name                string
		Ingress             []K8sIngressInfo
		Images              []Image
		K8sDeployment       apps.Deployment
		AppStateInfo        AppStateInfo
		HealthcheckPath     string
		ApiDocumentationUrl string
		VistectureApp       vistectureCore.Application
	}

	AppStateInfo struct {
		State                  uint
		StateReason            string
		HealthCheckType        string
		HealthyAlsoFromIngress bool
	}

	Image struct {
		Version  string
		FullPath string
	}

	// K8sIngressInfo holds Kubernetes Ingress Info
	K8sIngressInfo struct {
		URL   string
		Host  string
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

var (
	healthcheck = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "application_healthcheck_ok",
		Help: "Application Healthcheck OK Status",
	}, []string{
		"application",
	})
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(healthcheck)
}

const (
	State_unknown = iota
	State_failed
	State_unhealthy
	State_healthy
	State_unstable
	// This is the Interval for goroutine polling of kubernetes
	refreshInterval = 15

	HealthCheckType_NotCheckedYet = ""
	HealthCheckType_SimpleCheck   = "simple"
	HealthCheckType_HealthCheck   = "healthcheck"
	HealthCheckType_Job           = "job"
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
	lastResults := make(map[string][]AppDeploymentInfo)
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
		jobs, err := stm.KubeInfoService.GetJobsByApp()
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
			log.Printf(".. Result: %v %v %v", status.Name, status.AppStateInfo.State, status.AppStateInfo.StateReason)

			lastResults[status.Name] = append(lastResults[status.Name], status)
			if len(lastResults[status.Name]) > 20 {
				//delete last if more than 20
				lastResults[status.Name] = lastResults[status.Name][:len(lastResults[status.Name])-1]
			}

			//mark as unstable if in last 10 was a failure
			if status.AppStateInfo.State == State_healthy {
				for _, lastStatus := range lastResults[status.Name] {
					if lastStatus.AppStateInfo.State == State_failed || lastStatus.AppStateInfo.State == State_unhealthy {
						status.AppStateInfo.State = State_unstable
						status.AppStateInfo.StateReason = fmt.Sprintf("Failed check in last 20 checks: %v / %v", lastStatus.AppStateInfo.State, lastStatus.AppStateInfo.StateReason)
					}
				}
			}

			stm.apps[status.Name] = status
			switch status.AppStateInfo.State {
			case State_healthy:
				healthcheck.With(prometheus.Labels{"application":status.Name}).Set(2)
			case State_unhealthy, State_unstable:
				healthcheck.With(prometheus.Labels{"application":status.Name}).Set(1)
			default:
				healthcheck.With(prometheus.Labels{"application":status.Name}).Set(0)
			}

		}

		// unlock map
		stm.mu.Unlock()
	}
}

// checkAppStatusInKubernetes iterates through k8sDeployments and controls the result channel
func checkAppStatusInKubernetes(app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sServices map[string]v1.Service, k8sIngresses map[string][]K8sIngressInfo, k8sJobs map[string][]v1Batch.Job) chan AppDeploymentInfo {
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

//TODO - support Cronjob also
func checkJob(name string, app *vistectureCore.Application, k8sJobs map[string][]v1Batch.Job) AppDeploymentInfo {
	jobs, exists := k8sJobs[name]

	d := AppDeploymentInfo{
		Name:          name,
		VistectureApp: *app,
	}
	d.AppStateInfo.HealthCheckType = HealthCheckType_Job

	if !exists {
		d.AppStateInfo.State = State_unknown
		d.AppStateInfo.StateReason = "No job found"
		return d
	}

	var lastJob *v1Batch.Job
	for _, job := range jobs {
		if job.Status.CompletionTime == nil {
			continue
		}
		if lastJob == nil {
			lastJob = &job
		}
		if lastJob.Status.CompletionTime.Before(*job.Status.CompletionTime) {
			//take newer job
			lastJob = &job
		}
	}

	if lastJob == nil {
		d.AppStateInfo.State = State_unknown
		d.AppStateInfo.StateReason = "No completed job found"
		return d
	}

	if lastJob.Status.Failed > 0 {
		//one succeded job is ok
		d.AppStateInfo.State = State_unhealthy
		d.AppStateInfo.StateReason = "Last job failed: " + lastJob.Name
		return d
	}

	d.AppStateInfo.State = State_healthy
	return d
}

func checkDeploymentWithHealthCheck(name string, app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sServices map[string]v1.Service, k8sIngresses map[string][]K8sIngressInfo) AppDeploymentInfo {

	// Replace Name by configured Kubernetes Name
	if n, ok := app.Properties["k8sDeploymentName"]; ok && n != "" {
		name = n
	}

	depl, exists := k8sDeployments[name]

	d := AppDeploymentInfo{
		Name:          name,
		VistectureApp: *app,
	}

	if !exists {
		d.AppStateInfo.State = State_unknown
		d.AppStateInfo.StateReason = "No deployment found"

		return d
	}

	//add ingresses found for kubernetes Name
	d.Ingress = k8sIngresses[name]

	d.K8sDeployment = depl

	for _, c := range depl.Spec.Template.Spec.Containers {
		d.Images = append(d.Images, buildImageStruct(c.Image))
	}

	if !activeDeploymentExists(depl) {
		d.AppStateInfo.State = State_failed
		d.AppStateInfo.StateReason = "No active deployment"
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
		d.AppStateInfo.State = State_failed
		d.AppStateInfo.StateReason = "Deployment has no service for healthcheck that matches the config / " + k8sHealthCheckServiceName
		return d
	}
	if len(service.Spec.Ports) < 1 {
		d.AppStateInfo.State = State_failed
		d.AppStateInfo.StateReason = "Service has no port.. cannot check " + k8sHealthCheckServiceName
		return d
	}

	if h, ok := app.Properties["healthCheckPath"]; ok {
		d.HealthcheckPath = h
	}

	//Add a link to apiDocPath if possible:
	if len(k8sIngresses[name]) > 0 {
		if apiDocPath, ok := app.Properties["apiDocPath"]; ok {
			d.ApiDocumentationUrl = fmt.Sprintf("https://%v/%v", k8sIngresses[name][0].Host, apiDocPath)
		}
	}

	domain := fmt.Sprintf("%v:%v", k8sHealthCheckServiceName, service.Spec.Ports[0].Port)
	healthStatusOfService, reason, healthcheckType := checkHealth("http://"+domain, app.Properties["healthCheckPath"])
	d.AppStateInfo.HealthCheckType = healthcheckType
	if !healthStatusOfService {
		d.AppStateInfo.State = State_unhealthy
		d.AppStateInfo.StateReason = "Service Unhealthy: " + reason
		return d
	}

	//Try to do the healtcheck also from (public) ingress - if an ingress exist
	if len(k8sIngresses[k8sHealthCheckServiceName]) > 0 && d.AppStateInfo.State == State_healthy {
		d.AppStateInfo.HealthyAlsoFromIngress = checkPublicHealth(k8sIngresses[k8sHealthCheckServiceName], app.Properties["healthCheckPath"])
	}

	//In case the application need to be checked from outside - let it fail
	if _, ok := app.Properties["k8sHealthCheckThroughIngress"]; ok {
		if !d.AppStateInfo.HealthyAlsoFromIngress {
			if len(k8sIngresses[k8sHealthCheckServiceName]) == 0 {
				d.AppStateInfo.State = State_failed
				d.AppStateInfo.StateReason = "No Ingress for service " + k8sHealthCheckServiceName
			} else {
				d.AppStateInfo.State = State_unhealthy
				d.AppStateInfo.StateReason = fmt.Sprintf("Calling healthcheckPath %v from public ingress failed", app.Properties["healthCheckPath"])
			}
			return d
		}
	}

	d.AppStateInfo.State = State_healthy
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
		ok, _, _ := checkHealth("https://"+ing.Host, healtcheckPath)
		if ok {
			return true
		}
	}
	return false
}

func checkHealth(checkBaseUrl string, healtcheckPath string) (bool, string, string) {
	checkUrl := checkBaseUrl + healtcheckPath
	r, httpErr := http.Get(checkUrl)

	if httpErr != nil {
		return false, httpErr.Error(), HealthCheckType_NotCheckedYet
	}

	statusCode := r.StatusCode

	if healtcheckPath != "" {
		//Parse healthcheck

		jsonMap := &HealthCheckResponse{
			Services: []HealthCheckService{},
		}

		responseBody, bodyErr := ioutil.ReadAll(r.Body)
		if bodyErr != nil {
			return false, "Could not read from HealthcheckPath", HealthCheckType_HealthCheck
		}
		jsonError := json.Unmarshal(responseBody, jsonMap)
		// Check if Response is valid
		if jsonError != nil {
			return false, fmt.Sprintf("HealthcheckPath Format Error from %s", checkUrl), HealthCheckType_HealthCheck
		}

		if statusCode != 200 {
			statusText := fmt.Sprintf("Status  %v for %v", statusCode, checkUrl)
			for _, service := range jsonMap.Services {
				if !service.Alive {
					statusText = statusText + fmt.Sprintf("%v (%v) \n", service.Name, service.Details)
				}
			}
			return false, statusText, HealthCheckType_HealthCheck
		}
		return true, "", HealthCheckType_HealthCheck
	}

	//Fallback if no healthcheck is configured

	if statusCode > 500 {
		return false, fmt.Sprintf("Fallbackcheck returns error status %v ", statusCode), HealthCheckType_SimpleCheck
	}

	return true, "", HealthCheckType_SimpleCheck

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
