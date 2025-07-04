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

package images

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"

	kubekeyapiv1alpha2 "github.com/kubesys/kubekey/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/connector"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/logger"
	"github.com/kubesys/kubekey/cmd/kk/apis/kubekey/v1alpha2"
)

const (
	cnRegistry          = "registry.cn-beijing.aliyuncs.com"
	cnNamespaceOverride = "kubesphereio"
)

// Image defines image's info.
type Image struct {
	RepoAddr          string
	Namespace         string
	NamespaceOverride string
	Repo              string
	Tag               string
	Group             string
	Enable            bool
	NamespaceRewrite  *v1alpha2.NamespaceRewrite
}

// Images contains a list of Image
type Images struct {
	Images []Image
}

// ImageName is used to generate image's full name.
func (image Image) ImageName() string {
	return fmt.Sprintf("%s:%s", image.ImageRepo(), image.Tag)
}

// ImageNamespace is used to get image's namespace
func (image Image) ImageNamespace() string {
	if os.Getenv("KKZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == cnRegistry {
			image.NamespaceOverride = cnNamespaceOverride
		}
	}

	if image.NamespaceOverride != "" {
		return image.NamespaceOverride
	} else {
		return image.Namespace
	}
}

// ImageRegistryAddr is used to get image's registry address.
func (image Image) ImageRegistryAddr() string {
	if os.Getenv("KKZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == cnRegistry {
			image.RepoAddr = cnRegistry
		}
	}
	if image.RepoAddr != "" {
		return image.RepoAddr
	} else {
		return "docker.io"
	}
}

// ImageRepo is used to generate image's repo address.
func (image Image) ImageRepo() string {
	var prefix string

	if os.Getenv("KKZONE") == "cn" {
		if image.RepoAddr == "" || image.RepoAddr == cnRegistry {
			image.RepoAddr = cnRegistry
			image.NamespaceOverride = cnNamespaceOverride
		}
	}

	if image.NamespaceRewrite != nil {
		switch image.NamespaceRewrite.Policy {
		case v1alpha2.ChangePrefix:
			matchSrc := ""
			for _, src := range image.NamespaceRewrite.Src {
				if strings.Contains(image.Namespace, src) {
					matchSrc = src
				}
			}
			modifiedNamespace := ""
			if matchSrc == "" {
				// if not match, add dest prefix
				modifiedNamespace = fmt.Sprintf("%s/%s", image.NamespaceRewrite.Dest, image.Namespace)
			} else {
				// if match, change it
				modifiedNamespace = strings.ReplaceAll(image.Namespace, matchSrc, image.NamespaceRewrite.Dest)
			}
			logger.Log.Debugf("changed image namespace: %s -> %s", image.Namespace, modifiedNamespace)
			image.Namespace = modifiedNamespace
		default:
			logger.Log.Warn("namespace rewrite action not specified")
		}
	}

	if image.RepoAddr == "" {
		if image.Namespace == "" {
			prefix = ""
		} else {
			prefix = fmt.Sprintf("%s/", image.Namespace)
		}
	} else {
		if image.NamespaceOverride == "" {
			if image.Namespace == "" {
				prefix = fmt.Sprintf("%s/library/", image.RepoAddr)
			} else {
				prefix = fmt.Sprintf("%s/%s/", image.RepoAddr, image.Namespace)
			}
		} else {
			prefix = fmt.Sprintf("%s/%s/", image.RepoAddr, image.NamespaceOverride)
		}
	}

	return fmt.Sprintf("%s%s", prefix, image.Repo)
}

// PullImages is used to pull images in the list of Image.
func (images *Images) PullImages(runtime connector.Runtime, kubeConf *common.KubeConf) error {
	pullCmd := "docker"
	switch kubeConf.Cluster.Kubernetes.ContainerManager {
	case "crio":
		pullCmd = "crictl"
	case "containerd":
		pullCmd = "crictl"
	case "isula":
		pullCmd = "isula"
	default:
		pullCmd = "docker"
	}

	host := runtime.RemoteHost()

	for _, image := range images.Images {
		switch {
		case host.IsRole(common.Master) && image.Group == kubekeyapiv1alpha2.Master && image.Enable,
			host.IsRole(common.Worker) && image.Group == kubekeyapiv1alpha2.Worker && image.Enable,
			(host.IsRole(common.Master) || host.IsRole(common.Worker)) && image.Group == kubekeyapiv1alpha2.K8s && image.Enable,
			host.IsRole(common.ETCD) && image.Group == kubekeyapiv1alpha2.Etcd && image.Enable:

			imagePullName := image.ImageName()
			logger.Log.Messagef(host.GetName(), "downloading image: %s", imagePullName)

			var pullCommand string
			if pullCmd == "crictl" {
				pullCommand = fmt.Sprintf("env PATH=$PATH %s pull %s", pullCmd, imagePullName)
			} else {
				pullCommand = fmt.Sprintf("env PATH=$PATH %s pull %s --platform %s", pullCmd, imagePullName, host.GetArch())
			}

			if _, err := runtime.GetRunner().SudoCmd(pullCommand, false); err != nil {
				return errors.Wrap(err, "pull image failed")
			}
		default:
			continue
		}
	}
	return nil
}

// DefaultRegistry is used to get default registry address.
func DefaultRegistry() string {
	if os.Getenv("KKZONE") == "cn" {
		return cnRegistry
	}
	return "docker.io"
}
