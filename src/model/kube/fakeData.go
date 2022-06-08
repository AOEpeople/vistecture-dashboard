package kube

import (
	appsV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	networkingV1 "k8s.io/api/networking/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	// DemoService fake implemnation used for testing
	DemoService struct{}
)

var _ KubeInfoServiceInterface = &DemoService{}

// GetKubernetesDeployments returns fake deployments
func (d *DemoService) GetKubernetesDeployments() (map[string]appsV1.Deployment, error) {

	deployments := make(map[string]appsV1.Deployment)
	deployments["flamingo"] = appsV1.Deployment{

		ObjectMeta: metaV1.ObjectMeta{
			Name: "flamingo",
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
	}
	deployments["akeneo"] = appsV1.Deployment{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "akeneo",
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
	}
	deployments["keycloak"] = appsV1.Deployment{

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
							Host: "keycloak.om3",
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
	services := make(map[string]v1.Service)
	services["keycloak"] = v1.Service{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "keycloak",
		},
	}
	services["flamingo"] = v1.Service{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "flamingo",
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Port: 80},
			},
		},
	}

	services["akeneo"] = v1.Service{
		ObjectMeta: metaV1.ObjectMeta{
			Name: "akeneo",
		},
	}

	return services, nil
}

// GetJobsByApp returns jobs matching app names
func (k *DemoService) GetJobsByApp() (map[string][]batchV1.Job, error) {
	return nil, nil
}
