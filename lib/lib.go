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

package lib

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ngpitt/blinkt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	added     = iota
	updated   = iota
	deleted   = iota
	unchanged = iota
)

type Controller interface {
	Add(name, color string) bool
	Update(name, color string) bool
	Delete(name string) bool
	Watch(objType runtime.Object,
		listFunc cache.ListFunc,
		watchFunc cache.WatchFunc,
		addFunc func(obj interface{}),
		updateFunc func(oldObj, newObj interface{}),
		deleteFunc func(obj interface{}))
	Cleanup()
}

type ControllerObj struct {
	brightness   float64
	resourceList []resource
	blinkt       blinkt.Blinkt
}

type resource struct {
	name  string
	color string
	state int
}

func NewController() Controller {
	brightness, err := strconv.ParseFloat(os.Getenv("BRIGHTNESS"), 64)
	if err != nil {
		log.Panicln(err.Error())
	}
	return &ControllerObj{
		brightness,
		[]resource{},
		blinkt.NewBlinkt(blinkt.Blue, brightness),
	}
}

func NewClientset() *kubernetes.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panicln(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln(err.Error())
	}
	return clientset
}

func RatioToColor(target, actual int64) string {
	ratio := math.Min(2, 2*float64(actual)/float64(target))
	b := int(math.Max(0, 255*(1-ratio)))
	r := int(math.Max(0, 255*(ratio-1)))
	g := 255 - b - r
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}

func (o *ControllerObj) Add(name, color string) bool {
	o.resourceList = append(o.resourceList, resource{name, color, added})
	o.updateBlinkt()
	return true
}

func (o *ControllerObj) Update(name, color string) bool {
	r := o.getResource(name)
	if r == nil || color == r.color {
		return false
	}
	r.color = color
	r.state = updated
	o.updateBlinkt()
	return true
}

func (o *ControllerObj) Delete(name string) bool {
	r := o.getResource(name)
	if r == nil {
		return false
	}
	r.state = deleted
	o.updateBlinkt()
	return true
}

func (o *ControllerObj) Watch(
	objType runtime.Object,
	listFunc cache.ListFunc,
	watchFunc cache.WatchFunc,
	addFunc func(obj interface{}),
	updateFunc func(oldObj, newObj interface{}),
	deleteFunc func(obj interface{})) {
	log.Println("Starting the Blinkt controller")
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		stopCh <- struct{}{}
	}()
	resyncPeriod, err := time.ParseDuration(os.Getenv("RESYNC_PERIOD"))
	if err != nil {
		log.Panicln(err.Error())
	}
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		objType,
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: deleteFunc,
		},
	)
	go controller.Run(stopCh)
	<-stopCh
}

func (o *ControllerObj) Cleanup() {
	log.Println("Stopping the Blinkt controller")
	o.blinkt.Close(blinkt.Red, o.brightness)
}

func (o *ControllerObj) getResource(name string) *resource {
	for i, r := range o.resourceList {
		if r.name == name {
			return &o.resourceList[i]
		}
	}
	return nil
}

func (o *ControllerObj) updateBlinkt() {
	i := 0
	for ; i < len(o.resourceList); i++ {
		r := &o.resourceList[i]
		switch r.state {
		case added:
			fallthrough
		case updated:
			if i < 8 {
				o.blinkt.Flash(i, r.color, o.brightness, 2, 50*time.Millisecond)
				o.blinkt.Set(i, r.color, o.brightness)
			}
			r.state = unchanged
		case deleted:
			if i < 8 {
				o.blinkt.Flash(i, r.color, o.brightness, 2, 50*time.Millisecond)
			}
			o.resourceList = append(o.resourceList[:i], o.resourceList[i+1:]...)
			i--
		case unchanged:
			if i < 8 {
				o.blinkt.Set(i, r.color, o.brightness)
			}
		}
	}
	for ; i < 8; i++ {
		o.blinkt.Set(i, blinkt.Off, 0)
	}
	o.blinkt.Show()
}
