// Copyright (c) 2017 Apprenda, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/lib"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	metrics "k8s.io/heapster/metrics/apis/metrics/v1alpha1"
)

func getPodColor(pod *v1.Pod, clientset *kubernetes.Clientset) string {
	color := pod.Labels["blinktColor"]
	switch color {
	case "":
		return blinkt.Blue
	case "cpu":
		url := fmt.Sprintf("apis/metrics/v1alpha1/namespaces/%s/pods/%s", pod.Namespace, pod.Name)
		response, err := clientset.Core().RESTClient().Get().
			Namespace("kube-system").
			Prefix("proxy").
			Resource("services").
			Name("heapster").
			Suffix(url).
			Do().Raw()
		if err != nil {
			log.Panicln(err.Error())
		}
		var podMetrics metrics.PodMetrics
		err = json.Unmarshal(response, &podMetrics)
		if err != nil {
			log.Panicln(err.Error())
		}
		cpuRequested := pod.Spec.Containers[0].Resources.Requests.Cpu().Value()
		cpuUsed := podMetrics.Containers[0].Usage.Cpu().Value()
		ratio := math.Min(2, 2*float64(cpuRequested)/float64(cpuUsed))
		b := int(math.Max(0, 255*(1-ratio)))
		r := int(math.Max(0, 255*(ratio-1)))
		g := 255 - b - r
		return fmt.Sprintf("%02X%02X%02X", r, g, b)
	}
	return color
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panicln(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln(err.Error())
	}
	brightness, err := strconv.ParseFloat(os.Getenv("BRIGHTNESS"), 64)
	if err != nil {
		log.Panicln(err.Error())
	}
	resyncPeriod, err := time.ParseDuration(os.Getenv("RESYNC_PERIOD"))
	if err != nil {
		log.Panicln(err.Error())
	}
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	c := lib.NewController(brightness)
	defer c.Close()
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinkt": "show"}.String(),
		FieldSelector: fields.Set{"spec.nodeName": nodeName}.String(),
	}
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.CoreV1().Pods(namespace).List(listOptions)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.CoreV1().Pods(namespace).Watch(listOptions)
			},
		},
		&v1.Pod{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				if c.Add(pod.Name, getPodColor(pod, clientset)) {
					log.Println("Pod ", pod.Name, " added")
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				pod := newObj.(*v1.Pod)
				if c.Update(pod.Name, getPodColor(pod, clientset)) {
					log.Println("Pod ", pod.Name, " updated")
				}
			},
			DeleteFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				if c.Delete(pod.Name) {
					log.Println("Pod ", pod.Name, " deleted")
				}
			},
		},
	)
	c.Watch(controller)
}
