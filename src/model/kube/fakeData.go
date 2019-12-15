package kube

import (
	apibatchv1 "k8s.io/api/batch/v1"
	apiextensionv1beta "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"

	//"k8s.io/client-go/pkg/api/v1"
	//apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	//v1Batch "k8s.io/client-go/pkg/apis/batch/v1"
	//extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

type (
	DemoService struct{}
)

func (d *DemoService) GetKubernetesDeployments() (map[string]appsv1.Deployment, error) {

	deployments := make(map[string]appsv1.Deployment)
	deployments["flamingo"] = appsv1.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name: "flamingo",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Image: "flamingo:v1.0.0"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas:  3,
			Replicas:           5,
			ObservedGeneration: 132,
			Conditions: []appsv1.DeploymentCondition{
				{Status: apiv1.ConditionTrue, Type: appsv1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
			},
		},
	}
	deployments["akeneo"] = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "akeneo",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Image: "akeneo:v1.2.3"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas:  1,
			Replicas:           1,
			ObservedGeneration: 32,
			Conditions: []appsv1.DeploymentCondition{
				{Status: apiv1.ConditionTrue, Type: appsv1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
			},
		},
	}
	deployments["keycloak"] = appsv1.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name: "keycloak",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Image: "keycloak:v1.0.0"},
						{Image: "keycloak-support:v1.0.0"},
					},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas:  2,
			Replicas:           2,
			ObservedGeneration: 12,
			Conditions: []appsv1.DeploymentCondition{
				{Status: apiv1.ConditionTrue, Type: appsv1.DeploymentAvailable, Message: "Test Condition is feeling good!"},
			},
		},
	}
	return deployments, nil
}

func (d *DemoService) GetIngressesByService() (map[string][]K8sIngressInfo, error) {

	ingressList := &apiextensionv1beta.IngressList{
		Items: []apiextensionv1beta.Ingress{
			{
				Spec: apiextensionv1beta.IngressSpec{
					Rules: []apiextensionv1beta.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: apiextensionv1beta.IngressRuleValue{
								HTTP: &apiextensionv1beta.HTTPIngressRuleValue{
									Paths: []apiextensionv1beta.HTTPIngressPath{
										{Backend: apiextensionv1beta.IngressBackend{ServiceName: "flamingo"}, Path: "/"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: apiextensionv1beta.IngressSpec{
					Rules: []apiextensionv1beta.IngressRule{
						{
							Host: "google.com",
							IngressRuleValue: apiextensionv1beta.IngressRuleValue{
								HTTP: &apiextensionv1beta.HTTPIngressRuleValue{
									Paths: []apiextensionv1beta.HTTPIngressPath{
										{Backend: apiextensionv1beta.IngressBackend{ServiceName: "akeneo"}, Path: "/akeneo"},
									},
								},
							},
						},
					},
				},
			},
			{
				Spec: apiextensionv1beta.IngressSpec{
					Rules: []apiextensionv1beta.IngressRule{
						{
							Host: "keycloak.bla",
							IngressRuleValue: apiextensionv1beta.IngressRuleValue{
								HTTP: &apiextensionv1beta.HTTPIngressRuleValue{
									Paths: []apiextensionv1beta.HTTPIngressPath{
										{Backend: apiextensionv1beta.IngressBackend{ServiceName: "keycloak"}, Path: "/blabla"},
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
				Spec: apiextensionv1beta.IngressSpec{
					Rules: []apiextensionv1beta.IngressRule{
						{
							Host: "keycloak.om3",
							IngressRuleValue: apiextensionv1beta.IngressRuleValue{
								HTTP: &apiextensionv1beta.HTTPIngressRuleValue{
									Paths: []apiextensionv1beta.HTTPIngressPath{
										{Backend: apiextensionv1beta.IngressBackend{ServiceName: "keycloak"}, Path: "/"},
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

func (d *DemoService) GetServices() (map[string]apiv1.Service, error) {
	services := make(map[string]apiv1.Service)
	services["keycloak"] = apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "keycloak",
		},
	}
	services["flamingo"] = apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "flamingo",
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{Port: 80},
			},
		},
	}

	services["akeneo"] = apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "akeneo",
		},
	}

	return services, nil
}

func (k *DemoService) GetJobsByApp() (map[string][]apibatchv1.Job, error) {
	return nil, nil
}
