/*
 Copyright 2021 The KubeSphere Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package pipelines

import (
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesys/kubekey/cmd/kk/pkg/certs"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/module"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/pipeline"
)

func RenewCertsPipeline(runtime *common.KubeRuntime) error {
	m := []module.Module{
		&precheck.GreetingsModule{},
		&certs.RenewCertsModule{},
		&certs.CheckCertsModule{},
		&certs.PrintClusterCertsModule{},
	}

	p := pipeline.Pipeline{
		Name:    "RenewCertsPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}
	return nil
}

func RenewCerts(args common.Argument) error {
	var loaderType string
	if args.FilePath != "" {
		loaderType = common.File
	} else {
		loaderType = common.AllInOne
	}

	runtime, err := common.NewKubeRuntime(loaderType, args)
	if err != nil {
		return err
	}

	if err := RenewCertsPipeline(runtime); err != nil {
		return err
	}
	return nil
}
