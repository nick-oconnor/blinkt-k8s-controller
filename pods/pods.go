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
	"os"
	"time"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/controller"
	"github.com/ngpitt/blinkt-k8s-controller/helpers"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

func main() {
	brightness := flag.Float64("brightness", 0.25, "strip brightness")
	resyncPeriod := flag.Duration("resync_period", 5*time.Second, "resync period")
	namespace := flag.String("namespace", v1.NamespaceDefault, "namespace to monitor")
	flag.Parse()
	nodeName := os.Getenv("NODE_NAME")
	c := controller.NewController(*brightness)
	defer c.Cleanup()
	kubernetesClient, metricsClient := helpers.NewClients()
	c.Watch(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				options.FieldSelector = fields.Set{"spec.nodeName": nodeName}.String()
				return kubernetesClient.Core().Pods(*namespace).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = labels.Set{"blinktShow": "true"}.String()
				options.FieldSelector = fields.Set{"spec.nodeName": nodeName}.String()
				return kubernetesClient.Core().Pods(*namespace).Watch(options)
			},
		},
		&v1.Pod{},
		*resyncPeriod,
		func(obj interface{}) string {
			pod := obj.(*v1.Pod)
			color := pod.Labels["blinktColor"]
			switch color {
			case "":
				color = blinkt.Blue
			case "cpu":
				metrics, err := metricsClient.Metrics().PodMetricses(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
				cpuUsed := int64(0)
				if err == nil {
					cpuUsed = metrics.Containers[0].Usage.Cpu().MilliValue()
				}
				cpuRequested := pod.Spec.Containers[0].Resources.Requests.Cpu().MilliValue()
				color = helpers.RatioToColor(cpuRequested, cpuUsed)
			}
			return color
		})
}
