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
	Add(name, color string) bool
	Update(name, color string) bool
	Delete(name string) bool
	Watch(controller cache.Controller)
	Close()
}

type ControllerObj struct {
	brightness   float64
	resourceList []*resource
	blinkt       blinkt.Blinkt
}

type resource struct {
	name  string
	color string
	state int
}

func NewController(brightness float64) Controller {
	return &ControllerObj{
		brightness,
		make([]*resource, 0),
		blinkt.NewBlinkt(blinkt.Blue, brightness),
	}
}

func (o *ControllerObj) Add(name, color string) bool {
	o.resourceList = append(o.resourceList, &resource{name, color, added})
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

func (o *ControllerObj) Watch(controller cache.Controller) {
	log.Println("Starting the Blinkt controller")
	stopCh := make(chan struct{}, 1)
	defer close(stopCh)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigs
		stopCh <- struct{}{}
	}()
	go controller.Run(stopCh)
	<-stopCh
}

func (o *ControllerObj) Close() {
	log.Println("Stopping the Blinkt controller")
	o.blinkt.Close(blinkt.Red, o.brightness)
}

func (o *ControllerObj) getResource(name string) *resource {
	for _, r := range o.resourceList {
		if r.name == name {
			return r
		}
	}
	return nil
}

func (o *ControllerObj) updateBlinkt() {
	i := 0
	for ; i < len(o.resourceList); i++ {
		r := o.resourceList[i]
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
