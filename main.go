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
	defaultPodColor        = "000B87"
	defaultStartColor      = "66FF00"
	defaultStopColor       = "FF0000"
	defaultPixelBrightness = 0.3
)

// BlinktK8sController is the main interface for a Blinkt Controller
type BlinktK8sController interface {
	Init(color, nodename, namespace string, stopCh <-chan struct{}) error
	StartWatchingPods()
}

type blinktPodImpl struct {
	bl        blinkt.Blinkt
	nodename  string
	namespace string

	podList []*v1.Pod

	resyncPeriod time.Duration

	clientset     *kubernetes.Clientset
	podController cache.Controller
	podStore      cache.Store
	listOptions   metav1.ListOptions
	stopCh        <-chan struct{}
}

// NewBlinktK8sController creates a new Blinkt Controller
func NewBlinktK8sController(color, nodename, namespace string, stopCh <-chan struct{}) (BlinktK8sController, error) {
	b := blinktPodImpl{}
	err := b.Init(color, nodename, namespace, stopCh)
	if err != nil {
		return nil, err
	}
	b.podList = make([]*v1.Pod, 0, 30)
	return &b, nil
}

func (b *blinktPodImpl) Init(color, nodename, namespace string, stopCh <-chan struct{}) error {
	log.Println("Starting BlinktK8sController")

	b.resyncPeriod = time.Minute * 30
	b.nodename = nodename
	b.namespace = namespace
	b.listOptions = metav1.ListOptions{
		LabelSelector: labels.Set{"blinkt": "show"}.String(),
		FieldSelector: fields.Set{"spec.nodeName": nodename}.String(),
	}
	b.stopCh = stopCh

	b.initBlinkt()

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	// external cluster config
	// var err error
	// config := &rest.Config{
	// 	Host: "http://127.0.0.1:8001",
	// 	//BearerToken: token,
	// }

	// creates the clientset
	b.clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func (b *blinktPodImpl) initBlinkt() {
	b.bl = blinkt.NewBlinkt()
	b.bl.ShowAnimOnStart = true
	b.bl.ShowAnimOnExit = true
	b.bl.Setup()
}

func (b *blinktPodImpl) addPod(pod *v1.Pod) {
	b.podList = append(b.podList, pod)
	numPods := len(b.podList)
	log.Println("Pod added: ", pod.Name, " Total Pods: ", numPods)
	if numPods < 9 {
		color := pod.Labels["blinktColor"]
		if color == "" {
			color = defaultPodColor
		}
		newPixel := numPods - 1
		b.bl.FlashPixel(newPixel, 2, defaultStartColor)
		b.bl.SetPixelHex(newPixel, color)
		b.bl.SetPixelBrightness(newPixel, defaultPixelBrightness)
		b.bl.Show()
	}
}

func (b *blinktPodImpl) removePod(pod *v1.Pod) {
	ok, podIdx := false, 0
	for i, p := range b.podList {
		if p.Name == pod.Name {
			ok = true
			podIdx = i
			break
		}
	}
	if !ok {
		log.Println("Error: pod not found in list")
		return
	}
	if podIdx == len(b.podList)-1 {
		b.podList = b.podList[:podIdx]
	} else {
		b.podList = append(b.podList[:podIdx], b.podList[podIdx+1:]...)
	}
	endIdx := len(b.podList)
	log.Println("Pod removed: ", pod.Name, " Total Pods: ", endIdx)
	if podIdx < 8 {
		b.bl.FlashPixel(podIdx, 2, defaultStopColor)
		if endIdx > 8 {
			endIdx = 8
		}
		if endIdx < 8 {
			for pixel := endIdx; pixel < 8; pixel++ {
				b.bl.SetPixel(pixel, 0, 0, 0)
			}
		}
		for pixel, pod := range b.podList[:endIdx] {
			color := pod.Labels["blinktColor"]
			if color == "" {
				color = defaultPodColor
			}
			b.bl.SetPixelHex(pixel, color)
			b.bl.SetPixelBrightness(pixel, defaultPixelBrightness)
		}
		b.bl.Show()
	}
}

func (b *blinktPodImpl) newResourceEventHandlerFuncs() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			b.addPod(obj.(*v1.Pod))
		},
		UpdateFunc: func(old, new interface{}) {},
		DeleteFunc: func(obj interface{}) {
			b.removePod(obj.(*v1.Pod))
		},
	}
}

// watchPods starts the watch of Kubernetes Pods resources and updates the corresponding store
func (b *blinktPodImpl) StartWatchingPods() {

	b.podStore, b.podController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return b.clientset.CoreV1().Pods(b.namespace).List(b.listOptions)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return b.clientset.CoreV1().Pods(b.namespace).Watch(b.listOptions)
			},
		},
		&v1.Pod{},
		b.resyncPeriod,
		b.newResourceEventHandlerFuncs(),
	)

	go b.podController.Run(b.stopCh)

	<-b.stopCh

	log.Println("Shutting down BlinktK8sController")
	b.bl.Cleanup()
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

	color := os.Getenv("COLOR")
	nodename := os.Getenv("NODE_NAME")
	namespace := os.Getenv("NAMESPACE")
	b, err := NewBlinktK8sController(color, nodename, namespace, stopCh)
	if err != nil {
		panic(err.Error())
	}
	b.StartWatchingPods()
}
