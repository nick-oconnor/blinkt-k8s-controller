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
	"strconv"
	"time"

	"github.com/ngpitt/blinkt"
	"github.com/ngpitt/blinkt-k8s-controller/lib"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func getNodeColor(node *v1.Node) string {
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady {
			switch c.Status {
			case v1.ConditionTrue:
				color := node.Labels["blinktReadyColor"]
				if color == "" {
					color = blinkt.Green
				}
				return color
			case v1.ConditionFalse:
				color := node.Labels["blinktNotReadyColor"]
				if color == "" {
					color = blinkt.Red
				}
				return color
			}
		}
	}
	return blinkt.Red
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
	c := lib.NewController(brightness)
	defer c.Close()
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set{"blinkt": "show"}.String(),
	}
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return clientset.CoreV1().Nodes().List(listOptions)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return clientset.CoreV1().Nodes().Watch(listOptions)
			},
		},
		&v1.Node{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				if c.Add(node.Name, getNodeColor(node)) {
					log.Println("Node ", node.Name, " added")
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				node := newObj.(*v1.Node)
				if c.Update(node.Name, getNodeColor(node)) {
					log.Println("Node ", node.Name, " updated")
				}
			},
			DeleteFunc: func(obj interface{}) {
				node := obj.(*v1.Node)
				if c.Delete(node.Name) {
					log.Println("Node ", node.Name, " deleted")
				}
			},
		},
	)
	c.Watch(controller)
}
