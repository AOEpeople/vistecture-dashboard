package kube

import (
	"log"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	v1Batch "k8s.io/client-go/pkg/apis/batch/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	// kubeClient is a Kubernetes Client Object
	kubeClient struct {
		Namespace  string
		Clientset  *kubernetes.Clientset
		kubeconfig clientcmd.ClientConfig
		restconfig *rest.Config
	}

	KubeInfoServiceInterface interface {
		GetKubernetesDeployments() (map[string]apps.Deployment, error)
		GetIngressesByService() (map[string][]K8sIngressInfo, error)
		GetServices() (map[string]v1.Service, error)
		GetJobsByApp() (map[string][]v1Batch.Job, error)
	}

	KubeInfoService struct {
		DemoMode bool
	}
)

// kubeClientFromConfig loads a new kubeClient from the usual configuration
// (KUBECONFIG env param / selfconfigured in kubernetes)
func KubeClientFromConfig() (*kubeClient, error) {
	var client = new(kubeClient)
	var err error

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	configOverrides := &clientcmd.ConfigOverrides{}

	client.kubeconfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	client.restconfig, err = client.kubeconfig.ClientConfig()

	if err != nil {
		return nil, err
	}

	client.Clientset, err = kubernetes.NewForConfig(client.restconfig)
	if err != nil {
		return nil, err
	}

	client.Namespace, _, err = client.kubeconfig.Namespace()
	if err != nil {
		return nil, err
	}

	return client, nil
}

// getKubernetesDeployments fetches from Config or Demo Data
func (k *KubeInfoService) GetKubernetesDeployments() (map[string]apps.Deployment, error) {
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
	log.Printf("K8s: found %v deployments..\n", len(deployments.Items))

	for _, deployment := range deployments.Items {
		deploymentIndex[deployment.Name] = deployment
	}

	return deploymentIndex, nil
}

// getIngressesByService fetches from Config or Demo Data
func (k *KubeInfoService) GetIngressesByService() (map[string][]K8sIngressInfo, error) {
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
	log.Printf("K8s: found %v ingresses..\n", len(ingresses.Items))

	return groupByServiceName(ingresses), nil
}

func groupByServiceName(ingresses *extensions.IngressList) map[string][]K8sIngressInfo {
	ingressIndex := make(map[string][]K8sIngressInfo)
	for _, ing := range ingresses.Items {
		for _, rule := range ing.Spec.Rules {
			for _, p := range rule.HTTP.Paths {
				name := p.Backend.ServiceName
				ingressIndex[name] = append(ingressIndex[name], K8sIngressInfo{URL: rule.Host + p.Path, Host: rule.Host})
			}
		}
	}
	return ingressIndex
}

func (k *KubeInfoService) GetServices() (map[string]v1.Service, error) {

	client, err := KubeClientFromConfig()

	if err != nil {
		return nil, err
	}

	serviceClient := client.Clientset.CoreV1().Services(client.Namespace)
	services, err := serviceClient.List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	serviceIndex := make(map[string]v1.Service)
	log.Printf("K8s: found %v Services..\n", len(services.Items))

	for _, service := range services.Items {
		serviceIndex[service.Name] = service

	}
	return serviceIndex, nil
}

func (k *KubeInfoService) GetJobsByApp() (map[string][]v1Batch.Job, error) {

	client, err := KubeClientFromConfig()

	if err != nil {
		return nil, err
	}

	jobsClient := client.Clientset.BatchV1().Jobs(client.Namespace)
	jobs, err := jobsClient.List(metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	jobsIndex := make(map[string][]v1Batch.Job)

	log.Printf("K8s: found %v Jobs..\n", len(jobs.Items))
	for _, job := range jobs.Items {
		//Match the jobname to appname (by deleting the last generated number for cronjobs - e.g. "akeneo-12345"  is the last created job for "akeneo")
		applicationname := job.Name
		reg := regexp.MustCompile("(.*)-([0-9]+)")
		submatches := reg.FindStringSubmatch(applicationname)
		if len(submatches) == 3 {
			//fmt.Printf("%q\n", submatches)
			//log.Printf("submatch %v for %v", submatches[1], applicationname)
			applicationname = submatches[1]
		}
		jobsIndex[applicationname] = append(jobsIndex[applicationname], job)
	}
	return jobsIndex, nil
}
