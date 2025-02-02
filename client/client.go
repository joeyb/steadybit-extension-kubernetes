// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package client

import (
	"errors"
	"flag"
	"fmt"
	"github.com/rs/zerolog/log"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerAppsv1 "k8s.io/client-go/listers/apps/v1"
	listerCorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var K8S *Client

type Client struct {
	Distribution         string
	daemonSetsLister     listerAppsv1.DaemonSetLister
	daemonSetsInformer   cache.SharedIndexInformer
	deploymentsLister    listerAppsv1.DeploymentLister
	deploymentsInformer  cache.SharedIndexInformer
	podsLister           listerCorev1.PodLister
	podsInformer         cache.SharedIndexInformer
	replicaSetsLister    listerAppsv1.ReplicaSetLister
	replicaSetsInformer  cache.SharedIndexInformer
	servicesLister       listerCorev1.ServiceLister
	servicesInformer     cache.SharedIndexInformer
	statefulSetsLister   listerAppsv1.StatefulSetLister
	statefulSetsInformer cache.SharedIndexInformer
	eventsInformer       cache.SharedIndexInformer
	nodesLister          listerCorev1.NodeLister
	nodesInformer        cache.SharedIndexInformer
}

func (c *Client) Pods() []*corev1.Pod {
	pods, err := c.podsLister.List(labels.Everything())
	if err != nil {
		log.Error().Err(err).Msgf("Error while fetching pods")
		return []*corev1.Pod{}
	}
	return pods
}

func (c *Client) PodsByDeployment(deployment *appsv1.Deployment) []*corev1.Pod {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		log.Error().Err(err).Msgf("Error while creating a selector from deployment %s/%s - selector %s", deployment.Name, deployment.Namespace, deployment.Spec.Selector)
		return nil
	}
	list, err := c.podsLister.Pods(deployment.Namespace).List(selector)
	if err != nil {
		log.Error().Err(err).Msgf("Error while fetching Pods for Deployment %s/%s - selector %s", deployment.Name, deployment.Namespace, selector)
		return nil
	}
	return list
}

func (c *Client) Deployments() []*appsv1.Deployment {
	deployments, err := c.deploymentsLister.List(labels.Everything())
	if err != nil {
		log.Error().Err(err).Msgf("Error while fetching deployments")
		return []*appsv1.Deployment{}
	}
	return deployments
}

func (c *Client) ServicesByPod(pod *corev1.Pod) []*corev1.Service {
	services, err := c.servicesLister.List(labels.Everything())
	if err != nil {
		log.Error().Err(err).Msgf("Error while fetching services")
		return []*corev1.Service{}
	}
	var result []*corev1.Service
	for _, service := range services {
		match := service.Spec.Selector != nil
		for key, value := range service.Spec.Selector {
			if value != pod.ObjectMeta.Labels[key] {
				match = false
			}
		}
		if match {
			result = append(result, service)
		}
	}
	return result
}

func (c *Client) DaemonSetByNamespaceAndName(namespace string, name string) *appsv1.DaemonSet {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, _, err := c.daemonSetsInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Error().Err(err).Msgf("Error during lookup of DaemonSet %s/%s", namespace, name)
	}
	if item != nil {
		return item.(*appsv1.DaemonSet)
	} else {
		return nil
	}
}
func (c *Client) DeploymentByNamespaceAndName(namespace string, name string) *appsv1.Deployment {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, _, err := c.deploymentsInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Error().Err(err).Msgf("Error during lookup of Deployment %s/%s", namespace, name)
	}
	if item != nil {
		return item.(*appsv1.Deployment)
	} else {
		return nil
	}
}
func (c *Client) ReplicaSetByNamespaceAndName(namespace string, name string) *appsv1.ReplicaSet {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, _, err := c.replicaSetsInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Error().Err(err).Msgf("Error during lookup of ReplicaSet %s/%s", namespace, name)
	}
	if item != nil {
		return item.(*appsv1.ReplicaSet)
	} else {
		return nil
	}
}
func (c *Client) StatefulSetByNamespaceAndName(namespace string, name string) *appsv1.StatefulSet {
	key := fmt.Sprintf("%s/%s", namespace, name)
	item, _, err := c.statefulSetsInformer.GetIndexer().GetByKey(key)
	if err != nil {
		log.Error().Err(err).Msgf("Error during lookup of StatefulSet %s/%s", namespace, name)
	}
	if item != nil {
		return item.(*appsv1.StatefulSet)
	} else {
		return nil
	}
}

func (c *Client) NodesReadyCount() int {
	nodes := c.Nodes()
	nodeCountReady := 0
	for _, node := range nodes {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				nodeCountReady = nodeCountReady + 1
			}
		}
	}
	return nodeCountReady
}

func (c *Client) Nodes() []*corev1.Node {
	nodes, err := c.nodesLister.List(labels.Everything())
	if err != nil {
		log.Error().Err(err).Msgf("Error while fetching nodes")
		return []*corev1.Node{}
	}
	return nodes
}

func (c *Client) Events(since time.Time) *[]corev1.Event {
	events := c.eventsInformer.GetIndexer().List()
	//filter events by time
	result := filterEvents(events, since)
	//sort events by time
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTimestamp.Time.Before(result[j].LastTimestamp.Time)
	})
	return &result
}

func filterEvents(events []interface{}, since time.Time) []corev1.Event {
	var filtered []corev1.Event
	for _, event := range events {
		if event.(*corev1.Event).LastTimestamp.Time.After(since) {
			filtered = append(filtered, *event.(*corev1.Event))
		}
	}
	return filtered
}

func PrepareClient(stopCh <-chan struct{}) {
	clientset, rootApiPath := createClientset()
	K8S = CreateClient(clientset, stopCh, rootApiPath)
}

// CreateClient is visible for testing
func CreateClient(clientset kubernetes.Interface, stopCh <-chan struct{}, rootApiPath string) *Client {
	factory := informers.NewSharedInformerFactory(clientset, 0)

	// DeploymentsInformer.SetTransform() // TODO - Check whether we could use transformers to remove stuff --> save RAM?
	daemonSets := factory.Apps().V1().DaemonSets()
	daemonSetsInformer := daemonSets.Informer()
	deployments := factory.Apps().V1().Deployments()
	deploymentsInformer := deployments.Informer()
	pods := factory.Core().V1().Pods()
	podsInformer := pods.Informer()
	replicaSets := factory.Apps().V1().ReplicaSets()
	replicaSetsInformer := replicaSets.Informer()
	services := factory.Core().V1().Services()
	servicesInformer := services.Informer()
	statefulSets := factory.Apps().V1().StatefulSets()
	statefulSetsInformer := statefulSets.Informer()
	eventsInformer := factory.Core().V1().Events().Informer()
	nodes := factory.Core().V1().Nodes()
	nodesInformer := nodes.Informer()

	defer runtime.HandleCrash()

	go factory.Start(stopCh)

	log.Info().Msgf("Start Kubernetes cache sync.")
	if !cache.WaitForCacheSync(stopCh,
		daemonSetsInformer.HasSynced,
		deploymentsInformer.HasSynced,
		podsInformer.HasSynced,
		replicaSetsInformer.HasSynced,
		servicesInformer.HasSynced,
		statefulSetsInformer.HasSynced,
		eventsInformer.HasSynced,
		nodesInformer.HasSynced,
	) {
		log.Fatal().Msg("Timed out waiting for caches to sync")
	}
	log.Info().Msgf("Caches synced.")

	distribution := "kubernetes"
	if isOpenShift(rootApiPath) {
		distribution = "openshift"
	}

	return &Client{
		Distribution:         distribution,
		daemonSetsLister:     daemonSets.Lister(),
		daemonSetsInformer:   daemonSetsInformer,
		deploymentsLister:    deployments.Lister(),
		deploymentsInformer:  deploymentsInformer,
		podsLister:           pods.Lister(),
		podsInformer:         podsInformer,
		replicaSetsLister:    replicaSets.Lister(),
		replicaSetsInformer:  replicaSetsInformer,
		servicesLister:       services.Lister(),
		servicesInformer:     servicesInformer,
		statefulSetsLister:   statefulSets.Lister(),
		statefulSetsInformer: statefulSetsInformer,
		eventsInformer:       eventsInformer,
		nodesLister:          nodes.Lister(),
		nodesInformer:        nodesInformer,
	}
}

func isOpenShift(rootApiPath string) bool {
	return rootApiPath == "/oapi" || rootApiPath == "oapi"
}

func createClientset() (*kubernetes.Clientset, string) {
	config, err := rest.InClusterConfig()
	if err == nil {
		log.Info().Msgf("Extension is running inside a cluster, config found")
	} else if errors.Is(err, rest.ErrNotInCluster) {
		log.Info().Msgf("Extension is not running inside a cluster, try local .kube config")
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()
		// use the current context in kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
	}

	if err != nil {
		log.Fatal().Err(err).Msgf("Could not find kubernetes config")
	}

	config.UserAgent = "steadybit-extension-kubernetes"
	config.Timeout = time.Second * 10
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Could not create kubernetes client")
	}

	info, err := clientset.ServerVersion()
	if err != nil {
		log.Fatal().Err(err).Msgf("Could not fetch server version.")
	}

	log.Info().Msgf("Cluster connected! Kubernetes Server Version %+v", info)

	return clientset, config.APIPath
}

func IsExcludedFromDiscovery(objectMeta metav1.ObjectMeta) bool {
	discoveryEnabled, keyExists := objectMeta.Labels["steadybit.com/discovery-disabled"]
	if keyExists && strings.ToLower(discoveryEnabled) == "true" {
		return true
	}
	steadybitAgent, steadybitAgentKeyExists := objectMeta.Labels["com.steadybit.agent"]
	if steadybitAgentKeyExists && strings.ToLower(steadybitAgent) == "true" {
		return true
	}
	return false
}
