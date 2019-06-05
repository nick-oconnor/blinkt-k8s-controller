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

package helpers

import (
	"fmt"
	"log"
	"math"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

func NewClients() (*kubernetes.Clientset, *metrics.Clientset) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panicln(err.Error())
	}
	kubernetesClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Panicln(err.Error())
	}
	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		log.Panicln(err.Error())
	}
	return kubernetesClient, metricsClient
}

func RatioToColor(target, actual int64) string {
	ratio := math.Min(2, 2*float64(actual)/float64(target))
	b := int(math.Max(0, 255*(1-ratio)))
	r := int(math.Max(0, 255*(ratio-1)))
	g := 255 - b - r
	return fmt.Sprintf("%02X%02X%02X", r, g, b)
}
