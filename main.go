// The MIT License (MIT)
//
// Copyright (c) 2016 Apprenda Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ikester/blinkt"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/fields"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/rest"
	"k8s.io/client-go/1.5/tools/cache"
)

// BlinktK8sController is the main interface for a Blinkt Controller
type BlinktK8sController interface {
	Init(color, nodename, namespace string, stopCh <-chan struct{}) error
	StartWatchingPods()
}

type blinktPodImpl struct {
	bl        blinkt.Blinkt
	color     string
	nodename  string
	namespace string
	r, g, b   int

	numPods int

	resyncPeriod time.Duration

	clientset     *kubernetes.Clientset
	podController *cache.Controller
	podStore      cache.Store
	listOptions   api.ListOptions
	stopCh        <-chan struct{}
}

// NewBlinktK8sController creates a new Blinkt Controller
func NewBlinktK8sController(color, nodename, namespace string, stopCh <-chan struct{}) (BlinktK8sController, error) {
	b := blinktPodImpl{}
	err := b.Init(color, nodename, namespace, stopCh)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (b *blinktPodImpl) Init(color, nodename, namespace string, stopCh <-chan struct{}) error {
	log.Println("Starting BlinktK8sController")
	b.color = color
	if color == "" {
		color = "#FF00FF"
	}
	b.r, b.g, b.b = blinkt.Hex2RGB(color)

	b.resyncPeriod = time.Minute * 30
	b.nodename = nodename
	b.namespace = namespace
	b.listOptions = api.ListOptions{
		LabelSelector: labels.Set{"blinkt": "show"}.AsSelector(),
		FieldSelector: fields.Set{"spec.nodeName": nodename}.AsSelector(),
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

func (b *blinktPodImpl) updateDisplay(up bool) {
	log.Printf("There are now %d blinkt labeled pods on this node\n", b.numPods)
	if up && b.numPods < 9 {
		newPixel := b.numPods - 1
		b.bl.FlashPixel(newPixel, 2, "#66FF00")
		b.bl.SetPixel(newPixel, b.r, b.g, b.b)
		b.bl.SetPixelBrightness(newPixel, 0.5)
	} else if !up && b.numPods < 8 {
		oldPixel := b.numPods
		b.bl.FlashPixel(oldPixel, 2, "#FF0000")
		b.bl.SetPixel(oldPixel, 0, 0, 0)
		b.bl.SetPixelBrightness(oldPixel, 0.5)
	}
	b.bl.Show()
}

func (b *blinktPodImpl) newResourceEventHandlerFuncs() cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Println("Pod added: ", obj.(*v1.Pod).Name)
			b.numPods++
			b.updateDisplay(true)
		},
		UpdateFunc: func(old, new interface{}) {},
		DeleteFunc: func(obj interface{}) {
			log.Println("Pod deleted: ", obj.(*v1.Pod).Name)
			b.numPods--
			b.updateDisplay(false)
		},
	}
}

// watchPods starts the watch of Kubernetes Pods resources and updates the corresponding store
func (b *blinktPodImpl) StartWatchingPods() {

	b.podStore, b.podController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return b.clientset.CoreClient.Get().
					Namespace(b.namespace).
					Resource("pods").
					VersionedParams(&options, api.ParameterCodec).
					FieldsSelectorParam(b.listOptions.FieldSelector).
					LabelsSelectorParam(b.listOptions.LabelSelector).
					Do().
					Get()
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return b.clientset.CoreClient.Get().
					Prefix("watch").
					Namespace(b.namespace).
					Resource("pods").
					VersionedParams(&options, api.ParameterCodec).
					FieldsSelectorParam(b.listOptions.FieldSelector).
					LabelsSelectorParam(b.listOptions.LabelSelector).
					Watch()
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
