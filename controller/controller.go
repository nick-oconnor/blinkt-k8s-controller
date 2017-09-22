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
	Add(name, color string)
	Update(name, color string)
	Delete(name string)
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
	resyncPeriod time.Duration
	resourceList []resource
	blinkt       blinkt.Blinkt
}

type resource struct {
	name    string
	color   string
	state   int
	updated time.Time
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

func (o *ControllerObj) Add(name, color string) {
	o.resourceList = append(o.resourceList, resource{name, color, added, time.Now()})
	o.updateBlinkt()
}

func (o *ControllerObj) Update(name, color string) {
	r := o.getResource(name)
	if r == nil {
		o.Add(name, color)
		return
	}
	r.updated = time.Now()
	if color == r.color {
		return
	}
	r.color = color
	r.state = updated
	o.updateBlinkt()
}

func (o *ControllerObj) Delete(name string) {
	r := o.getResource(name)
	if r == nil {
		return
	}
	r.state = deleted
	o.updateBlinkt()
}

func (o *ControllerObj) Watch(
	objType runtime.Object,
	listFunc cache.ListFunc,
	watchFunc cache.WatchFunc,
	addFunc func(obj interface{}),
	updateFunc func(oldObj, newObj interface{}),
	deleteFunc func(obj interface{})) {
	_, controller := cache.NewInformer(
		&cache.ListWatch{
			ListFunc:  listFunc,
			WatchFunc: watchFunc,
		},
		objType,
		o.resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    addFunc,
			UpdateFunc: updateFunc,
			DeleteFunc: deleteFunc,
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
		if r.updated.Before(time.Now().Add(-3 * o.resyncPeriod)) {
			r.state = deleted
		}
		switch r.state {
		case added:
			log.Println("Added", r.name)
			fallthrough
		case updated:
			log.Println("Updated", r.name)
			if i < 8 {
				o.blinkt.Flash(i, r.color, o.brightness, 2, 50*time.Millisecond)
				o.blinkt.Set(i, r.color, o.brightness)
			}
			r.state = unchanged
		case deleted:
			log.Println("Deleted", r.name)
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
