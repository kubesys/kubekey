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

package cluster

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infracontrolplanev1 "github.com/kubesys/kubekey/controlplane/k3s/api/v1beta1"
)

// ControlPlaneMachineLabelsForCluster returns a set of labels to add to a control plane machine for this specific cluster.
func ControlPlaneMachineLabelsForCluster(kcp *infracontrolplanev1.K3sControlPlane, clusterName string) map[string]string {
	labels := map[string]string{}

	// Add the labels from the MachineTemplate.
	// Note: we intentionally don't use the map directly to ensure we don't modify the map in KCP.
	for k, v := range kcp.Spec.MachineTemplate.ObjectMeta.Labels {
		labels[k] = v
	}

	// Always force these labels over the ones coming from the spec.
	labels[clusterv1.ClusterLabelName] = clusterName
	labels[clusterv1.MachineControlPlaneLabelName] = ""
	return labels
}
