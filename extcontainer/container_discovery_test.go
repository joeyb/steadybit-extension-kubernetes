// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package extcontainer

import (
	"context"
	kclient "github.com/steadybit/extension-kubernetes/client"
	"github.com/steadybit/extension-kubernetes/extconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"testing"
	"time"
)

func Test_getDiscoveredContainer(t *testing.T) {
	// Given
	stopCh := make(chan struct{})
	defer close(stopCh)
	client, clientset := getTestClient(stopCh)
	extconfig.Config.ClusterName = "development"
	extconfig.Config.LabelFilter = []string{"secret-label"}

	_, err := clientset.CoreV1().
		Services("default").
		Create(context.Background(), &v1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop-kevelaer",
				Namespace: "default",
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"best-city": "Kevelaer",
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().
		Services("default").
		Create(context.Background(), &v1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop-solingen",
				Namespace: "default",
			},
			Spec: v1.ServiceSpec{
				Selector: map[string]string{
					"best-city": "Solingen",
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().
		Pods("default").
		Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop",
				Namespace: "default",
				Labels: map[string]string{
					"best-city":    "Kevelaer",
					"secret-label": "secret-value",
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						ContainerID: "crio://abcdef",
						Name:        "MrFancyPants",
						Image:       "nginx",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "worker-1",
				Containers: []v1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: "Always",
					},
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	// When
	assert.Eventually(t, func() bool {
		return len(getDiscoveredContainerEnrichmentData(client)) == 1
	}, time.Second, 100*time.Millisecond)

	// Then
	targets := getDiscoveredContainerEnrichmentData(client)
	require.Len(t, targets, 1)
	target := targets[0]
	assert.Equal(t, "crio://abcdef", target.Id)
	assert.Equal(t, KubernetesContainerEnrichmentDataType, target.EnrichmentDataType)
	assert.Equal(t, map[string][]string{
		"k8s.cluster-name":          {"development"},
		"k8s.container.id":          {"crio://abcdef"},
		"k8s.container.id.stripped": {"abcdef"},
		"k8s.container.name":        {"MrFancyPants"},
		"k8s.container.ready":       {"false"},
		"k8s.container.image":       {"nginx"},
		"k8s.namespace":             {"default"},
		"k8s.node.name":             {"worker-1"},
		"k8s.pod.name":              {"shop"},
		"k8s.pod.label.best-city":   {"Kevelaer"},
		"k8s.label.best-city":       {"Kevelaer"},
		"k8s.service.name":          {"shop-kevelaer"},
		"k8s.distribution":          {"openshift"},
	}, target.Attributes)
}

func Test_getDiscoveredContainerShouldIgnoreLabeledPods(t *testing.T) {
	// Given
	stopCh := make(chan struct{})
	defer close(stopCh)
	client, clientset := getTestClient(stopCh)
	extconfig.Config.ClusterName = "development"

	_, err := clientset.CoreV1().
		Pods("default").
		Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop",
				Namespace: "default",
				Labels: map[string]string{
					"best-city": "Kevelaer",
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						ContainerID: "crio://abcdef",
						Name:        "MrFancyPants",
						Image:       "nginx",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "worker-1",
				Containers: []v1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: "Always",
					},
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().
		Pods("default").
		Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop-ignored",
				Namespace: "default",
				Labels: map[string]string{
					"best-city":                        "Kevelaer",
					"steadybit.com/discovery-disabled": "true",
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						ContainerID: "crio://abcdef",
						Name:        "MrFancyPants",
						Image:       "nginx",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "worker-1",
				Containers: []v1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: "Always",
					},
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	// When
	assert.Eventually(t, func() bool {
		return len(getDiscoveredContainerEnrichmentData(client)) == 1
	}, time.Second, 100*time.Millisecond)

	// Then
	targets := getDiscoveredContainerEnrichmentData(client)
	require.Len(t, targets, 1)
}

func Test_getDiscoveredContainerShouldNotIgnoreLabeledPodsIfExcludesDisabled(t *testing.T) {
	// Given
	stopCh := make(chan struct{})
	defer close(stopCh)
	client, clientset := getTestClient(stopCh)
	extconfig.Config.ClusterName = "development"
	extconfig.Config.DisableDiscoveryExcludes = true

	_, err := clientset.CoreV1().
		Pods("default").
		Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop",
				Namespace: "default",
				Labels: map[string]string{
					"best-city": "Kevelaer",
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						ContainerID: "crio://abcdef",
						Name:        "MrFancyPants",
						Image:       "nginx",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "worker-1",
				Containers: []v1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: "Always",
					},
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	_, err = clientset.CoreV1().
		Pods("default").
		Create(context.Background(), &v1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shop-ignored",
				Namespace: "default",
				Labels: map[string]string{
					"best-city":                        "Kevelaer",
					"steadybit.com/discovery-disabled": "true",
				},
			},
			Status: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						ContainerID: "crio://abcdef",
						Name:        "MrFancyPants",
						Image:       "nginx",
					},
				},
			},
			Spec: v1.PodSpec{
				NodeName: "worker-1",
				Containers: []v1.Container{
					{
						Name:            "nginx",
						Image:           "nginx",
						ImagePullPolicy: "Always",
					},
				},
			},
		}, metav1.CreateOptions{})
	require.NoError(t, err)

	// When
	assert.Eventually(t, func() bool {
		return len(getDiscoveredContainerEnrichmentData(client)) >= 1
	}, time.Second, 100*time.Millisecond)

	// Then
	targets := getDiscoveredContainerEnrichmentData(client)
	require.Len(t, targets, 2)
}

func getTestClient(stopCh <-chan struct{}) (*kclient.Client, kubernetes.Interface) {
	clientset := testclient.NewSimpleClientset()
	client := kclient.CreateClient(clientset, stopCh, "/oapi")
	return client, clientset
}
