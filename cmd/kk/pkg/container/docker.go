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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/container/templates"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/connector"
	"github.com/kubesys/kubekey/cmd/kk/pkg/files"
	"github.com/kubesys/kubekey/cmd/kk/pkg/registry"
	"github.com/kubesys/kubekey/cmd/kk/pkg/utils"
	"github.com/pkg/errors"
)

type SyncDockerBuildxPluginBinaries struct {
	common.KubeAction
}

func (s *SyncDockerBuildxPluginBinaries) Execute(runtime connector.Runtime) error {
	if err := utils.ResetTmpDir(runtime); err != nil {
		return err
	}

	binariesMapObj, ok := s.PipelineCache.Get(common.KubeBinaries + "-" + runtime.RemoteHost().GetArch())
	if !ok {
		return errors.New("get KubeBinary by pipeline cache failed")
	}
	binariesMap := binariesMapObj.(map[string]*files.KubeBinary)

	buildx, ok := binariesMap[common.Buildx]
	if !ok {
		return errors.New("get KubeBinary key buildx by pipeline cache failed")
	}

	dst := filepath.Join(common.TmpDir, buildx.FileName)
	if err := runtime.GetRunner().Scp(buildx.Path(), dst); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("sync docker binaries failed"))
	}

	if _, err := runtime.GetRunner().SudoCmd(
		fmt.Sprintf("mkdir -p /usr/local/lib/docker/cli-plugins && mv %s /usr/local/lib/docker/cli-plugins/docker-buildx && chmod +x /usr/local/lib/docker/cli-plugins/docker-buildx && rm -rf %s", dst, dst),
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("install docker-buildx binaries failed"))
	}
	return nil
}

type SyncDockerBinaries struct {
	common.KubeAction
}

func (s *SyncDockerBinaries) Execute(runtime connector.Runtime) error {
	if err := utils.ResetTmpDir(runtime); err != nil {
		return err
	}

	binariesMapObj, ok := s.PipelineCache.Get(common.KubeBinaries + "-" + runtime.RemoteHost().GetArch())
	if !ok {
		return errors.New("get KubeBinary by pipeline cache failed")
	}
	binariesMap := binariesMapObj.(map[string]*files.KubeBinary)

	docker, ok := binariesMap[common.Docker]
	if !ok {
		return errors.New("get KubeBinary key docker by pipeline cache failed")
	}

	dst := filepath.Join(common.TmpDir, docker.FileName)
	if err := runtime.GetRunner().Scp(docker.Path(), dst); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("sync docker binaries failed"))
	}

	if _, err := runtime.GetRunner().SudoCmd(
		fmt.Sprintf("mkdir -p /usr/bin && tar -zxf %s && mv docker/* /usr/bin && rm -rf docker", dst),
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("install container runtime docker binaries failed"))
	}
	return nil
}

type SyncCriDockerdBinaries struct {
	common.KubeAction
}

func (s *SyncCriDockerdBinaries) Execute(runtime connector.Runtime) error {
	if err := utils.ResetTmpDir(runtime); err != nil {
		return err
	}

	binariesMapObj, ok := s.PipelineCache.Get(common.KubeBinaries + "-" + runtime.RemoteHost().GetArch())
	if !ok {
		return errors.New("get KubeBinary by pipeline cache failed")
	}
	binariesMap := binariesMapObj.(map[string]*files.KubeBinary)

	criDockerd, ok := binariesMap[common.CriDockerd]
	if !ok {
		return errors.New("get KubeBinary key cri-dockerd by pipeline cache failed")
	}

	dst := filepath.Join(common.TmpDir, criDockerd.FileName)
	if err := runtime.GetRunner().Scp(criDockerd.Path(), dst); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("sync cri-dockerd binaries failed"))
	}

	if _, err := runtime.GetRunner().SudoCmd(
		fmt.Sprintf("mkdir -p /usr/bin && tar -zxf %s && mv cri-dockerd/* /usr/bin && rm -rf cri-dockerd", dst),
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("install container runtime cri-dockerd binaries failed"))
	}
	return nil
}

type EnableContainerdForDocker struct {
	common.KubeAction
}

func (e *EnableContainerdForDocker) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd(
		"systemctl daemon-reload && systemctl enable containerd &&  systemctl start containerd",
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("enable and start containerd failed"))
	}
	return nil
}

type EnableDocker struct {
	common.KubeAction
}

func (e *EnableDocker) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd(
		"systemctl daemon-reload && systemctl enable docker && systemctl start docker",
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("enable and start docker failed"))
	}
	return nil
}

type EnableCriDockerd struct {
	common.KubeAction
}

func (e *EnableCriDockerd) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd(
		"systemctl daemon-reload && systemctl enable cri-docker && systemctl start cri-docker",
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("enable and start cri-docker failed"))
	}
	return nil
}

type DockerLoginRegistry struct {
	common.KubeAction
}

func (p *DockerLoginRegistry) Execute(runtime connector.Runtime) error {

	auths := registry.DockerRegistryAuthEntries(p.KubeConf.Cluster.Registry.Auths)

	for repo, entry := range auths {
		if len(entry.Username) == 0 || len(entry.Password) == 0 {
			continue
		}
		cmd := fmt.Sprintf("HOME=$HOME docker login --username '%s' --password '%s' %s", escapeSpecialCharacters(entry.Username), escapeSpecialCharacters(entry.Password), repo)
		if _, err := runtime.GetRunner().SudoCmd(cmd, false); err != nil {
			return errors.Wrapf(err, "login registry failed, cmd: %v, err:%v", cmd, err)
		}
	}

	if output, err := runtime.GetRunner().SudoCmd(
		"if [ -e $HOME/.docker/config.json ]; "+
			"then echo 'exist'; "+
			"fi", false); err == nil && strings.Contains(output, "exist") {

		cmd := "mkdir -p /.docker && cp -f $HOME/.docker/config.json /.docker/ && chmod 0644 /.docker/config.json "
		if _, err := runtime.GetRunner().SudoCmd(cmd, false); err != nil {
			return errors.Wrapf(err, "copy docker auths failed cmd: %v, err:%v", cmd, err)
		}
	}

	return nil
}

type DisableDocker struct {
	common.KubeAction
}

func (d *DisableDocker) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("systemctl disable docker && systemctl stop docker",
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), fmt.Sprintf("disable and stop docker failed"))
	}

	// remove docker related files
	files := []string{
		"/usr/bin/runc",
		"/usr/bin/ctr",
		"/usr/bin/docker*",
		"/usr/bin/containerd-shim-runc-v2",
		filepath.Join("/etc/systemd/system", templates.DockerService.Name()),
		filepath.Join("/etc/docker", templates.DockerConfig.Name()),
		templates.DockerDataDir(d.KubeConf),
	}

	if d.KubeConf.Cluster.Kubernetes.IsAtLeastV124() && d.KubeConf.Cluster.Kubernetes.ContainerManager == common.Docker {
		if _, err := runtime.GetRunner().SudoCmd("systemctl disable cri-docker && systemctl stop cri-docker",
			false); err != nil {
			return errors.Wrap(errors.WithStack(err), fmt.Sprintf("disable and stop cri-docker failed"))
		}
		files = append(files, filepath.Join("/etc/systemd/system", templates.CriDockerService.Name()))
		files = append(files, "/var/run/cri-dockerd.sock")
	}

	for _, file := range files {
		_, _ = runtime.GetRunner().SudoCmd(fmt.Sprintf("rm -rf %s", file), true)
	}
	return nil
}

func escapeSpecialCharacters(str string) string {
	newStr := strings.ReplaceAll(str, "$", "\\$")
	newStr = strings.ReplaceAll(newStr, "&", "\\&")
	newStr = strings.ReplaceAll(newStr, "*", "\\*")
	return newStr
}
