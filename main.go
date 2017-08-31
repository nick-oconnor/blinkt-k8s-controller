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
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ikester/blinkt"

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

const (
	addedColor               = "00FF00"
	updatedColor             = "000B87"
	removedColor             = "FF0000"
	defaultPodColor          = "000B87"
	defaultNodeReadyColor    = "000B87"
	defaultNodeNotReadyColor = "FF0000"
)

const (
	monitorPods  = "pods"
	monitorNodes = "nodes"
)

const (
	added     = iota
	updated   = iota
	removed   = iota
	unchanged = iota
)

type blinktInterface interface {
	Init(monitor, nodeName, namespace string, brightness float64, stopCh <-chan struct{}) error
	Watch()
}

type blinktDaemon struct {
	monitor    string
	nodeName   string
	namespace  string
	brightness float64

	resourceList []*resource

	blinkt       blinkt.Blinkt
	listOptions  metav1.ListOptions
	resyncPeriod time.Duration
	clientset    *kubernetes.Clientset
	controller   cache.Controller
	store        cache.Store
	stopCh       <-chan struct{}
}

type resource struct {
	name  string
	color string
	state int
}

func NewBlinktDaemon(monitor, nodeName, namespace string, brightness float64, stopCh <-chan struct{}) (blinktInterface, error) {
	d := blinktDaemon{}

	err := d.Init(monitor, nodeName, namespace, brightness, stopCh)
	if err != nil {
		return nil, err
	}

	return &d, nil
}

func (d *blinktDaemon) Init(monitor, nodeName, namespace string, brightness float64, stopCh <-chan struct{}) error {
	log.Println("Starting the Blinkt daemon")

	d.monitor = monitor
	d.nodeName = nodeName
	d.namespace = namespace
	d.brightness = brightness
	d.resourceList = make([]*resource, 0)

	d.blinkt = blinkt.NewBlinkt()
	d.blinkt.ShowAnimOnStart = true
	d.blinkt.ShowAnimOnExit = true
	d.blinkt.Setup()

	switch monitor {
	case monitorPods:
		d.listOptions = metav1.ListOptions{
			LabelSelector: labels.Set{"blinkt": "show"}.String(),
			FieldSelector: fields.Set{"spec.nodeName": nodeName}.String(),
		}
	case monitorNodes:
		d.listOptions = metav1.ListOptions{
			LabelSelector: labels.Set{"blinkt": "show"}.String(),
		}
	}

	d.resyncPeriod = time.Minute * 5
	d.stopCh = stopCh

	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	d.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func getPodColor(pod *v1.Pod) string {
	color := pod.Labels["blinktColor"]
	if color == "" {
		color = defaultPodColor
	}
	return color
}

func getNodeColor(node *v1.Node) string {
	for _, c := range node.Status.Conditions {
		if c.Type == v1.NodeReady {
			switch c.Status {
			case v1.ConditionTrue:
				color := node.Labels["blinktReadyColor"]
				if color == "" {
					color = defaultNodeReadyColor
				}
				return color
			case v1.ConditionFalse:
				color := node.Labels["blinktNotReadyColor"]
				if color == "" {
					color = defaultNodeNotReadyColor
				}
				return color
			}
		}
	}
	return defaultNodeNotReadyColor
}

func (d *blinktDaemon) getResource(name string) *resource {
	for _, resource := range d.resourceList {
		if resource.name == name {
			return resource
		}
	}
	return nil
}

func (d *blinktDaemon) updatePixels() {
	i := 0
	for ; i < len(d.resourceList); i++ {
		r := d.resourceList[i]
		switch r.state {
		case added:
			if i < 8 {
				d.blinkt.FlashPixel(i, 2, addedColor)
				d.blinkt.SetPixelHex(i, r.color)
				d.blinkt.SetPixelBrightness(i, d.brightness)
			}
			r.state = unchanged
		case updated:
			if i < 8 {
				d.blinkt.FlashPixel(i, 2, updatedColor)
				d.blinkt.SetPixelHex(i, r.color)
				d.blinkt.SetPixelBrightness(i, d.brightness)
			}
			r.state = unchanged
		case removed:
			if i < 8 {
				d.blinkt.FlashPixel(i, 2, removedColor)
			}
			d.resourceList = append(d.resourceList[:i], d.resourceList[i+1:]...)
			i--
		case unchanged:
			d.blinkt.SetPixelHex(i, r.color)
			d.blinkt.SetPixelBrightness(i, d.brightness)
		}
	}
	for ; i < 8; i++ {
		d.blinkt.SetPixel(i, 0, 0, 0)
	}
	d.blinkt.Show()
}

func (d *blinktDaemon) addPod(pod *v1.Pod) {
	log.Println("Pod ", pod.Name, " added (", len(d.resourceList)+1, " known pods)")
	d.resourceList = append(d.resourceList, &resource{pod.Name, getPodColor(pod), added})
	d.updatePixels()
}

func (d *blinktDaemon) addNode(node *v1.Node) {
	log.Println("Node ", node.Name, " added (", len(d.resourceList)+1, " known nodes)")
	d.resourceList = append(d.resourceList, &resource{node.Name, getNodeColor(node), added})
	d.updatePixels()
}

func (d *blinktDaemon) updatePod(pod *v1.Pod) {
	resource := d.getResource(pod.Name)
	if resource == nil {
		log.Println("Pod ", pod.Name, " not found in list")
	}
	log.Println("Pod ", pod.Name, " updated (", len(d.resourceList), " known pods)")
	color := getPodColor(pod)
	if color != resource.color {
		resource.color = color
		resource.state = updated
		d.updatePixels()
	}
}

func (d *blinktDaemon) updateNode(node *v1.Node) {
	resource := d.getResource(node.Name)
	if resource == nil {
		log.Println("Node ", node.Name, " not found in list")
		return
	}
	log.Println("Node ", node.Name, " updated (", len(d.resourceList), " known nodes)")
	color := getNodeColor(node)
	if color != resource.color {
		resource.color = color
		resource.state = updated
		d.updatePixels()
	}
}

func (d *blinktDaemon) removePod(pod *v1.Pod) {
	resource := d.getResource(pod.Name)
	if resource == nil {
		log.Println("Pod ", pod.Name, " not found in list")
		return
	}
	log.Println("Pod ", pod.Name, " removed (", len(d.resourceList)-1, " known pods)")
	resource.state = removed
	d.updatePixels()
}

func (d *blinktDaemon) removeNode(node *v1.Node) {
	resource := d.getResource(node.Name)
	if resource == nil {
		log.Println("Node ", node.Name, " not found in list")
		return
	}
	log.Println("Node ", node.Name, " removed (", len(d.resourceList)-1, " known nodes)")
	resource.state = removed
	d.updatePixels()
}

func (d *blinktDaemon) newPodEventHandlerFuncs() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d.addPod(obj.(*v1.Pod))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			d.updatePod(newObj.(*v1.Pod))
		},
		DeleteFunc: func(obj interface{}) {
			d.removePod(obj.(*v1.Pod))
		},
	}
}

func (d *blinktDaemon) newNodeEventHandlerFuncs() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d.addNode(obj.(*v1.Node))
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			d.updateNode(newObj.(*v1.Node))
		},
		DeleteFunc: func(obj interface{}) {
			d.removeNode(obj.(*v1.Node))
		},
	}
}

func (d *blinktDaemon) Watch() {
	switch d.monitor {
	case monitorPods:
		d.store, d.controller = cache.NewInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return d.clientset.CoreV1().Pods(d.namespace).List(d.listOptions)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return d.clientset.CoreV1().Pods(d.namespace).Watch(d.listOptions)
				},
			},
			&v1.Pod{},
			d.resyncPeriod,
			d.newPodEventHandlerFuncs(),
		)
	case monitorNodes:
		d.store, d.controller = cache.NewInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return d.clientset.CoreV1().Nodes().List(d.listOptions)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return d.clientset.CoreV1().Nodes().Watch(d.listOptions)
				},
			},
			&v1.Node{},
			d.resyncPeriod,
			d.newNodeEventHandlerFuncs(),
		)
	}

	go d.controller.Run(d.stopCh)

	<-d.stopCh

	log.Println("Stopping the Blinkt daemon")
	d.blinkt.Cleanup()
}

func main() {
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		stopCh <- struct{}{}
	}()
	monitor := os.Getenv("MONITOR")
	brightness, err := strconv.ParseFloat(os.Getenv("BRIGHTNESS"), 64)
	if err != nil {
		panic(err.Error())
	}
	nodeName := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	d, err := NewBlinktDaemon(monitor, nodeName, namespace, brightness, stopCh)
	if err != nil {
		panic(err.Error())
	}
	d.Watch()
}
