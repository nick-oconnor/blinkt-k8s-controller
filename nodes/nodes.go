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

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/lib"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/metrics/pkg/apis/metrics/v1alpha1"
)

func getNodeColor(node *v1.Node) string {
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady {
			switch c.Status {
			case v1.ConditionTrue:
				color := node.Labels["blinktReadyColor"]
				switch color {
				case "":
					color = blinkt.Blue
				case "cpu":
					url := fmt.Sprintf("http://heapster.kube-system/apis/metrics/v1alpha1/nodes/%s", node.Name)
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
						metrics := v1alpha1.NodeMetrics{}
						err = json.Unmarshal(bytes, &metrics)
						if err != nil {
							log.Panicln(err.Error())
						}
						cpuUsed = metrics.Usage.Cpu().MilliValue()
					}
					cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
					color = lib.RatioToColor(cpuCapacity, cpuUsed)
				}
				return color
			case v1.ConditionFalse:
				color := node.Labels["blinktNotReadyColor"]
				if color == "" {
					color = blinkt.Off
				}
				return color
			}
		}
	}
	return blinkt.Red
}

func main() {
	controller := lib.NewController()
	defer controller.Cleanup()
	clientset := lib.NewClientset()
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinktShow": "true"}.String(),
	}
	controller.Watch(
		&v1.Node{},
		func(options metav1.ListOptions) (runtime.Object, error) {
			return clientset.CoreV1().Nodes().List(listOptions)
		},
		func(options metav1.ListOptions) (watch.Interface, error) {
			return clientset.CoreV1().Nodes().Watch(listOptions)
		},
		func(obj interface{}) {
			node := obj.(*v1.Node)
			if controller.Add(node.Name, getNodeColor(node)) {
				log.Println("Node ", node.Name, " added")
			}
		},
		func(oldObj, newObj interface{}) {
			node := newObj.(*v1.Node)
			if controller.Update(node.Name, getNodeColor(node)) {
				log.Println("Node ", node.Name, " updated")
			}
		},
		func(obj interface{}) {
			node := obj.(*v1.Node)
			if controller.Delete(node.Name) {
				log.Println("Node ", node.Name, " deleted")
			}
		})
}
