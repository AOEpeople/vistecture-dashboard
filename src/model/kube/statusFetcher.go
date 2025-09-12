package kube

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	vistectureCore "github.com/AOEpeople/vistecture/v2/model/core"
	"github.com/prometheus/client_golang/prometheus"
	apps "k8s.io/api/apps/v1"
	v1Batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

type (
	StatusFetcher struct {
		mu                    *sync.RWMutex
		apps                  map[string]AppDeploymentInfo
		definedVistectureApps []*vistectureCore.Application
		KubeInfoService       KubeInfoServiceInterface
	}

	// AppDeploymentInfo wraps Info on any Deployment's Data
	AppDeploymentInfo struct {
		Name                string
		Labels              map[string]string
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
		Path  string
		Alive bool
	}

	// HealthCheckResponse wraps a list of Services
	HealthCheckResponse struct {
		Services []HealthCheckService `json:"services"`
	}

	// HealthCheckService describes a Service the Application is dependent of, its Liveness and Details on its Status
	HealthCheckService struct {
		Name    string `json:"name"`
		Alive   bool   `json:"alive"`
		Details string `json:"details"`
	}
)

const healthCheckUserAgent = "VistectureDashboard"

var (
	healthcheck = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "application_health_status",
		Help: "Application Healthcheck  Status",
	}, []string{
		"application",
		"team",
	})

	healthcheckDependencies = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "application_health_dependency",
		Help: "Application Healthcheck Dependency",
	}, []string{
		"application",
		"dependency",
		"team",
	})

	httpClient = &http.Client{}
)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(healthcheck)
	prometheus.MustRegister(healthcheckDependencies)

	httpClient.Timeout = 15 * time.Second
}

const (
	State_unknown = iota
	State_failed
	State_unhealthy
	State_healthy
	State_unstable
	State_ignored
)

const (
	// This is the Interval for goroutine polling of kubernetes
	refreshInterval = 15

	HealthCheckType_NotCheckedYet = ""
	HealthCheckType_SimpleCheck   = "simple"
	HealthCheckType_HealthCheck   = "healthcheck"
	HealthCheckType_Job           = "job"
)

func NewStatusFetcher(apps []*vistectureCore.Application, demoMode bool, fakeHealthcheckPort int32) *StatusFetcher {
	statusManager := new(StatusFetcher)
	statusManager.mu = new(sync.RWMutex)
	statusManager.apps = make(map[string]AppDeploymentInfo)
	statusManager.definedVistectureApps = apps
	if demoMode {
		statusManager.KubeInfoService = &DemoService{fakeHealthcheckPort: fakeHealthcheckPort}
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

// FetchStatusInRegularInterval controls the interval in which new info is fetched and loops over configured applications
func (stm *StatusFetcher) FetchStatusInRegularInterval(ignoredServices []string) {
	var tickIteration = 0
	lastResults := make(map[string][]AppDeploymentInfo)
	fetcher := func() {
		// Add Deployments to Dashboard
		k8sDeployments, err := stm.KubeInfoService.GetKubernetesDeployments()

		if err != nil {
			panic("Could not get Deployment Config, check Configuration and Kubernetes Connection: " + err.Error())
		}

		configMaps, err := stm.KubeInfoService.GetConfigMaps()
		if err != nil {
			panic("Could not get Config Maps, check Configuration and Kubernetes Connection: " + err.Error())
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

		tickIteration++

		// results is a list of channels, which get filled by the fetcher
		var results []chan AppDeploymentInfo

		for _, app := range stm.definedVistectureApps {
			// Deployment is not on Kubernetes
			if di, ok := app.Properties["deployment"]; !ok || di != "kubernetes" {
				continue
			}
			// wait a bit between healthchecks to not do them all at once
			millisecondsToWait := rand.Intn(700) + 300
			time.Sleep(time.Millisecond * time.Duration(millisecondsToWait))

			results = append(results, checkAppStatusInKubernetes(ignoredServices, app, k8sDeployments, services, ingresses, jobs, configMaps))
		}

		// exclusive lock map for write access
		stm.mu.Lock()

		// read all results in to map
		for _, result := range results {
			// get result from future
			status := <-result

			// prepend status to list of last results
			lastResults[status.Name] = append([]AppDeploymentInfo{status}, lastResults[status.Name]...)
			if len(lastResults[status.Name]) > 20 {
				// limit to 20
				lastResults[status.Name] = lastResults[status.Name][:20]
			}

			countRecentUnstable := 0
			var recentIssues []string
			// mark as unstable if in last was a failure
			if status.AppStateInfo.State == State_healthy {
				for _, lastStatus := range lastResults[status.Name] {
					if lastStatus.AppStateInfo.State == State_failed || lastStatus.AppStateInfo.State == State_unhealthy {
						countRecentUnstable++
						recentIssues = append(recentIssues, lastStatus.AppStateInfo.StateReason)
					}
				}
			}

			if countRecentUnstable > 0 {
				status.AppStateInfo.State = State_unstable
				status.AppStateInfo.StateReason = fmt.Sprintf(
					"Failed %d out of %d checks in the last %d seconds\n%s",
					countRecentUnstable,
					len(lastResults[status.Name]),
					len(lastResults[status.Name])*refreshInterval,
					strings.Join(recentIssues, "\n"),
				)
			}

			stm.apps[status.Name] = status
			switch status.AppStateInfo.State {
			case State_healthy, State_ignored:
				healthcheck.With(prometheus.Labels{"application": status.Name, "team": status.VistectureApp.Team}).Set(0)
			case State_unhealthy, State_unstable:
				healthcheck.With(prometheus.Labels{"application": status.Name, "team": status.VistectureApp.Team}).Set(2)
			case State_failed:
				healthcheck.With(prometheus.Labels{"application": status.Name, "team": status.VistectureApp.Team}).Set(3)
			case State_unknown:
				healthcheck.With(prometheus.Labels{"application": status.Name, "team": status.VistectureApp.Team}).Set(1)
			}

		}

		// unlock map
		stm.mu.Unlock()
	}

	fetcher()
	for range time.Tick(refreshInterval * time.Second) {
		fetcher()
	}
}

// checkAppStatusInKubernetes iterates through k8sDeployments and controls the result channel
func checkAppStatusInKubernetes(ignoredServices []string, app *vistectureCore.Application, k8sDeployments map[string]apps.Deployment, k8sServices map[string]v1.Service, k8sIngresses map[string][]K8sIngressInfo, k8sJobs map[string][]v1Batch.Job, k8sConfigMaps map[string]v1.ConfigMap) chan AppDeploymentInfo {
	// result (like a futures)
	res := make(chan AppDeploymentInfo, 1)

	// start fetcher routing
	go func(res chan<- AppDeploymentInfo) {
		name := app.Name
		config := k8sConfigMaps[name]
		if n, ok := config.Data["k8sDeploymentName"]; ok {
			app.Properties["k8sDeploymentName"] = n
		}

		var info AppDeploymentInfo
		// Replace Name by configured Kubernetes Name
		if n, ok := app.Properties["k8sType"]; ok && n == "job" {
			info = checkJob(name, app, k8sJobs)
		} else {
			info = checkDeploymentWithHealthCheck(name, app, k8sDeployments, k8sServices, k8sIngresses)
		}

		if slices.Contains(ignoredServices, name) {
			info.AppStateInfo.State = State_ignored
			info.AppStateInfo.StateReason = "Ignored by setting override"
		}

		res <- info
	}(res)

	return res
}

// TODO - support Cronjob also
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

		if lastJob.Status.CompletionTime.Before(job.Status.CompletionTime) {
			// take newer job
			lastJob = &job
		}
	}

	if lastJob == nil {
		d.AppStateInfo.State = State_unknown
		d.AppStateInfo.StateReason = "No completed job found"
		return d
	}

	if lastJob.Status.Succeeded == 0 && lastJob.Status.Failed > 0 {
		// one succeeded job is ok
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

	// add ingresses found for kubernetes Name
	d.Ingress = k8sIngresses[name]

	d.K8sDeployment = depl

	for _, c := range depl.Spec.Template.Spec.Containers {
		d.Images = append(d.Images, buildImageStruct(c.Image))
	}

	d.Labels = make(map[string]string)
	for k, e := range depl.Labels {
		if k == "helm.sh/version" {
			d.Labels["helm"] = e
		}
		d.Labels[k] = e
	}

	if !podExists(depl) {
		d.AppStateInfo.State = State_failed
		d.AppStateInfo.StateReason = "No pod available"
		return d
	}

	// Now check the service
	k8sHealthCheckServiceName := name
	if h, ok := app.Properties["k8sHealthCheckServiceName"]; ok {
		k8sHealthCheckServiceName = h
		// Add ingresses that might exists for seperate k8sHealthCheckServiceName
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
		d.AppStateInfo.StateReason = "Service has no port. Cannot check " + k8sHealthCheckServiceName
		return d
	}

	if h, ok := app.Properties["healthCheckPath"]; ok {
		d.HealthcheckPath = h
	}

	// Add a link to apiDocPath if possible:
	if len(k8sIngresses[name]) > 0 {
		if apiDocPath, ok := app.Properties["apiDocPath"]; ok {
			d.ApiDocumentationUrl = fmt.Sprintf("https://%v/%v", k8sIngresses[name][0].Host, apiDocPath)
		}
	}

	foundMetricsPort := findMetricsPort(app, service)

	domain := fmt.Sprintf("%s:%d", k8sHealthCheckServiceName, foundMetricsPort)
	healthStatusOfService, reason, healthcheckType := checkHealth(d, "http://"+domain, app.Properties["healthCheckPath"])
	d.AppStateInfo.HealthCheckType = healthcheckType

	if !healthStatusOfService {
		d.AppStateInfo.State = State_unhealthy
		d.AppStateInfo.StateReason = "Service Unhealthy: " + reason
		return d
	}

	// In case the application need to be checked from outside, do the check and let it fail if unhealthy/misconfigured
	if _, ok := app.Properties["k8sHealthCheckThroughIngress"]; ok {
		// Try to do the healthcheck from ingress
		if len(k8sIngresses[k8sHealthCheckServiceName]) > 0 {
			d.AppStateInfo.HealthyAlsoFromIngress = checkPublicHealth(k8sIngresses[k8sHealthCheckServiceName], app.Properties["healthCheckPath"])
		}

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

func podExists(deployment apps.Deployment) bool {
	return deployment.Status.AvailableReplicas != 0
}

// checkPublicHealth calls the healthcheck via public ingress
func checkPublicHealth(ingresses []K8sIngressInfo, healtcheckPath string) bool {
	var reason string
	var checktype string
	var ok bool
	for _, ing := range ingresses {
		// At least one ingress should succeed
		ok, reason, checktype = checkHealth(AppDeploymentInfo{}, "https://"+ing.Host, healtcheckPath)
		if ok {
			return true
		}
	}
	log.Printf("checkPublicHealth failed Reason:%v / Via:%v", reason, checktype)
	return false
}

func checkHealth(status AppDeploymentInfo, checkBaseUrl string, healtcheckPath string) (bool, string, string) {
	checkUrl := checkBaseUrl + healtcheckPath

	req, reqErr := http.NewRequest("GET", checkUrl, nil)
	if reqErr != nil {
		return false, reqErr.Error(), HealthCheckType_NotCheckedYet
	}

	req.Header.Set("User-Agent", healthCheckUserAgent)
	r, httpErr := httpClient.Do(req)

	if httpErr != nil {
		return false, httpErr.Error(), HealthCheckType_NotCheckedYet
	}

	statusCode := r.StatusCode

	if healtcheckPath != "" {
		// Parse healthcheck
		jsonMap := &HealthCheckResponse{
			Services: []HealthCheckService{},
		}

		responseBody, bodyErr := io.ReadAll(r.Body)
		if bodyErr != nil {
			return false, "Could not read from HealthcheckPath", HealthCheckType_HealthCheck
		}
		jsonError := json.Unmarshal(responseBody, jsonMap)
		// Check if Response is valid
		if jsonError != nil {
			return false, fmt.Sprintf("HealthcheckPath Format Error from %s", checkUrl), HealthCheckType_HealthCheck
		}

		statusText := fmt.Sprintf("Status %v for %v ", statusCode, checkUrl)
		finalStatus := true

		for _, service := range jsonMap.Services {
			s := float64(0)
			if !service.Alive {
				statusText += fmt.Sprintf("%v (%v) \n", service.Name, service.Details)
				s = 1
				finalStatus = false
			}

			if status.Name != "" {
				healthcheckDependencies.With(prometheus.Labels{"application": status.Name, "dependency": service.Name, "team": status.VistectureApp.Team}).Set(s)
			}
		}

		return finalStatus, statusText, HealthCheckType_HealthCheck
	}

	// Fallback if no healthcheck is configured
	if statusCode > 500 {
		return false, fmt.Sprintf("Fallbackcheck returns error status %v ", statusCode), HealthCheckType_SimpleCheck
	}

	return true, "", HealthCheckType_SimpleCheck

}

func findMetricsPort(app *vistectureCore.Application, service v1.Service) int32 {
	if port, found := app.Properties["healthCheckPort"]; found {
		intPort, err := strconv.ParseInt(port, 10, 32)
		if err == nil {
			return int32(intPort)
		}
	}

	if portName, found := app.Properties["healthCheckPortName"]; found {
		for _, port := range service.Spec.Ports {
			if port.Name == portName {
				return port.Port
			}
		}
	}

	return service.Spec.Ports[0].Port
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
