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
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/lib"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics/v1alpha1"
)

func getPodColor(pod *v1.Pod) string {
	color := pod.Labels["blinktColor"]
	switch color {
	case "":
		color = blinkt.Blue
	case "cpu":
		url := fmt.Sprintf("http://heapster.kube-system/apis/metrics/v1alpha1/namespaces/%s/pods/%s", pod.Namespace, pod.Name)
		response, err := http.Get(url)
		if err != nil {
			log.Panicln(err.Error())
		}
		cpuUsed := int64(0)
		if response.StatusCode == http.StatusOK {
			bytes, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Panicln(err.Error())
			}
			metrics := v1alpha1.PodMetrics{}
			err = json.Unmarshal(bytes, &metrics)
			if err != nil {
				log.Panicln(err.Error())
			}
			cpuUsed = metrics.Containers[0].Usage.Cpu().MilliValue()
		}
		cpuRequested := pod.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
		color = lib.RatioToColor(cpuRequested, cpuUsed)
	}
	return color
}

func main() {
	controller := lib.NewController()
	defer controller.Cleanup()
	clientset := lib.NewClientset()
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinktShow": "true"}.String(),
		FieldSelector: fields.Set{"spec.nodeName": nodeName}.String(),
	}
	controller.Watch(
		&v1.Pod{},
		func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Pods(namespace).List(listOptions)
		},
		func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Pods(namespace).Watch(listOptions)
		},
		func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if controller.Add(pod.Name, getPodColor(pod)) {
				log.Println("Pod ", pod.Name, " added")
			}
		},
		func(oldObj, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			if controller.Update(pod.Name, getPodColor(pod)) {
				log.Println("Pod ", pod.Name, " updated")
			}
		},
		func(obj interface{}) {
			pod := obj.(*v1.Pod)
			if controller.Delete(pod.Name) {
				log.Println("Pod ", pod.Name, " deleted")
			}
		})
}
