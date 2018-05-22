package kube

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	v1Batch "k8s.io/client-go/pkg/apis/batch/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type (
	DemoService struct{}
)

func (d *DemoService) GetKubernetesDeployments() (map[string]apps.Deployment, error) {
	deployments := make(map[string]apps.Deployment)
	deployments["flamingo"] = apps.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name: "flamingo",
		},
		Spec: apps.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Image: "flamingo:v1.0.0"},
					},
				},
			},
		},
		Status: apps.DeploymentStatus{
			AvailableReplicas:  3,
			Replicas:           5,
			ObservedGeneration: 132,
			Conditions: []apps.DeploymentCondition{
				{Status: v1.ConditionTrue, Type: "TestCondition", Message: "Test Condition is feeling good!"},
			},
		},
	}
	deployments["akeneo"] = apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "akeneo",
		},
		Spec: apps.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Image: "akeneo:v1.2.3"},
					},
				},
			},
		},
		Status: apps.DeploymentStatus{
			AvailableReplicas:  1,
			Replicas:           1,
			ObservedGeneration: 32,
			Conditions: []apps.DeploymentCondition{
				{Status: v1.ConditionTrue, Type: apps.DeploymentAvailable, Message: "Test Condition is feeling good!"},
			},
		},
	}
	deployments["keycloak"] = apps.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name: "keycloak",
		},
		Spec: apps.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{Image: "keycloak:v1.0.0"},
						{Image: "keycloak-support:v1.0.0"},
					},
				},
			},
		},
		Status: apps.DeploymentStatus{
			AvailableReplicas:  2,
			Replicas:           2,
			ObservedGeneration: 12,
			Conditions: []apps.DeploymentCondition{
				{Status: v1.ConditionTrue, Type: apps.DeploymentAvailable, Message: "Test Condition is feeling good!"},
			},
		},
	}
	return deployments, nil
}

func (d *DemoService) GetIngressesByService() (map[string][]K8sIngressInfo, error) {

	ingressList := &extensions.IngressList{
		Items: []extensions.Ingress{
			{
				Spec: extensions.IngressSpec{
					Rules: []extensions.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: extensions.IngressRuleValue{
								HTTP: &extensions.HTTPIngressRuleValue{
									Paths: []extensions.HTTPIngressPath{
										{Backend: extensions.IngressBackend{ServiceName: "flamingo"}, Path: "/"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: extensions.IngressSpec{
					Rules: []extensions.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: extensions.IngressRuleValue{
								HTTP: &extensions.HTTPIngressRuleValue{
									Paths: []extensions.HTTPIngressPath{
										{Backend: extensions.IngressBackend{ServiceName: "akeneo"}, Path: "/akeneo"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: extensions.IngressSpec{
					Rules: []extensions.IngressRule{
						{
							Host: "keycloak.bla",
							IngressRuleValue: extensions.IngressRuleValue{
								HTTP: &extensions.HTTPIngressRuleValue{
									Paths: []extensions.HTTPIngressPath{
										{Backend: extensions.IngressBackend{ServiceName: "keycloak"}, Path: "/blabla"},
									},
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "keycloak",
				},
				Spec: extensions.IngressSpec{
					Rules: []extensions.IngressRule{
						{
							Host: "keycloak.om3",
							IngressRuleValue: extensions.IngressRuleValue{
								HTTP: &extensions.HTTPIngressRuleValue{
									Paths: []extensions.HTTPIngressPath{
										{Backend: extensions.IngressBackend{ServiceName: "keycloak"}, Path: "/"},
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

func (d *DemoService) GetServices() (map[string]v1.Service, error) {
	services := make(map[string]v1.Service)
	services["keycloak"] = v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keycloak",
		},
	}
	services["flamingo"] = v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flamingo",
		},
	}

	services["akeneo"] = v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "akeneo",
		},
	}

	return services, nil
}

func (k *DemoService) GetJobs() (map[string]v1Batch.Job, error) {

	return nil, nil
}
