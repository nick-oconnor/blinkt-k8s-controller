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
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ngpitt/blinkt"

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
	Watch(controller cache.Controller)
	Close()
}

type ControllerObj struct {
	brightness   float64
	resourceList []*Resource
	blinkt       blinkt.Blinkt
}

type Resource struct {
	Name  string
	Color string
	State int
}

func NewController(brightness float64) Controller {
	return &ControllerObj{
		brightness,
		make([]*Resource, 0),
		blinkt.NewBlinkt(blinkt.Blue, brightness),
	}
}

func (o *ControllerObj) getResource(name string) *Resource {
	for _, r := range o.resourceList {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func (o *ControllerObj) Add(name, color string) {
	o.resourceList = append(o.resourceList, &Resource{name, color, added})
	o.updateBlinkt()
}

func (o *ControllerObj) Update(name, color string) {
	r := o.getResource(name)
	if r == nil || color == r.Color {
		return
	}
	r.Color = color
	r.State = updated
	o.updateBlinkt()
}

func (o *ControllerObj) Delete(name string) {
	r := o.getResource(name)
	if r == nil {
		return
	}
	r.State = deleted
	o.updateBlinkt()
}

func (o *ControllerObj) updateBlinkt() {
	i := 0
	for ; i < len(o.resourceList); i++ {
		r := o.resourceList[i]
		switch r.State {
		case added:
			if i < 8 {
				o.blinkt.Flash(i, blinkt.Green, o.brightness, 2, 50*time.Millisecond)
				o.blinkt.Set(i, r.Color, o.brightness)
			}
			r.State = unchanged
		case updated:
			if i < 8 {
				o.blinkt.Flash(i, blinkt.Blue, o.brightness, 2, 50*time.Millisecond)
				o.blinkt.Set(i, r.Color, o.brightness)
			}
			r.State = unchanged
		case deleted:
			if i < 8 {
				o.blinkt.Flash(i, blinkt.Red, o.brightness, 2, time.Millisecond*50)
			}
			o.resourceList = append(o.resourceList[:i], o.resourceList[i+1:]...)
			i--
		case unchanged:
			if i < 8 {
				o.blinkt.Set(i, r.Color, o.brightness)
			}
		}
	}
	for ; i < 8; i++ {
		o.blinkt.Set(i, blinkt.Off, 0)
	}
	o.blinkt.Show()
}

func (o *ControllerObj) Watch(controller cache.Controller) {
	log.Println("Starting the Blinkt controller")
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChan
		stopCh <- struct{}{}
	}()
	go controller.Run(stopCh)
	<-stopCh
}

func (o *ControllerObj) Close() {
	log.Println("Stopping the Blinkt controller")
	o.blinkt.Close(blinkt.Red, o.brightness)
}
