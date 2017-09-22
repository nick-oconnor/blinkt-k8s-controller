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

package controller

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ngpitt/blinkt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

const (
	added     = iota
	updated   = iota
	deleted   = iota
	unchanged = iota
)

type Controller interface {
	Watch(objType runtime.Object,
		listFunc cache.ListFunc,
		watchFunc cache.WatchFunc,
		colorFunc func(obj interface{}) string)
	Cleanup()
}

type ControllerObj struct {
	brightness   float64
	resyncPeriod time.Duration
	resourceList []resource
	blinkt       blinkt.Blinkt
}

type resource struct {
	key   string
	color string
	state int
}

func NewController() Controller {
	brightness, err := strconv.ParseFloat(os.Getenv("BRIGHTNESS"), 64)
	if err != nil {
		log.Panicln(err.Error())
	}
	resyncPeriod, err := time.ParseDuration(os.Getenv("RESYNC_PERIOD"))
	if err != nil {
		log.Panicln(err.Error())
	}
	return &ControllerObj{
		brightness,
		resyncPeriod,
		[]resource{},
		blinkt.NewBlinkt(blinkt.Blue, brightness),
	}
}

func (o *ControllerObj) Watch(
	objType runtime.Object,
	listFunc cache.ListFunc,
	watchFunc cache.WatchFunc,
	colorFunc func(obj interface{}) string) {
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		objType,
		o.resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key := keyFunc(obj)
				color := colorFunc(obj)
				r := resource{key, color, added}
				log.Print("Adding ", r.key, "...\n")
				o.resourceList = append(o.resourceList, r)
				o.updateBlinkt()
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				key := keyFunc(newObj)
				color := colorFunc(newObj)
				r := o.getResource(key)
				if color == r.color {
					return
				}
				log.Print("Updating ", r.key, "...\n")
				r.color = color
				r.state = updated
				o.updateBlinkt()
			},
			DeleteFunc: func(obj interface{}) {
				key := keyFunc(obj)
				r := o.getResource(key)
				log.Println("Deleting ", r.key, "...\n")
				r.state = deleted
				o.updateBlinkt()
			},
		},
	)
	sigs := make(chan os.Signal)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	stopCh := make(chan struct{})
	go func() {
		<-sigs
		log.Println("Stopping the Blinkt controller...")
		close(stopCh)
	}()
	log.Println("Starting the Blinkt controller...")
	controller.Run(stopCh)
}

func (o *ControllerObj) Cleanup() {
	o.blinkt.Cleanup(blinkt.Red, o.brightness)
}

func (o *ControllerObj) getResource(key string) *resource {
	for i, r := range o.resourceList {
		if r.key == key {
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

func keyFunc(obj interface{}) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Panicln(err.Error())
	}
	return key
}
