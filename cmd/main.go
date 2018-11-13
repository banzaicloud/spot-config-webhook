// Copyright © 2018 Banzai Cloud
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

package main

import (
	"github.com/banzaicloud/spot-config-webhook/pkg"
	"github.com/openshift/generic-admission-server/pkg/cmd"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {

	initConfig()

	log.SetLevel(log.DebugLevel)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Error(err)
	}

	cmd.RunAdmissionServer(pkg.NewAdmissionHook(
		clientset,
		viper.GetString(pkg.SpotAnnotationKey),
		viper.GetString(pkg.SpotApiResourceGroup),
		viper.GetString(pkg.SpotApiResourceVersion),
		viper.GetString(pkg.SpotApiResourceName),
		viper.GetString(pkg.SpotConfigMapNamespace),
		viper.GetString(pkg.SpotConfigMapName),
		viper.GetString(pkg.SpotSchedulerName),
	))
}
func initConfig() {
	viper.AutomaticEnv()
	viper.SetDefault(pkg.SpotAnnotationKey, "admission.banzaicloud.com")
	viper.SetDefault(pkg.SpotApiResourceGroup, "app.banzaicloud.io/odPercentage")
	viper.SetDefault(pkg.SpotApiResourceVersion, "v1beta1")
	viper.SetDefault(pkg.SpotApiResourceName, "spotscheduling")
	viper.SetDefault(pkg.SpotConfigMapNamespace, "pipeline-system")
	viper.SetDefault(pkg.SpotConfigMapName, "spot-deploy-config")
	viper.SetDefault(pkg.SpotSchedulerName, "spot-scheduler")
}
