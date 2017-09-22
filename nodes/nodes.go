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
	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/controller"
	"github.com/ngpitt/blinkt-k8s-controller/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/metricsutil"
)

func getNodeColor(node *api.Node, heapsterClient *metricsutil.HeapsterMetricsClient) string {
	for _, c := range node.Status.Conditions {
		if c.Type == api.NodeReady {
			switch c.Status {
			case api.ConditionTrue:
				color := node.Labels["blinktReadyColor"]
				switch color {
				case "":
					color = blinkt.Blue
				case "cpu":
					nodeMetrics, _ := heapsterClient.GetNodeMetrics(node.Name, labels.Nothing())
					cpuUsed := int64(0)
					if len(nodeMetrics) > 0 {
						cpuUsed = nodeMetrics[0].Usage.Cpu().MilliValue()
					}
					cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
					color = helpers.RatioToColor(cpuCapacity, cpuUsed)
				}
				return color
			case api.ConditionFalse:
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
	controller := controller.NewController()
	defer controller.Cleanup()
	coreClient := helpers.NewCoreClient()
	heapsterClient := metricsutil.DefaultHeapsterMetricsClient(coreClient)
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinktShow": "true"}.String(),
	}
	controller.Watch(
		&api.Node{},
		func(options metav1.ListOptions) (runtime.Object, error) {
			return coreClient.Nodes().List(listOptions)
		},
		func(options metav1.ListOptions) (watch.Interface, error) {
			return coreClient.Nodes().Watch(listOptions)
		},
		func(obj interface{}) {
			node := obj.(*api.Node)
			controller.Add(node.Name, getNodeColor(node, heapsterClient))
		},
		func(oldObj, newObj interface{}) {
			node := newObj.(*api.Node)
			controller.Update(node.Name, getNodeColor(node, heapsterClient))
		},
		func(obj interface{}) {
			node := obj.(*api.Node)
			controller.Delete(node.Name)
		})
}
