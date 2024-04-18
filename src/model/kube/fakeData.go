package kube

import (
	appsV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type (
	// DemoService fake implementation used for testing
	DemoService struct {
		fakeHealthcheckPort int32
	}
)

var _ KubeInfoServiceInterface = &DemoService{}

// GetKubernetesDeployments returns fake deployments
func (d *DemoService) GetKubernetesDeployments() (map[string]appsV1.Deployment, error) {
	deployments := map[string]appsV1.Deployment{
		"service": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "service",
				Labels: map[string]string{
					"chart": "service-1.0.1",
				},
			},
			Spec: appsV1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Image: "service:v1.0.0"},
						},
					},
				},
			},
			Status: appsV1.DeploymentStatus{
				AvailableReplicas:  2,
				Replicas:           2,
				ObservedGeneration: 12,
				Conditions: []appsV1.DeploymentCondition{
					{Status: v1.ConditionTrue, Type: appsV1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
				},
			},
		},
		"flamingo": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "flamingo",
				Labels: map[string]string{
					"chart": "flamingo-1.0.1",
				},
			},
			Spec: appsV1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Image: "flamingo:v1.0.0"},
						},
					},
				},
			},
			Status: appsV1.DeploymentStatus{
				AvailableReplicas:  3,
				Replicas:           5,
				ObservedGeneration: 132,
				Conditions: []appsV1.DeploymentCondition{
					{Status: v1.ConditionTrue, Type: appsV1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
				},
			},
		},
		"akeneo": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "akeneo",
				Labels: map[string]string{
					"chart":           "akeneo-1.2.3",
					"helm.sh/version": "1.2.3",
				},
			},
			Spec: appsV1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Image: "akeneo:v1.2.3"},
						},
					},
				},
			},
			Status: appsV1.DeploymentStatus{
				AvailableReplicas:  1,
				Replicas:           1,
				ObservedGeneration: 32,
				Conditions: []appsV1.DeploymentCondition{
					{Status: v1.ConditionTrue, Type: appsV1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
				},
			},
		},
		"keycloak": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "keycloak",
			},
			Spec: appsV1.DeploymentSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{Image: "keycloak:v1.0.0"},
							{Image: "keycloak-support:v1.0.0"},
						},
					},
				},
			},
			Status: appsV1.DeploymentStatus{
				AvailableReplicas:  2,
				Replicas:           2,
				ObservedGeneration: 12,
				Conditions: []appsV1.DeploymentCondition{
					{Status: v1.ConditionTrue, Type: appsV1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
				},
			},
		},
	}

	return deployments, nil
}

// GetIngressesByService returns fake services
func (d *DemoService) GetIngressesByService() (map[string][]K8sIngressInfo, error) {
	ingressList := &networkingV1.IngressList{
		Items: []networkingV1.Ingress{
			{
				Spec: networkingV1.IngressSpec{
					Rules: []networkingV1.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: networkingV1.IngressRuleValue{
								HTTP: &networkingV1.HTTPIngressRuleValue{
									Paths: []networkingV1.HTTPIngressPath{
										{Backend: networkingV1.IngressBackend{Service: &networkingV1.IngressServiceBackend{Name: "flamingo"}}, Path: "/"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: networkingV1.IngressSpec{
					Rules: []networkingV1.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: networkingV1.IngressRuleValue{
								HTTP: &networkingV1.HTTPIngressRuleValue{
									Paths: []networkingV1.HTTPIngressPath{
										{Backend: networkingV1.IngressBackend{Service: &networkingV1.IngressServiceBackend{Name: "akeneo"}}, Path: "/akeneo"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: networkingV1.IngressSpec{
					Rules: []networkingV1.IngressRule{
						{
							Host: "keycloak.bla",
							IngressRuleValue: networkingV1.IngressRuleValue{
								HTTP: &networkingV1.HTTPIngressRuleValue{
									Paths: []networkingV1.HTTPIngressPath{
										{Backend: networkingV1.IngressBackend{Service: &networkingV1.IngressServiceBackend{Name: "keycloak"}}, Path: "/blabla"},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metaV1.ObjectMeta{
					Name: "keycloak",
				},
				Spec: networkingV1.IngressSpec{
					Rules: []networkingV1.IngressRule{
						{
							Host: "keycloak.aoe",
							IngressRuleValue: networkingV1.IngressRuleValue{
								HTTP: &networkingV1.HTTPIngressRuleValue{
									Paths: []networkingV1.HTTPIngressPath{
										{Backend: networkingV1.IngressBackend{Service: &networkingV1.IngressServiceBackend{Name: "keycloak"}}, Path: "/"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return groupByServiceName(ingressList), nil
}

// GetServices returns fake services
func (d *DemoService) GetServices() (map[string]v1.Service, error) {
	services := map[string]v1.Service{
		"localhost": {
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{Port: d.fakeHealthcheckPort, TargetPort: intstr.FromInt(8080)},
				},
			},
			ObjectMeta: metaV1.ObjectMeta{
				Name: "localhost",
			},
		},
		"keycloak": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "keycloak",
			},
		},
		"flamingo": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "flamingo",
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{Port: 80},
				},
			},
		},
		"akeneo": {
			ObjectMeta: metaV1.ObjectMeta{
				Name: "akeneo",
			},
		},
	}

	return services, nil
}

// GetConfigMaps returns fake config maps
func (d *DemoService) GetConfigMaps() (map[string]v1.ConfigMap, error) {
	return nil, nil
}

// GetJobsByApp returns jobs matching app names
func (d *DemoService) GetJobsByApp() (map[string][]batchV1.Job, error) {
	return nil, nil
}
