package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	apps "k8s.io/client-go/pkg/apis/apps/v1beta1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type (
	kubeClient struct {
		namespace  string
		clientset  *kubernetes.Clientset
		kubeconfig clientcmd.ClientConfig
		restconfig *rest.Config
	}
)

// KubeClientFromConfig loads a new kubeClient from the usual configuration
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

	client.clientset, err = kubernetes.NewForConfig(client.restconfig)
	if err != nil {
		return nil, err
	}

	client.namespace, _, err = client.kubeconfig.Namespace()
	if err != nil {
		return nil, err
	}

	return client, nil
}

func demoDeployments() *apps.DeploymentList {
	return &apps.DeploymentList{
		Items: []apps.Deployment{
			{
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
			},
			{
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
						{Status: v1.ConditionTrue, Type: "TestCondition", Message: "Test Condition is feeling good!"},
					},
				},
			},
			{
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
			},
		},
	}
}

func demoIngresses() *extensions.IngressList {
	return &extensions.IngressList{
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
}
