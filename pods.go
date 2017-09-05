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
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

func getPodColor(pod *v1.Pod) string {
	color := pod.Labels["blinktColor"]
	if color == "" {
		color = blinkt.Blue
	}
	return color
}

func main() {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	brightness, _ := strconv.ParseFloat(os.Getenv("BRIGHTNESS"), 64)
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	c := lib.NewController(brightness)
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
		time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				log.Println("Pod ", pod.Name, " added")
				c.Add(pod.Name, getPodColor(pod))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				pod := newObj.(*v1.Pod)
				log.Println("Pod ", pod.Name, " updated")
				c.Update(pod.Name, getPodColor(pod))
			},
			DeleteFunc: func(obj interface{}) {
				pod := obj.(*v1.Pod)
				log.Println("Pod ", pod.Name, " deleted")
				c.Delete(pod.Name)
			},
		},
	)
	c.Watch(controller)
}
