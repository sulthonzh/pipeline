package monitor

import (
	"fmt"

	"github.com/banzaicloud/pipeline/cluster"
	"github.com/banzaicloud/pipeline/config"
	"github.com/banzaicloud/pipeline/model"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var logger *logrus.Logger
var log *logrus.Entry

// Simple init for logging
func init() {
	logger = config.Logger()
	log = logger.WithFields(logrus.Fields{"tag": "monitor"})
	viper.SetDefault("monitor.release", "pipeline")
	viper.SetDefault("monitor.enabled", false)
}

type prometheusTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

//PrometheusCfg
type PrometheusCfg struct {
	Endpoint string
	Name     string
}

//UpdatePrometheusConfig updates the Prometheus configuration
func UpdatePrometheusConfig() error {
	log := logger.WithFields(logrus.Fields{"tag": "PrometheusConfig"})
	//TODO configsets
	if !viper.GetBool("monitor.enabled") {
		log.Warn("Update monitoring confiouration is disabled")
		return nil
	}

	//TODO move to configuration or sg like this
	prometheusConfigMap := "prometheus-server"
	releaseName := viper.GetString("monitor.release")
	log.Debugf("Prometheus relelase name: %s", releaseName)
	log.Debugf("Prometheus Config map  name: %s", prometheusConfigMap)
	prometheusConfigMapName := releaseName + "-" + prometheusConfigMap
	log.Debugf("Prometheus Config map full name: %s", prometheusConfigMapName)

	var clusters []cluster.CommonCluster
	db := model.GetDB()
	db.Find(&clusters)
	var prometheusConfig []PrometheusCfg
	//Gathering information about clusters
	for _, cluster := range clusters {

		kubeEndpoint, err := cluster.GetAPIEndpoint()
		if err != nil {
			log.Errorf("Cluster endpoint not doinf for cluster: %s", cluster.GetName())
		}

		log.Debugf("Cluster Endpoint IP: %s", kubeEndpoint)

		prometheusConfig = append(
			prometheusConfig,
			PrometheusCfg{
				Endpoint: kubeEndpoint,
				Name:     cluster.GetName(),
			})

	}
	prometheusConfigRaw := GenerateConfig(prometheusConfig)

	log.Info("Kubernetes in-cluster configuration.")
	config, err := rest.InClusterConfig()
	if err != nil {
		return errors.Wrap(err, "can't use kubernetes in-cluster config")
	}
	client := kubernetes.NewForConfigOrDie(config)

	//TODO configurable namespace and service
	configmap, err := client.CoreV1().ConfigMaps("default").Get(prometheusConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting kubernetes confgimap failed: %s", err)
	}
	log.Info("Updating configmap")
	configmap.Data["prometheus.yml"] = string(prometheusConfigRaw)
	client.CoreV1().ConfigMaps("default").Update(configmap)
	log.Info("Update configmap finished")

	return nil
}
