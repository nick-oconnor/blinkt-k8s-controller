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
	"flag"
	"time"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/controller"
	"github.com/ngpitt/blinkt-k8s-controller/helpers"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func main() {
	brightness := flag.Float64("brightness", 0.25, "strip brightness")
	resyncPeriod := flag.Duration("resync_period", 5*time.Second, "resync period")
	flag.Parse()
	c := controller.NewController(*brightness)
	defer c.Cleanup()
	kubernetesClientset, heapsterClientset := helpers.NewClientsets()
	c.Watch(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				return kubernetesClientset.CoreV1().Nodes().List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				return kubernetesClientset.CoreV1().Nodes().Watch(options)
			},
		},
		&v1.Node{},
		*resyncPeriod,
		func(obj interface{}) string {
			node := obj.(*v1.Node)
			for _, c := range node.Status.Conditions {
				if c.Type == v1.NodeReady {
					switch c.Status {
					case v1.ConditionTrue:
						color := node.Labels["blinktReadyColor"]
						switch color {
						case "":
							color = blinkt.Blue
						case "cpu":
							metrics, err := heapsterClientset.MetricsV1alpha1().NodeMetricses().Get(node.Name, metav1.GetOptions{})
							cpuUsed := int64(0)
							if err == nil {
								cpuUsed = metrics.Usage.Cpu().MilliValue()
							}
							cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
							color = helpers.RatioToColor(cpuCapacity, cpuUsed)
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
		})
}
