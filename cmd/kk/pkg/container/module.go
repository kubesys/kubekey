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

package container

import (
	"path/filepath"
	"strings"

	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/container/templates"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/action"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/logger"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/prepare"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/task"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/util"
	"github.com/kubesys/kubekey/cmd/kk/pkg/images"
	"github.com/kubesys/kubekey/cmd/kk/pkg/kubernetes"
	"github.com/kubesys/kubekey/cmd/kk/pkg/registry"
)

type InstallContainerModule struct {
	common.KubeModule
	Skip bool
}

func (i *InstallContainerModule) IsSkip() bool {
	return i.Skip
}

func (i *InstallContainerModule) Init() {
	i.Name = "InstallContainerModule"
	i.Desc = "Install container manager"

	switch i.KubeConf.Cluster.Kubernetes.ContainerManager {
	case common.Docker:
		i.Tasks = InstallDocker(i)
	case common.Containerd:
		i.Tasks = InstallContainerd(i)
	case common.Crio:
		// TODO: Add the steps of cri-o's installation.
	case common.Isula:
		// TODO: Add the steps of iSula's installation.
	default:
		logger.Log.Fatalf("Unsupported container runtime: %s", strings.TrimSpace(i.KubeConf.Cluster.Kubernetes.ContainerManager))
	}
}

func InstallDocker(m *InstallContainerModule) []task.Interface {

	syncBuildxPluginBinaries := &task.RemoteTask{
		Name:  "SyncDockerBuildxBinaries",
		Desc:  "Sync docker buildx binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&WithBuildxPlugin{},
		},
		Action:   new(SyncDockerBuildxPluginBinaries),
		Parallel: true,
		Retry:    2,
	}

	syncBinaries := &task.RemoteTask{
		Name:  "SyncDockerBinaries",
		Desc:  "Sync docker binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&DockerExist{Not: true},
		},
		Action:   new(SyncDockerBinaries),
		Parallel: true,
		Retry:    2,
	}

	generateContainerdService := &task.RemoteTask{
		Name:  "GenerateContainerdService",
		Desc:  "Generate containerd service",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action: &action.Template{
			Template: templates.ContainerdService,
			Dst:      filepath.Join("/etc/systemd/system", templates.ContainerdService.Name()),
		},
		Parallel: true,
	}

	generateDockerService := &task.RemoteTask{
		Name:  "GenerateDockerService",
		Desc:  "Generate docker service",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&DockerExist{Not: true},
		},
		Action: &action.Template{
			Template: templates.DockerService,
			Dst:      filepath.Join("/etc/systemd/system", templates.DockerService.Name()),
		},
		Parallel: true,
	}

	generateDockerConfig := &task.RemoteTask{
		Name:  "GenerateDockerConfig",
		Desc:  "Generate docker config",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&DockerExist{Not: true},
		},
		Action: &action.Template{
			Template: templates.DockerConfig,
			Dst:      filepath.Join("/etc/docker/", templates.DockerConfig.Name()),
			Data: util.Data{
				"Mirrors":            templates.Mirrors(m.KubeConf),
				"InsecureRegistries": templates.InsecureRegistries(m.KubeConf),
				"DataRoot":           templates.DockerDataDir(m.KubeConf),
				"BridgeIP":           templates.BridgeIP(m.KubeConf),
			},
		},
		Parallel: true,
	}

	enableContainerdForDocker := &task.RemoteTask{
		Name:  "EnableContainerd",
		Desc:  "Enable containerd",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action:   new(EnableContainerdForDocker),
		Parallel: true,
	}

	enableDocker := &task.RemoteTask{
		Name:  "EnableDocker",
		Desc:  "Enable docker",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&DockerExist{Not: true},
		},
		Action:   new(EnableDocker),
		Parallel: true,
	}

	dockerLoginRegistry := &task.RemoteTask{
		Name:  "Login PrivateRegistry",
		Desc:  "Add auths to container runtime",
		Hosts: m.Runtime.GetAllHosts(),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&DockerExist{},
			&PrivateRegistryAuth{},
		},
		Action:   new(DockerLoginRegistry),
		Parallel: true,
	}

	return []task.Interface{
		syncBinaries,
		generateContainerdService,
		generateDockerService,
		generateDockerConfig,
		enableContainerdForDocker,
		enableDocker,
		dockerLoginRegistry,
		syncBuildxPluginBinaries,
	}
}

func InstallContainerd(m *InstallContainerModule) []task.Interface {
	syncContainerd := &task.RemoteTask{
		Name:  "SyncContainerd",
		Desc:  "Sync containerd binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action:   new(SyncContainerd),
		Parallel: true,
		Retry:    2,
	}

	generateContainerdService := &task.RemoteTask{
		Name:  "GenerateContainerdService",
		Desc:  "Generate containerd service",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action: &action.Template{
			Template: templates.ContainerdService,
			Dst:      filepath.Join("/etc/systemd/system", templates.ContainerdService.Name()),
		},
		Parallel: true,
	}

	generateContainerdConfig := &task.RemoteTask{
		Name:  "GenerateContainerdConfig",
		Desc:  "Generate containerd config",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action: &action.Template{
			Template: templates.ContainerdConfig,
			Dst:      filepath.Join("/etc/containerd/", templates.ContainerdConfig.Name()),
			Data: util.Data{
				"Mirrors":            templates.Mirrors(m.KubeConf),
				"InsecureRegistries": m.KubeConf.Cluster.Registry.InsecureRegistries,
				"SandBoxImage":       images.GetImage(m.Runtime, m.KubeConf, "pause").ImageName(),
				"Auths":              registry.DockerRegistryAuthEntries(m.KubeConf.Cluster.Registry.Auths),
				"DataRoot":           templates.ContainerdDataDir(m.KubeConf),
				"NvidiaRuntime":      m.KubeConf.Cluster.Kubernetes.EnableNvidiaRuntime(),
				"HasRemoteMirrors":   len(m.KubeConf.Cluster.Registry.RemoteMirrors) > 0,
			},
		},
		Parallel: true,
	}

	enableContainerd := &task.RemoteTask{
		Name:  "EnableContainerd",
		Desc:  "Enable containerd",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action:   new(EnableContainerd),
		Parallel: true,
	}

	syncCrictlBinaries := &task.RemoteTask{
		Name:  "SyncCrictlBinaries",
		Desc:  "Sync crictl binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&CrictlExist{Not: true},
		},
		Action:   new(SyncCrictlBinaries),
		Parallel: true,
		Retry:    2,
	}

	generateCrictlConfig := &task.RemoteTask{
		Name:  "GenerateCrictlConfig",
		Desc:  "Generate crictl config",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&CrictlExist{Not: false},
		},
		Action: &action.Template{
			Template: templates.CrictlConfig,
			Dst:      filepath.Join("/etc/", templates.CrictlConfig.Name()),
			Data: util.Data{
				"Endpoint": m.KubeConf.Cluster.Kubernetes.ContainerRuntimeEndpoint,
			},
		},
		Parallel: true,
	}

	// Add a task to generate remote mirror proxy configuration
	generateContainerdMirrorConfig := &task.RemoteTask{
		Name:  "GenerateContainerdMirrorConfig",
		Desc:  "Generate containerd mirror config",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&kubernetes.NodeInCluster{Not: true},
			&ContainerdExist{Not: true},
		},
		Action:   new(GenerateContainerdMirrorConfig),
		Parallel: true,
	}

	tasks := []task.Interface{
		syncContainerd,
		generateContainerdService,
		generateContainerdConfig,
		enableContainerd,
		syncCrictlBinaries,
		generateCrictlConfig,
	}

	// Only add this task when remote mirror proxies are configured
	if len(m.KubeConf.Cluster.Registry.RemoteMirrors) > 0 {
		tasks = append(tasks, generateContainerdMirrorConfig)
	}

	return tasks
}

type InstallCriDockerdModule struct {
	common.KubeModule
	Skip bool
}

func (m *InstallCriDockerdModule) IsSkip() bool {
	return m.Skip
}

func (m *InstallCriDockerdModule) Init() {
	m.Name = "InstallCriDockerdModule"
	m.Desc = "Install cri-dockerd"

	syncCriDockerdBinaries := &task.RemoteTask{
		Name:  "SyncCriDockerdBinaries",
		Desc:  "Sync cri-dockerd binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&CriDockerdExist{Not: true},
			&common.AtLeastV124{},
		},
		Action:   new(SyncCriDockerdBinaries),
		Parallel: true,
		Retry:    2,
	}

	generateCriDockerdService := &task.RemoteTask{
		Name:  "GenerateCriDockerdService",
		Desc:  "Generate cri-dockerd service",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&CriDockerdExist{Not: true},
			&common.AtLeastV124{},
		},
		Action: &action.Template{
			Template: templates.CriDockerService,
			Dst:      filepath.Join("/etc/systemd/system", templates.CriDockerService.Name()),
			Data: util.Data{
				"SandBoxImage": images.GetImage(m.Runtime, m.KubeConf, "pause").ImageName(),
			},
		},
		Parallel: true,
	}

	enableCriDockerd := &task.RemoteTask{
		Name:  "EnableCriDockerd",
		Desc:  "Enable cri-dockerd",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&CriDockerdExist{Not: true},
			&common.AtLeastV124{},
		},
		Action:   new(EnableCriDockerd),
		Parallel: true,
	}

	syncCrictlBinaries := &task.RemoteTask{
		Name:  "SyncCrictlBinaries",
		Desc:  "Sync crictl binaries",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&CrictlExist{Not: true},
			&common.AtLeastV124{},
		},
		Action:   new(SyncCrictlBinaries),
		Parallel: true,
		Retry:    2,
	}

	generateCrictlConfig := &task.RemoteTask{
		Name:  "GenerateCrictlConfig",
		Desc:  "Generate crictl config",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&common.AtLeastV124{},
		},
		Action: &action.Template{
			Template: templates.CrictlConfig,
			Dst:      filepath.Join("/etc/", templates.CrictlConfig.Name()),
			Data: util.Data{
				"Endpoint": m.KubeConf.Cluster.Kubernetes.ContainerRuntimeEndpoint,
			},
		},
		Parallel: true,
	}

	m.Tasks = []task.Interface{
		syncCriDockerdBinaries,
		generateCriDockerdService,
		enableCriDockerd,
		syncCrictlBinaries,
		generateCrictlConfig,
	}
}

type UninstallContainerModule struct {
	common.KubeModule
	Skip bool
}

func (i *UninstallContainerModule) IsSkip() bool {
	return i.Skip
}

func (i *UninstallContainerModule) Init() {
	i.Name = "UninstallContainerModule"
	i.Desc = "Uninstall container manager"

	switch i.KubeConf.Cluster.Kubernetes.ContainerManager {
	case common.Docker:
		i.Tasks = UninstallDocker(i)
	case common.Containerd:
		i.Tasks = UninstallContainerd(i)
	case common.Crio:
		// TODO: Add the steps of cri-o's installation.
	case common.Isula:
		// TODO: Add the steps of iSula's installation.
	default:
		logger.Log.Fatalf("Unsupported container runtime: %s", strings.TrimSpace(i.KubeConf.Cluster.Kubernetes.ContainerManager))
	}
}

func UninstallDocker(m *UninstallContainerModule) []task.Interface {

	disableDocker := &task.RemoteTask{
		Name:  "DisableDocker",
		Desc:  "Disable docker",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&DockerExist{Not: false},
		},
		Action:   new(DisableDocker),
		Parallel: true,
	}

	return []task.Interface{
		disableDocker,
	}
}

func UninstallContainerd(m *UninstallContainerModule) []task.Interface {
	disableContainerd := &task.RemoteTask{
		Name:  "UninstallContainerd",
		Desc:  "Uninstall containerd",
		Hosts: m.Runtime.GetHostsByRole(common.K8s),
		Prepare: &prepare.PrepareCollection{
			&ContainerdExist{Not: false},
		},
		Action:   new(DisableContainerd),
		Parallel: true,
	}

	return []task.Interface{
		disableContainerd,
	}
}

type CriMigrateModule struct {
	common.KubeModule

	Skip bool
}

func (i *CriMigrateModule) IsSkip() bool {
	return i.Skip
}

func (p *CriMigrateModule) Init() {
	p.Name = "CriMigrateModule"
	p.Desc = "Cri Migrate manager"

	if p.KubeConf.Arg.Role == common.Worker {
		p.Tasks = MigrateWCri(p)
	} else if p.KubeConf.Arg.Role == common.Master {
		p.Tasks = MigrateMCri(p)
	} else if p.KubeConf.Arg.Role == "all" {
		p.Tasks = MigrateACri(p)
	} else {
		logger.Log.Fatalf("Unsupported Role: %s", strings.TrimSpace(p.KubeConf.Arg.Role))
	}
}

func MigrateWCri(p *CriMigrateModule) []task.Interface {

	MigrateWCri := &task.RemoteTask{
		Name:     "MigrateToDocker",
		Desc:     "Migrate To Docker",
		Hosts:    p.Runtime.GetHostsByRole(common.Worker),
		Prepare:  new(common.OnlyWorker),
		Action:   new(MigrateSelfNodeCri),
		Parallel: false,
	}

	p.Tasks = []task.Interface{
		MigrateWCri,
	}

	return p.Tasks
}

func MigrateMCri(p *CriMigrateModule) []task.Interface {

	MigrateMCri := &task.RemoteTask{
		Name:     "MigrateMasterToDocker",
		Desc:     "Migrate Master To Docker",
		Hosts:    p.Runtime.GetHostsByRole(common.Master),
		Prepare:  new(common.IsMaster),
		Action:   new(MigrateSelfNodeCri),
		Parallel: false,
	}

	p.Tasks = []task.Interface{
		MigrateMCri,
	}

	return p.Tasks
}

func MigrateACri(p *CriMigrateModule) []task.Interface {

	MigrateACri := &task.RemoteTask{
		Name:     "MigrateMasterToDocker",
		Desc:     "Migrate Master To Docker",
		Hosts:    p.Runtime.GetHostsByRole(common.K8s),
		Action:   new(MigrateSelfNodeCri),
		Parallel: false,
	}

	p.Tasks = []task.Interface{
		MigrateACri,
	}

	return p.Tasks
}
