package discover

import (
	"encoding/json"
	"io/ioutil"
	"strings"

	"github.com/k8guard/k8guard-discover/metrics"
	lib "github.com/k8guard/k8guardlibs"
	"github.com/k8guard/k8guardlibs/messaging/kafka"
	"github.com/k8guard/k8guardlibs/violations"
	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/apps/v1beta1"
)

func GetAllDeployFromApi() []v1beta1.Deployment {
	deploys, err := Clientset.AppsV1beta1().Deployments(lib.Cfg.Namespace).List(metav1.ListOptions{})
	if err != nil {
		lib.Log.Error("error: ", err)
		panic(err.Error())
	}

	if lib.Cfg.OutputPodsToFile == true {
		r, _ := json.Marshal(deploys.Items)
		err = ioutil.WriteFile("deployments.txt", r, 0644)
		if err != nil {
			lib.Log.Error("error:", err)
			panic(err)
		}
	}
	metrics.Update(metrics.ALL_DEPLOYMENT_COUNT, len(deploys.Items))
	return deploys.Items
}

func GetBadDeploys(theDeploys []v1beta1.Deployment, sendToKafka bool) []lib.Deployment {
	timer := prometheus.NewTimer(prometheus.ObserverFunc(metrics.FNGetBadDeploys.Set))
	defer timer.ObserveDuration()

	allBadDeploys := []lib.Deployment{}

	cacheAllImages(true)

	for _, kd := range theDeploys {
		if isIgnoredNamespace(kd.Namespace) == true || isIgnoredDeployment(kd.ObjectMeta.Name) == true {
			continue
		}
		if kd.Status.Replicas == 0 {
			continue
		}
		d := lib.Deployment{}
		d.Name = kd.Name
		d.Cluster = lib.Cfg.ClusterName
		d.Namespace = kd.Namespace
		getVolumesWithHostPathForAPod(kd.Name, kd.Spec.Template.Spec, &d.ViolatableEntity)

		verifyRequiredAnnotations(kd.ObjectMeta, &d.ViolatableEntity, violations.REQUIRED_DEPLOYMENT_ANNOTATIONS_TYPE, lib.Cfg.RequiredDeploymentAnnotations)
		verifyRequiredLabels(kd.ObjectMeta, &d.ViolatableEntity, violations.REQUIRED_DEPLOYMENT_LABELS_TYPE, lib.Cfg.RequiredDeploymentLabels)

		verifyRequiredAnnotations(kd.Spec.Template.ObjectMeta, &d.ViolatableEntity, violations.REQUIRED_POD_ANNOTATIONS_TYPE, lib.Cfg.RequiredPodAnnotations)
		verifyRequiredLabels(kd.Spec.Template.ObjectMeta, &d.ViolatableEntity, violations.REQUIRED_POD_LABELS_TYPE, lib.Cfg.RequiredPodLabels)

		GetBadContainers(kd.Name, kd.Spec.Template.Spec, &d.ViolatableEntity)
		if isValidReplicaSize(kd) == false && isNotIgnoredViolation(kd.Name, violations.SINGLE_REPLICA_TYPE) {
			d.Violations = append(d.Violations, violations.Violation{Source: kd.Name, Type: violations.SINGLE_REPLICA_TYPE})
		}

		if len(d.ViolatableEntity.Violations) > 0 {
			allBadDeploys = append(allBadDeploys, d)
			if sendToKafka {
				lib.Log.Debug("Sending ", d.Name, " to kafka")
				err := KafkaProducer.SendData(lib.Cfg.KafkaActionTopic, kafka.DEPLOYMENT_MESSAGE, d)
				if err != nil {
					panic(err)
				}
			}
		}

	}
	metrics.Update(metrics.BAD_DEPLOYMENT_COUNT, len(allBadDeploys))
	return allBadDeploys
}

func isValidReplicaSize(deployment v1beta1.Deployment) bool {
	if *deployment.Spec.Replicas == 1 {
		return false
	}
	return true
}

func isIgnoredDeployment(deploymentName string) bool {
	for _, d := range lib.Cfg.IgnoredDeployments {
		if strings.HasPrefix(deploymentName, d) == true {
			return true
		}
	}
	return false
}
