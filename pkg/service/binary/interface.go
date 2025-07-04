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

// Package binary define the binaries operations on the remote instance.
package binary

import (
	"time"

	infrav1 "github.com/kubesys/kubekey/api/v1beta1"
	"github.com/kubesys/kubekey/pkg/clients/ssh"
	"github.com/kubesys/kubekey/pkg/scope"
	"github.com/kubesys/kubekey/pkg/service/binary/k3s"
	"github.com/kubesys/kubekey/pkg/service/binary/kubernetes"
)

// Binary defines the interface for the binaries operations.
type Binary interface {
	Download(timeout time.Duration) error
	UpgradeDownload(timeout time.Duration) error
}

// NewService returns a new service for the binaries operations.
func NewService(sshClient ssh.Interface, scope scope.KKInstanceScope, instanceScope *scope.InstanceScope, distribution string) Binary {
	switch distribution {
	case infrav1.KUBERNETES:
		return kubernetes.NewService(sshClient, scope, instanceScope)
	case infrav1.K3S:
		return k3s.NewService(sshClient, scope, instanceScope)
	}
	return nil
}
