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

package confirm

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	versionK8S "github.com/kubesys/kubekey/cmd/kk/pkg/version/kubernetes"

	kubekeyapiv1alpha2 "github.com/kubesys/kubekey/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/action"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/connector"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/logger"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/util"
	"github.com/mitchellh/mapstructure"
	"github.com/modood/table"
	"github.com/pkg/errors"
	versionutil "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/utils/strings/slices"
)

// PreCheckResults defines the items to be checked.
type PreCheckResults struct {
	Name       string `table:"name"`
	Sudo       string `table:"sudo"`
	Curl       string `table:"curl"`
	Openssl    string `table:"openssl"`
	Ebtables   string `table:"ebtables"`
	Socat      string `table:"socat"`
	Ipset      string `table:"ipset"`
	Ipvsadm    string `table:"ipvsadm"`
	Conntrack  string `table:"conntrack"`
	Chronyd    string `table:"chrony"`
	Docker     string `table:"docker"`
	Containerd string `table:"containerd"`
	Nfs        string `table:"nfs client"`
	Ceph       string `table:"ceph client"`
	Glusterfs  string `table:"glusterfs client"`
	Time       string `table:"time"`
}

type InstallationConfirm struct {
	common.KubeAction
}

func (i *InstallationConfirm) Execute(runtime connector.Runtime) error {
	var (
		results  []PreCheckResults
		stopFlag bool
	)

	pre := make([]map[string]string, 0, len(runtime.GetAllHosts()))
	for _, host := range runtime.GetAllHosts() {
		if v, ok := host.GetCache().Get(common.NodePreCheck); ok {
			pre = append(pre, v.(map[string]string))
		} else {
			return errors.New("get node check result failed by host cache")
		}
	}

	for node := range pre {
		var result PreCheckResults
		_ = mapstructure.Decode(pre[node], &result)
		results = append(results, result)
	}
	table.OutputA(results)
	reader := bufio.NewReader(os.Stdin)

	if !i.KubeConf.Arg.InstallPackages {
		for _, host := range results {
			if host.Sudo == "" {
				logger.Log.Errorf("%s: sudo is required.", host.Name)
				stopFlag = true
			}

			if host.Conntrack == "" {
				logger.Log.Errorf("%s: conntrack is required.", host.Name)
				stopFlag = true
			}

			if host.Socat == "" {
				logger.Log.Errorf("%s: socat is required.", host.Name)
				stopFlag = true
			}
		}
	}

	fmt.Println("")
	fmt.Println("This is a simple check of your environment.")
	fmt.Println("Before installation, ensure that your machines meet all requirements specified at")
	fmt.Println("https://github.com/kubesys/kubekey#requirements-and-recommendations")
	fmt.Println("")

	// check k8s version is supported
	k8sVersion := i.KubeConf.Cluster.Kubernetes.Version
	if k8sVersion != kubekeyapiv1alpha2.DefaultKubeVersion {
		suppportVersions := versionK8S.SupportedK8sVersionList()
		if !strings.HasPrefix(k8sVersion, "v") {
			k8sVersion = "v" + k8sVersion
		}
		if !slices.Contains(suppportVersions, k8sVersion) {
			fmt.Printf("The Kubernetes version: %s isn't supported.\n", k8sVersion)
			fmt.Println("Use kk version --show-supported-k8s,show supported k8s versions")
			fmt.Println("")
			stopFlag = true
		} else {
			fmt.Println("Install k8s with specify version: ", k8sVersion)
			fmt.Println("")
		}
	} else {
		fmt.Println("Install k8s with default version: ", kubekeyapiv1alpha2.DefaultKubeVersion)
		fmt.Println("")
	}

	if i.KubeConf.Cluster.Kubernetes.IsAtLeastV124() && i.KubeConf.Cluster.Kubernetes.ContainerManager == common.Docker {
		fmt.Println("[Notice]")
		fmt.Println("For Kubernetes v1.24 and later, dockershim has been deprecated.")
		fmt.Println("Current runtime is set to Docker and `cri-dockerd` will be installed to support Kubernetes v1.24 and later.")
		fmt.Println("Yoc can also specify a container runtime other than Docker to install Kubernetes v1.24 or later.")
		fmt.Println("You can set \"spec.kubernetes.containerManager\" in the configuration file to \"containerd\" or add \"--container-manager containerd\" to the \"./kk create cluster\" command.")
		fmt.Println("For more information, see:")
		fmt.Println("https://github.com/kubesys/kubekey/blob/master/docs/commands/kk-create-cluster.md")
		fmt.Println("https://kubernetes.io/docs/setup/production-environment/container-runtimes/#container-runtimes")
		fmt.Println("https://kubernetes.io/blog/2022/02/17/dockershim-faq/")
		fmt.Println("https://github.com/Mirantis/cri-dockerd")
		fmt.Println("")
	}

	if stopFlag {
		os.Exit(1)
	}

	if i.KubeConf.Arg.SkipConfirmCheck {
		return nil
	}

	confirmOK := false
	for !confirmOK {
		fmt.Printf("Continue this installation? [yes/no]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			logger.Log.Fatal(err)
		}
		input = strings.TrimSpace(strings.ToLower(input))

		switch strings.ToLower(input) {
		case "yes", "y":
			confirmOK = true
		case "no", "n":
			os.Exit(0)
		default:
			continue
		}
	}
	return nil
}

type DeleteConfirm struct {
	common.KubeAction
	Content string
}

func (d *DeleteConfirm) Execute(runtime connector.Runtime) error {
	reader := bufio.NewReader(os.Stdin)

	confirmOK := false
	for !confirmOK {
		fmt.Printf("Are you sure to delete this %s? [yes/no]: ", d.Content)
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.ToLower(strings.TrimSpace(input))

		switch strings.ToLower(input) {
		case "yes", "y":
			confirmOK = true
		case "no", "n":
			os.Exit(0)
		default:
			continue
		}
	}

	return nil
}

type UpgradeConfirm struct {
	common.KubeAction
}

func (u *UpgradeConfirm) Execute(runtime connector.Runtime) error {
	pre := make([]map[string]string, len(runtime.GetAllHosts()), len(runtime.GetAllHosts()))
	for i, host := range runtime.GetAllHosts() {
		if v, ok := host.GetCache().Get(common.NodePreCheck); ok {
			pre[i] = v.(map[string]string)
		} else {
			return errors.New("get node check result failed by host cache")
		}
	}

	results := make([]PreCheckResults, len(pre), len(pre))
	for i := range pre {
		var result PreCheckResults
		_ = mapstructure.Decode(pre[i], &result)
		results[i] = result
	}
	table.OutputA(results)
	fmt.Println()

	warningFlag := false
	cmp, err := versionutil.MustParseSemantic(u.KubeConf.Cluster.Kubernetes.Version).Compare("v1.19.0")
	if err != nil {
		logger.Log.Fatalf("Failed to compare kubernetes version: %v", err)
	}
	if cmp == 0 || cmp == 1 {
		for _, result := range results {
			if len(result.Docker) != 0 {
				dockerVersion, err := RefineDockerVersion(result.Docker)
				if err != nil {
					logger.Log.Fatalf("Failed to get docker version: %v", err)
				}
				cmp, err := versionutil.MustParseSemantic(dockerVersion).Compare("20.10.0")
				if err != nil {
					logger.Log.Fatalf("Failed to compare docker version: %v", err)
				}
				warningFlag = warningFlag || (cmp == -1)
			}
		}
		if warningFlag {
			fmt.Println(`
Warning:

  An old Docker version may cause the failure of upgrade. It is recommended that you upgrade Docker to 20.10+ beforehand.

  Issue: https://github.com/kubernetes/kubernetes/issues/101056`)
			fmt.Print("\n")
		}
	}

	nodeStats, ok := u.PipelineCache.GetMustString(common.ClusterNodeStatus)
	if !ok {
		return errors.New("get cluster nodes status failed by pipeline cache")
	}
	fmt.Println("Cluster nodes status:")
	fmt.Println(nodeStats + "\n")

	fmt.Println("Upgrade Confirmation:")
	currentK8sVersion, ok := u.PipelineCache.GetMustString(common.K8sVersion)
	if !ok {
		return errors.New("get current Kubernetes version failed by pipeline cache")
	}
	fmt.Printf("kubernetes version: %s to %s\n", currentK8sVersion, u.KubeConf.Cluster.Kubernetes.Version)

	if u.KubeConf.Cluster.KubeSphere.Enabled {
		currentKsVersion, ok := u.PipelineCache.GetMustString(common.KubeSphereVersion)
		if !ok {
			return errors.New("get current KubeSphere version failed by pipeline cache")
		}
		fmt.Printf("kubesphere version: %s to %s\n", currentKsVersion, u.KubeConf.Cluster.KubeSphere.Version)
	}
	fmt.Println()

	if k8sVersion, err := versionutil.ParseGeneric(u.KubeConf.Cluster.Kubernetes.Version); err == nil {
		if cri, ok := u.PipelineCache.GetMustString(common.ClusterNodeCRIRuntimes); ok {
			k8sV124 := versionutil.MustParseSemantic("v1.24.0")
			if k8sVersion.AtLeast(k8sV124) && versionutil.MustParseSemantic(currentK8sVersion).LessThan(k8sV124) && strings.Contains(cri, "docker") {
				fmt.Println("[Notice]")
				fmt.Println("For Kubernetes v1.24 and later, dockershim has been deprecated.")
				fmt.Println("The container runtime of the current cluster is Docker, `cri-dockerd` will be installed to support Kubernetes v1.24 and later.")
				fmt.Println("You can also migrate container runtime from Docker to other runtimes that are compatible with the Kubernetes CRI.")
				fmt.Println("For more information, see:")
				fmt.Println("https://kubernetes.io/docs/setup/production-environment/container-runtimes/#container-runtimes")
				fmt.Println("https://kubernetes.io/blog/2022/02/17/dockershim-faq/")
				fmt.Println("https://github.com/Mirantis/cri-dockerd")
				fmt.Println("https://kubernetes.io/docs/tasks/administer-cluster/migrating-from-dockershim/change-runtime-containerd/")
				fmt.Println("")
			}
		}
	}

	if featureGates, ok := u.PipelineCache.GetMustString(common.ClusterFeatureGates); ok {
		if featureGates != "" {
			fmt.Println("[Notice]")
			fmt.Println("The feature-gates in the cluster is as follow:")
			fmt.Println("")
			fmt.Printf("         %s\n", featureGates)
			fmt.Println("")
			fmt.Println("Please ensure that there are no deprecated feature-gate in the target version.")
			fmt.Println("You can modify the feature-gates in `kubeadm-config` and `kubelet-config` configmaps in the kube-system namespace.")
			fmt.Println("")
		}
	}

	reader := bufio.NewReader(os.Stdin)
	confirmOK := false
	for !confirmOK {
		fmt.Printf("Continue upgrading cluster? [yes/no]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.ToLower(strings.TrimSpace(input))

		switch input {
		case "yes", "y":
			confirmOK = true
		case "no", "n":
			os.Exit(0)
		default:
			continue
		}
	}
	return nil
}

func RefineDockerVersion(version string) (string, error) {
	var newVersionComponents []string
	versionMatchRE := regexp.MustCompile(`^\s*v?([0-9]+(?:\.[0-9]+)*)(.*)*$`)
	parts := versionMatchRE.FindStringSubmatch(version)
	if parts == nil {
		return "", fmt.Errorf("could not parse %q as version", version)
	}
	numbers, _ := parts[1], parts[2]
	components := strings.Split(numbers, ".")

	for _, c := range components {
		newVersion := strings.TrimPrefix(c, "0")
		if newVersion == "" {
			newVersion = "0"
		}
		newVersionComponents = append(newVersionComponents, newVersion)
	}
	return strings.Join(newVersionComponents, "."), nil
}

type CheckFile struct {
	action.BaseAction
	FileName string
}

func (c *CheckFile) Execute(runtime connector.Runtime) error {
	if util.IsExist(c.FileName) {
		reader := bufio.NewReader(os.Stdin)
		stop := false
		for {
			if stop {
				break
			}
			fmt.Printf("%s already exists. Are you sure you want to overwrite this file? [yes/no]: ", c.FileName)
			input, _ := reader.ReadString('\n')
			input = strings.ToLower(strings.TrimSpace(input))

			if input != "" {
				switch input {
				case "yes", "y":
					stop = true
				case "no", "n":
					os.Exit(0)
				}
			}
		}
	}
	return nil
}

type MigrateCri struct {
	common.KubeAction
}

func (d *MigrateCri) Execute(runtime connector.Runtime) error {
	reader := bufio.NewReader(os.Stdin)

	confirmOK := false
	for !confirmOK {
		fmt.Printf("Are you sure to Migrate Cri? [yes/no]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		input = strings.ToLower(strings.TrimSpace(input))

		switch strings.ToLower(input) {
		case "yes", "y":
			confirmOK = true
		case "no", "n":
			os.Exit(0)
		default:
			continue
		}
	}

	return nil
}
