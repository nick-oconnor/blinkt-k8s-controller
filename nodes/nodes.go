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

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubectl/metricsutil"
)

func main() {
	brightness := flag.Float64("brightness", 0.25, "strip brightness")
	resyncPeriod := flag.Duration("resync_period", 5*time.Second, "resync period")
	flag.Parse()
	c := controller.NewController(*brightness)
	defer c.Cleanup()
	client := helpers.NewCoreClient()
	heapsterClient := metricsutil.DefaultHeapsterMetricsClient(client)
	c.Watch(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				return client.Nodes().List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				return client.Nodes().Watch(options)
			},
		},
		&api.Node{},
		*resyncPeriod,
		func(obj interface{}) string {
			node := obj.(*api.Node)
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
		})
}
