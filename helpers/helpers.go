package helpers

import (
	"fmt"
	"log"
	"math"

	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

func NewCoreClient() *internalclientset.Clientset {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Panicln(err.Error())
	}
	clientset, err := internalclientset.NewForConfig(config)
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
