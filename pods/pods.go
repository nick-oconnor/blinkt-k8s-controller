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
	"log"
	"os"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/controller"
	"github.com/ngpitt/blinkt-k8s-controller/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/metricsutil"
)

func getPodColor(pod *api.Pod, heapsterClient *metricsutil.HeapsterMetricsClient) string {
	color := pod.Labels["blinktColor"]
	switch color {
	case "":
		color = blinkt.Blue
	case "cpu":
		metrics, _ := heapsterClient.GetPodMetrics(pod.Namespace, pod.Name, false, labels.Nothing())
		cpuUsed := int64(0)
		if len(metrics) > 0 {
			cpuUsed = metrics[0].Containers[0].Usage.Cpu().MilliValue()
		}
		cpuRequested := pod.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		color = helpers.RatioToColor(cpuRequested, cpuUsed)
	}
	return color
}

func main() {
	controller := controller.NewController()
	defer controller.Cleanup()
	coreClient := helpers.NewCoreClient()
	heapsterClient := metricsutil.DefaultHeapsterMetricsClient(coreClient)
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinktShow": "true"}.String(),
		FieldSelector: fields.Set{"spec.nodeName": nodeName}.String(),
	}
	controller.Watch(
		&api.Pod{},
		func(options metav1.ListOptions) (runtime.Object, error) {
			return coreClient.Pods(namespace).List(listOptions)
		},
		func(options metav1.ListOptions) (watch.Interface, error) {
			return coreClient.Pods(namespace).Watch(listOptions)
		},
		func(obj interface{}) {
			pod := obj.(*api.Pod)
			if controller.Add(pod.Name, getPodColor(pod, heapsterClient)) {
				log.Println("Pod", pod.Name, "added")
			}
		},
		func(oldObj, newObj interface{}) {
			pod := newObj.(*api.Pod)
			if controller.Update(pod.Name, getPodColor(pod, heapsterClient)) {
				log.Println("Pod", pod.Name, "updated")
			}
		},
		func(obj interface{}) {
			pod := obj.(*api.Pod)
			if controller.Delete(pod.Name) {
				log.Println("Pod", pod.Name, "deleted")
			}
		})
}
