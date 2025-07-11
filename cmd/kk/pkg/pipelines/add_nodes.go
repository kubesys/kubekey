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
	"fmt"

	kubekeyapiv1alpha2 "github.com/kubesys/kubekey/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesys/kubekey/cmd/kk/pkg/artifact"
	"github.com/kubesys/kubekey/cmd/kk/pkg/binaries"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/confirm"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/customscripts"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/os"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/registry"
	"github.com/kubesys/kubekey/cmd/kk/pkg/certs"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/container"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/module"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/pipeline"
	"github.com/kubesys/kubekey/cmd/kk/pkg/etcd"
	"github.com/kubesys/kubekey/cmd/kk/pkg/filesystem"
	"github.com/kubesys/kubekey/cmd/kk/pkg/images"
	"github.com/kubesys/kubekey/cmd/kk/pkg/k3s"
	"github.com/kubesys/kubekey/cmd/kk/pkg/k8e"
	"github.com/kubesys/kubekey/cmd/kk/pkg/kubernetes"
	"github.com/kubesys/kubekey/cmd/kk/pkg/loadbalancer"
)

func NewAddNodesPipeline(runtime *common.KubeRuntime) error {
	noArtifact := runtime.Arg.Artifact == ""

	m := []module.Module{
		&precheck.GreetingsModule{},
		&customscripts.CustomScriptsModule{Phase: "PreInstall", Scripts: runtime.Cluster.System.PreInstall},
		&precheck.NodePreCheckModule{},
		&confirm.InstallConfirmModule{},
		&artifact.UnArchiveModule{Skip: noArtifact},
		&os.RepositoryModule{Skip: noArtifact || !runtime.Arg.InstallPackages},
		&binaries.NodeBinariesModule{},
		&os.ConfigureOSModule{Skip: runtime.Cluster.System.SkipConfigureOS},
		&registry.RegistryCertsModule{Skip: len(runtime.GetHostsByRole(common.Registry)) == 0},
		//for one master to multi master kube-vip
		&loadbalancer.KubevipModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabledVip()},
		&kubernetes.RestartKubeletModule{},
		&kubernetes.StatusModule{},
		&container.InstallContainerModule{},
		&container.InstallCriDockerdModule{Skip: runtime.Cluster.Kubernetes.ContainerManager != "docker"},
		&images.PullModule{Skip: runtime.Arg.SkipPullImages},
		&etcd.PreCheckModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.CertsModule{Skip: runtime.Arg.SkipEtcd},
		&etcd.InstallETCDBinaryModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.ConfigureModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.BackupModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.Arg.SkipEtcd},
		&kubernetes.InstallKubeBinariesModule{},
		&kubernetes.JoinNodesModule{},
		&loadbalancer.HaproxyModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabled()},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !runtime.Cluster.Kubernetes.EnableAutoRenewCerts()},
		&customscripts.CustomScriptsModule{Phase: "PostInstall", Scripts: runtime.Cluster.System.PostInstall},
	}

	p := pipeline.Pipeline{
		Name:    "AddNodesPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}

	return nil
}

func NewK3sAddNodesPipeline(runtime *common.KubeRuntime) error {
	noArtifact := runtime.Arg.Artifact == ""

	m := []module.Module{
		&precheck.GreetingsModule{},
		&artifact.UnArchiveModule{Skip: noArtifact},
		&os.RepositoryModule{Skip: noArtifact || !runtime.Arg.InstallPackages},
		&binaries.K3sNodeBinariesModule{},
		&os.ConfigureOSModule{Skip: runtime.Cluster.System.SkipConfigureOS},
		&customscripts.CustomScriptsModule{Phase: "PreInstall", Scripts: runtime.Cluster.System.PreInstall},
		&k3s.StatusModule{},
		&etcd.PreCheckModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.CertsModule{Skip: runtime.Arg.SkipEtcd},
		&etcd.InstallETCDBinaryModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.ConfigureModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.BackupModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.Arg.SkipEtcd},
		&k3s.InstallKubeBinariesModule{},
		&k3s.JoinNodesModule{},
		&loadbalancer.K3sHaproxyModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabled()},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !runtime.Cluster.Kubernetes.EnableAutoRenewCerts()},
		&customscripts.CustomScriptsModule{Phase: "PostInstall", Scripts: runtime.Cluster.System.PostInstall},
	}

	p := pipeline.Pipeline{
		Name:    "AddNodesPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}

	return nil
}

func NewK8eAddNodesPipeline(runtime *common.KubeRuntime) error {
	noArtifact := runtime.Arg.Artifact == ""

	m := []module.Module{
		&precheck.GreetingsModule{},
		&artifact.UnArchiveModule{Skip: noArtifact},
		&os.RepositoryModule{Skip: noArtifact || !runtime.Arg.InstallPackages},
		&binaries.K8eNodeBinariesModule{},
		&os.ConfigureOSModule{Skip: runtime.Cluster.System.SkipConfigureOS},

		&k8e.StatusModule{},
		&etcd.PreCheckModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.CertsModule{Skip: runtime.Arg.SkipEtcd},
		&etcd.InstallETCDBinaryModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.ConfigureModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey},
		&etcd.BackupModule{Skip: runtime.Cluster.Etcd.Type != kubekeyapiv1alpha2.KubeKey || runtime.Arg.SkipEtcd},
		&k8e.InstallKubeBinariesModule{},
		&k8e.JoinNodesModule{},
		&loadbalancer.K3sHaproxyModule{Skip: !runtime.Cluster.ControlPlaneEndpoint.IsInternalLBEnabled()},
		&kubernetes.ConfigureKubernetesModule{},
		&filesystem.ChownModule{},
		&certs.AutoRenewCertsModule{Skip: !runtime.Cluster.Kubernetes.EnableAutoRenewCerts()},
	}

	p := pipeline.Pipeline{
		Name:    "AddNodesPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}

	return nil
}

func AddNodes(args common.Argument, downloadCmd string) error {
	args.DownloadCommand = func(path, url string) string {
		// this is an extension point for downloading tools, for example users can set the timeout, proxy or retry under
		// some poor network environment. Or users even can choose another cli, it might be wget.
		// perhaps we should have a build-in download function instead of totally rely on the external one
		return fmt.Sprintf(downloadCmd, path, url)
	}

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

	switch runtime.Cluster.Kubernetes.Type {
	case common.K3s:
		if err := NewK3sAddNodesPipeline(runtime); err != nil {
			return err
		}
	case common.K8e:
		if err := NewK8eAddNodesPipeline(runtime); err != nil {
			return err
		}
	case common.Kubernetes:
		fallthrough
	default:
		if err := NewAddNodesPipeline(runtime); err != nil {
			return err
		}
	}
	return nil
}
