/*
 Copyright 2022 The KubeSphere Authors.

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

package k8e

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	kubekeyapiv1alpha2 "github.com/kubesys/kubekey/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/action"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/connector"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/util"
	"github.com/kubesys/kubekey/cmd/kk/pkg/files"
	"github.com/kubesys/kubekey/cmd/kk/pkg/images"
	"github.com/kubesys/kubekey/cmd/kk/pkg/k8e/templates"
	"github.com/kubesys/kubekey/cmd/kk/pkg/utils"
)

type GetClusterStatus struct {
	common.KubeAction
}

func (g *GetClusterStatus) Execute(runtime connector.Runtime) error {
	exist, err := runtime.GetRunner().FileExist("/etc/systemd/system/k8e.service")
	if err != nil {
		return err
	}

	if !exist {
		g.PipelineCache.Set(common.ClusterExist, false)
		return nil
	} else {
		g.PipelineCache.Set(common.ClusterExist, true)

		if v, ok := g.PipelineCache.Get(common.ClusterStatus); ok {
			cluster := v.(*K8eStatus)
			if err := cluster.SearchVersion(runtime); err != nil {
				return err
			}
			if err := cluster.SearchKubeConfig(runtime); err != nil {
				return err
			}
			if err := cluster.LoadKubeConfig(runtime, g.KubeConf); err != nil {
				return err
			}
			if err := cluster.SearchNodeToken(runtime); err != nil {
				return err
			}
			if err := cluster.SearchInfo(runtime); err != nil {
				return err
			}
			if err := cluster.SearchNodesInfo(runtime); err != nil {
				return err
			}
			g.PipelineCache.Set(common.ClusterStatus, cluster)
		} else {
			return errors.New("get k8e cluster status by pipeline cache failed")
		}
	}
	return nil
}

type SyncKubeBinary struct {
	common.KubeAction
}

func (s *SyncKubeBinary) Execute(runtime connector.Runtime) error {
	binariesMapObj, ok := s.PipelineCache.Get(common.KubeBinaries + "-" + runtime.RemoteHost().GetArch())
	if !ok {
		return errors.New("get KubeBinary by pipeline cache failed")
	}
	binariesMap := binariesMapObj.(map[string]*files.KubeBinary)

	if err := SyncKubeBinaries(runtime, binariesMap); err != nil {
		return err
	}
	return nil
}

// SyncKubeBinaries is used to sync kubernetes' binaries to each node.
func SyncKubeBinaries(runtime connector.Runtime, binariesMap map[string]*files.KubeBinary) error {
	if err := utils.ResetTmpDir(runtime); err != nil {
		return err
	}

	binaryList := []string{"k8e", "helm", "kubecni"}
	for _, name := range binaryList {
		binary, ok := binariesMap[name]
		if !ok {
			return fmt.Errorf("get kube binary %s info failed: no such key", name)
		}

		fileName := binary.FileName
		switch name {
		case "kubecni":
			dst := filepath.Join(common.TmpDir, fileName)
			if err := runtime.GetRunner().Scp(binary.Path(), dst); err != nil {
				return errors.Wrap(errors.WithStack(err), fmt.Sprintf("sync kube binaries failed"))
			}
			if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("tar -zxf %s -C /opt/cni/bin", dst), false); err != nil {
				return err
			}
		default:
			dst := filepath.Join(common.BinDir, fileName)
			if err := runtime.GetRunner().SudoScp(binary.Path(), dst); err != nil {
				return errors.Wrap(errors.WithStack(err), fmt.Sprintf("sync kube binaries failed"))
			}
			if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chmod +x %s", dst), false); err != nil {
				return err
			}
		}
	}

	binaries := []string{"kubectl", "crictl", "ctr"}
	var createLinkCMDs []string
	for _, binary := range binaries {
		createLinkCMDs = append(createLinkCMDs, fmt.Sprintf("ln -snf /usr/local/bin/k8e /usr/local/bin/%s", binary))
	}
	if _, err := runtime.GetRunner().SudoCmd(strings.Join(createLinkCMDs, " && "), false); err != nil {
		return errors.Wrap(errors.WithStack(err), "create ctl tool link failed")
	}

	return nil
}

type ChmodScript struct {
	common.KubeAction
}

func (c *ChmodScript) Execute(runtime connector.Runtime) error {
	killAllScript := filepath.Join("/usr/local/bin", templates.K8eKillallScript.Name())
	uninstallScript := filepath.Join("/usr/local/bin", templates.K8eUninstallScript.Name())

	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chmod +x %s", killAllScript),
		false); err != nil {
		return err
	}
	if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf("chmod +x %s", uninstallScript),
		false); err != nil {
		return err
	}
	return nil
}

type GenerateK8eService struct {
	common.KubeAction
}

func (g *GenerateK8eService) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()

	var server string
	if !host.IsRole(common.Master) {
		server = fmt.Sprintf("https://%s:%d", g.KubeConf.Cluster.ControlPlaneEndpoint.Domain, g.KubeConf.Cluster.ControlPlaneEndpoint.Port)
	}

	defaultKubeletArs := map[string]string{
		"kube-reserved":   "cpu=200m,memory=250Mi,ephemeral-storage=1Gi",
		"system-reserved": "cpu=200m,memory=250Mi,ephemeral-storage=1Gi",
		"eviction-hard":   "memory.available<5%,nodefs.available<10%",
	}
	defaultKubeProxyArgs := map[string]string{
		"proxy-mode": "ipvs",
	}

	kubeApiserverArgs, _ := util.GetArgs(map[string]string{}, g.KubeConf.Cluster.Kubernetes.ApiServerArgs)
	kubeControllerManager, _ := util.GetArgs(map[string]string{
		"pod-eviction-timeout":        "3m0s",
		"terminated-pod-gc-threshold": "5",
	}, g.KubeConf.Cluster.Kubernetes.ControllerManagerArgs)
	kubeSchedulerArgs, _ := util.GetArgs(map[string]string{}, g.KubeConf.Cluster.Kubernetes.SchedulerArgs)
	kubeletArgs, _ := util.GetArgs(defaultKubeletArs, g.KubeConf.Cluster.Kubernetes.KubeletArgs)
	kubeProxyArgs, _ := util.GetArgs(defaultKubeProxyArgs, g.KubeConf.Cluster.Kubernetes.KubeProxyArgs)

	templateAction := action.Template{
		Template: templates.K8eService,
		Dst:      filepath.Join("/etc/systemd/system/", templates.K8eService.Name()),
		Data: util.Data{
			"Server":            server,
			"IsMaster":          host.IsRole(common.Master),
			"NodeIP":            host.GetInternalIPv4Address(),
			"HostName":          host.GetName(),
			"PodSubnet":         g.KubeConf.Cluster.Network.KubePodsCIDR,
			"ServiceSubnet":     g.KubeConf.Cluster.Network.KubeServiceCIDR,
			"ClusterDns":        g.KubeConf.Cluster.CorednsClusterIP(),
			"CertSANs":          g.KubeConf.Cluster.GenerateCertSANs(),
			"PauseImage":        images.GetImage(runtime, g.KubeConf, "pause").ImageName(),
			"ApiserverArgs":     kubeApiserverArgs,
			"ControllerManager": kubeControllerManager,
			"SchedulerArgs":     kubeSchedulerArgs,
			"KubeletArgs":       kubeletArgs,
			"KubeProxyArgs":     kubeProxyArgs,
		},
	}

	templateAction.Init(nil, nil)
	if err := templateAction.Execute(runtime); err != nil {
		return err
	}
	return nil
}

type GenerateK8eServiceEnv struct {
	common.KubeAction
}

func (g *GenerateK8eServiceEnv) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()

	clusterStatus, ok := g.PipelineCache.Get(common.ClusterStatus)
	if !ok {
		return errors.New("get cluster status by pipeline cache failed")
	}
	cluster := clusterStatus.(*K8eStatus)

	var externalEtcd kubekeyapiv1alpha2.ExternalEtcd
	var endpointsList []string
	var externalEtcdEndpoints, token string

	switch g.KubeConf.Cluster.Etcd.Type {
	case kubekeyapiv1alpha2.External:
		externalEtcd.Endpoints = g.KubeConf.Cluster.Etcd.External.Endpoints

		if len(g.KubeConf.Cluster.Etcd.External.CAFile) != 0 && len(g.KubeConf.Cluster.Etcd.External.CAFile) != 0 && len(g.KubeConf.Cluster.Etcd.External.CAFile) != 0 {
			externalEtcd.CAFile = fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(g.KubeConf.Cluster.Etcd.External.CAFile))
			externalEtcd.CertFile = fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(g.KubeConf.Cluster.Etcd.External.CertFile))
			externalEtcd.KeyFile = fmt.Sprintf("/etc/ssl/etcd/ssl/%s", filepath.Base(g.KubeConf.Cluster.Etcd.External.KeyFile))
		}
	default:
		for _, node := range runtime.GetHostsByRole(common.ETCD) {
			endpoint := fmt.Sprintf("https://%s:%d", node.GetInternalIPv4Address(), g.KubeConf.Cluster.Etcd.GetPort())
			endpointsList = append(endpointsList, endpoint)
		}
		externalEtcd.Endpoints = endpointsList

		externalEtcd.CAFile = "/etc/ssl/etcd/ssl/ca.pem"
		externalEtcd.CertFile = fmt.Sprintf("/etc/ssl/etcd/ssl/node-%s.pem", runtime.GetHostsByRole(common.Master)[0].GetName())
		externalEtcd.KeyFile = fmt.Sprintf("/etc/ssl/etcd/ssl/node-%s-key.pem", runtime.GetHostsByRole(common.Master)[0].GetName())
	}

	externalEtcdEndpoints = strings.Join(externalEtcd.Endpoints, ",")

	v121 := versionutil.MustParseSemantic("v1.21.0")
	atLeast := versionutil.MustParseSemantic(g.KubeConf.Cluster.Kubernetes.Version).AtLeast(v121)
	if atLeast {
		token = cluster.NodeToken
	} else {
		if !host.IsRole(common.Master) {
			token = cluster.NodeToken
		}
	}

	templateAction := action.Template{
		Template: templates.K8eServiceEnv,
		Dst:      filepath.Join("/etc/systemd/system/", templates.K8eServiceEnv.Name()),
		Data: util.Data{
			"DataStoreEndPoint": externalEtcdEndpoints,
			"DataStoreCaFile":   externalEtcd.CAFile,
			"DataStoreCertFile": externalEtcd.CertFile,
			"DataStoreKeyFile":  externalEtcd.KeyFile,
			"IsMaster":          host.IsRole(common.Master),
			"Token":             token,
		},
	}

	templateAction.Init(nil, nil)
	if err := templateAction.Execute(runtime); err != nil {
		return err
	}
	return nil
}

type EnableK8eService struct {
	common.KubeAction
}

func (e *EnableK8eService) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("systemctl daemon-reload && systemctl enable --now k8e",
		false); err != nil {
		return errors.Wrap(errors.WithStack(err), "enable k8e failed")
	}
	return nil
}

type CopyK8eKubeConfig struct {
	common.KubeAction
}

func (c *CopyK8eKubeConfig) Execute(runtime connector.Runtime) error {
	createConfigDirCmd := "mkdir -p /root/.kube && mkdir -p $HOME/.kube"
	getKubeConfigCmd := "cp -f /etc/k8e/k8e.yaml /root/.kube/config"

	cmd := strings.Join([]string{createConfigDirCmd, getKubeConfigCmd}, " && ")
	if _, err := runtime.GetRunner().SudoCmd(cmd, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "copy k8e kube config failed")
	}

	userMkdir := "mkdir -p $HOME/.kube"
	if _, err := runtime.GetRunner().Cmd(userMkdir, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "user mkdir $HOME/.kube failed")
	}

	userCopyKubeConfig := "cp -f /etc/k8e/k8e.yaml $HOME/.kube/config"
	if _, err := runtime.GetRunner().SudoCmd(userCopyKubeConfig, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "user copy /etc/k8e/k8e.yaml to $HOME/.kube/config failed")
	}

	userId, err := runtime.GetRunner().Cmd("echo $(id -u)", false)
	if err != nil {
		return errors.Wrap(errors.WithStack(err), "get user id failed")
	}

	userGroupId, err := runtime.GetRunner().Cmd("echo $(id -g)", false)
	if err != nil {
		return errors.Wrap(errors.WithStack(err), "get user group id failed")
	}

	chownKubeConfig := fmt.Sprintf("chown -R %s:%s $HOME/.kube", userId, userGroupId)
	if _, err := runtime.GetRunner().SudoCmd(chownKubeConfig, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "chown user kube config failed")
	}
	return nil
}

type AddMasterTaint struct {
	common.KubeAction
}

func (a *AddMasterTaint) Execute(runtime connector.Runtime) error {
	host := runtime.RemoteHost()

	cmd := fmt.Sprintf(
		"/usr/local/bin/kubectl taint nodes %s node-role.kubernetes.io/master=effect:NoSchedule --overwrite",
		host.GetName())

	if _, err := runtime.GetRunner().SudoCmd(cmd, false); err != nil {
		return errors.Wrap(errors.WithStack(err), "add master NoSchedule taint failed")
	}
	return nil
}

type AddWorkerLabel struct {
	common.KubeAction
}

func (a *AddWorkerLabel) Execute(runtime connector.Runtime) error {
	for _, host := range runtime.GetAllHosts() {
		if host.IsRole(common.Worker) {
			if _, err := runtime.GetRunner().SudoCmd(fmt.Sprintf(
				"/usr/local/bin/kubectl label --overwrite node %s node-role.kubernetes.io/worker=",
				host.GetName()), true); err != nil {
				return errors.Wrap(errors.WithStack(err), "add worker label failed")
			}
		}
	}

	return nil
}

type ExecUninstallScript struct {
	common.KubeAction
}

func (e *ExecUninstallScript) Execute(runtime connector.Runtime) error {
	if _, err := runtime.GetRunner().SudoCmd("systemctl daemon-reload && /usr/local/bin/k8e-killall.sh",
		true); err != nil {
		return errors.Wrap(errors.WithStack(err), "add master NoSchedule taint failed")
	}
	if _, err := runtime.GetRunner().SudoCmd("systemctl daemon-reload && /usr/local/bin/k8e-uninstall.sh",
		true); err != nil {
		return errors.Wrap(errors.WithStack(err), "add master NoSchedule taint failed")
	}
	return nil
}

type SaveKubeConfig struct {
	common.KubeAction
}

func (s *SaveKubeConfig) Execute(_ connector.Runtime) error {
	status, ok := s.PipelineCache.Get(common.ClusterStatus)
	if !ok {
		return errors.New("get kubernetes status failed by pipeline cache")
	}
	cluster := status.(*K8eStatus)

	oldServer := fmt.Sprintf("https://%s:%d", s.KubeConf.Cluster.ControlPlaneEndpoint.Domain, s.KubeConf.Cluster.ControlPlaneEndpoint.Port)
	newServer := fmt.Sprintf("https://%s:%d", s.KubeConf.Cluster.ControlPlaneEndpoint.Address, s.KubeConf.Cluster.ControlPlaneEndpoint.Port)
	newKubeConfigStr := strings.Replace(cluster.KubeConfig, oldServer, newServer, -1)
	kubeConfigBase64 := base64.StdEncoding.EncodeToString([]byte(newKubeConfigStr))

	config, err := clientcmd.NewClientConfigFromBytes([]byte(newKubeConfigStr))
	if err != nil {
		return err
	}
	restConfig, err := config.ClientConfig()
	if err != nil {
		return err
	}
	clientsetForCluster, err := kube.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubekey-system",
		},
	}
	if _, err := clientsetForCluster.
		CoreV1().
		Namespaces().
		Create(context.TODO(), namespace, metav1.CreateOptions{}); err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-kubeconfig", s.KubeConf.ClusterName),
		},
		Data: map[string]string{
			"kubeconfig": kubeConfigBase64,
		},
	}

	if _, err := clientsetForCluster.
		CoreV1().
		ConfigMaps("kubekey-system").
		Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
		return err
	}
	return nil
}
