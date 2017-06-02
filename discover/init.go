package discover

import (
	"github.com/bradfitz/gomemcache/memcache"
	"k8s.io/client-go/kubernetes"
	lib "github.com/k8guard/k8guardlibs"
	"github.com/k8guard/k8guardlibs/k8s"
	"fmt"
	"github.com/k8guard/k8guardlibs/messaging/kafka"

)


// Clientset talks to kubernetes API
var Clientset kubernetes.Clientset
var Memcached *memcache.Client
var KafkaProducer kafka.KafkaProducer


var err error
var KafkaProducerError error

func init() {
	Clientset, err = k8s.LoadClientset()
	Memcached = memcache.New(fmt.Sprintf("%s:11211", lib.Cfg.MemCachedHostname))

	KafkaProducer, KafkaProducerError = kafka.NewProducer(kafka.DISCOVER_CLIENTID, lib.Cfg)

	// defer KafkaProducer.Close()


}

