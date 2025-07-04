/*
Copyright 2020 The Kubernetes Authors.

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

package framework

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrabootstrapv1 "github.com/kubesys/kubekey/bootstrap/k3s/api/v1beta1"
	infracontrolplanev1 "github.com/kubesys/kubekey/controlplane/k3s/api/v1beta1"
	"github.com/kubesys/kubekey/test/e2e/framework/internal/log"
)

// CreateK3sControlPlaneInput is the input for CreateK3sControlPlane.
type CreateK3sControlPlaneInput struct {
	Creator         Creator
	ControlPlane    *infracontrolplanev1.K3sControlPlane
	MachineTemplate client.Object
}

// CreateK3sControlPlane creates the control plane object and necessary dependencies.
func CreateK3sControlPlane(ctx context.Context, input CreateK3sControlPlaneInput, intervals ...interface{}) {
	By("creating the machine template")
	Eventually(func() error {
		return input.Creator.Create(ctx, input.MachineTemplate)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to create MachineTemplate %s", input.MachineTemplate.GetName())

	By("creating a K3sControlPlane")
	Eventually(func() error {
		err := input.Creator.Create(ctx, input.ControlPlane)
		if err != nil {
			log.Logf("Failed to create the K3sControlPlane: %+v", err)
		}
		return err
	}, intervals...).Should(Succeed(), "Failed to create the K3sControlPlane %s", klog.KObj(input.ControlPlane))
}

// GetK3sControlPlaneByClusterInput is the input for GetK3sControlPlaneByCluster.
type GetK3sControlPlaneByClusterInput struct {
	Lister      Lister
	ClusterName string
	Namespace   string
}

// GetK3sControlPlaneByCluster returns the K3sControlPlane objects for a cluster.
// Important! this method relies on labels that are created by the CAPI controllers during the first reconciliation, so
// it is necessary to ensure this is already happened before calling it.
func GetK3sControlPlaneByCluster(ctx context.Context, input GetK3sControlPlaneByClusterInput) *infracontrolplanev1.K3sControlPlane {
	controlPlaneList := &infracontrolplanev1.K3sControlPlaneList{}
	Eventually(func() error {
		return input.Lister.List(ctx, controlPlaneList, byClusterOptions(input.ClusterName, input.Namespace)...)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to list K3sControlPlane object for Cluster %s", klog.KRef(input.Namespace, input.ClusterName))
	Expect(len(controlPlaneList.Items)).ToNot(BeNumerically(">", 1), "Cluster %s should not have more than 1 K3sControlPlane object", klog.KRef(input.Namespace, input.ClusterName))
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// WaitForK3sControlPlaneMachinesToExistInput is the input for WaitForK3sControlPlaneMachinesToExist.
type WaitForK3sControlPlaneMachinesToExistInput struct {
	Lister       Lister
	Cluster      *clusterv1.Cluster
	ControlPlane *infracontrolplanev1.K3sControlPlane
}

// WaitForK3sControlPlaneMachinesToExist will wait until all control plane machines have node refs.
func WaitForK3sControlPlaneMachinesToExist(ctx context.Context, input WaitForK3sControlPlaneMachinesToExistInput, intervals ...interface{}) {
	By("Waiting for all control plane nodes to exist")
	inClustersNamespaceListOption := client.InNamespace(input.Cluster.Namespace)
	// ControlPlane labels
	matchClusterListOption := client.MatchingLabels{
		clusterv1.MachineControlPlaneLabelName: "",
		clusterv1.ClusterLabelName:             input.Cluster.Name,
	}

	Eventually(func() (int, error) {
		machineList := &clusterv1.MachineList{}
		if err := input.Lister.List(ctx, machineList, inClustersNamespaceListOption, matchClusterListOption); err != nil {
			log.Logf("Failed to list the machines: %+v", err)
			return 0, err
		}
		count := 0
		for _, machine := range machineList.Items {
			if machine.Status.NodeRef != nil {
				count++
			}
		}
		return count, nil
	}, intervals...).Should(Equal(int(*input.ControlPlane.Spec.Replicas)), "Timed out waiting for %d control plane machines to exist", int(*input.ControlPlane.Spec.Replicas))
}

// WaitForOneK3sControlPlaneMachineToExistInput is the input for WaitForK3sControlPlaneMachinesToExist.
type WaitForOneK3sControlPlaneMachineToExistInput struct {
	Lister       Lister
	Cluster      *clusterv1.Cluster
	ControlPlane *infracontrolplanev1.K3sControlPlane
}

// WaitForOneK3sControlPlaneMachineToExist will wait until all control plane machines have node refs.
func WaitForOneK3sControlPlaneMachineToExist(ctx context.Context, input WaitForOneK3sControlPlaneMachineToExistInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WaitForOneK3sControlPlaneMachineToExist")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Getter can't be nil when calling WaitForOneK3sControlPlaneMachineToExist")
	Expect(input.ControlPlane).ToNot(BeNil(), "Invalid argument. input.ControlPlane can't be nil when calling WaitForOneK3sControlPlaneMachineToExist")

	By("Waiting for one control plane node to exist")
	inClustersNamespaceListOption := client.InNamespace(input.Cluster.Namespace)
	// ControlPlane labels
	matchClusterListOption := client.MatchingLabels{
		clusterv1.MachineControlPlaneLabelName: "",
		clusterv1.ClusterLabelName:             input.Cluster.Name,
	}

	Eventually(func() (bool, error) {
		machineList := &clusterv1.MachineList{}
		if err := input.Lister.List(ctx, machineList, inClustersNamespaceListOption, matchClusterListOption); err != nil {
			log.Logf("Failed to list the machines: %+v", err)
			return false, err
		}
		count := 0
		for _, machine := range machineList.Items {
			if machine.Status.NodeRef != nil {
				count++
			}
		}
		return count > 0, nil
	}, intervals...).Should(BeTrue(), "No Control Plane machines came into existence. ")
}

// WaitForControlPlaneToBeReadyInput is the input for WaitForControlPlaneToBeReady.
type WaitForControlPlaneToBeReadyInput struct {
	Getter       Getter
	ControlPlane *infracontrolplanev1.K3sControlPlane
}

// WaitForControlPlaneToBeReady will wait for a control plane to be ready.
func WaitForControlPlaneToBeReady(ctx context.Context, input WaitForControlPlaneToBeReadyInput, intervals ...interface{}) {
	By("Waiting for the control plane to be ready")
	controlplane := &infracontrolplanev1.K3sControlPlane{}
	Eventually(func() (infracontrolplanev1.K3sControlPlane, error) {
		key := client.ObjectKey{
			Namespace: input.ControlPlane.GetNamespace(),
			Name:      input.ControlPlane.GetName(),
		}
		if err := input.Getter.Get(ctx, key, controlplane); err != nil {
			return *controlplane, errors.Wrapf(err, "failed to get KCP")
		}
		return *controlplane, nil
	}, intervals...).Should(MatchFields(IgnoreExtras, Fields{
		"Status": MatchFields(IgnoreExtras, Fields{
			"Ready": BeTrue(),
		}),
	}), PrettyPrint(controlplane)+"\n")
}

// AssertControlPlaneFailureDomainsInput is the input for AssertControlPlaneFailureDomains.
type AssertControlPlaneFailureDomainsInput struct {
	Lister  Lister
	Cluster *clusterv1.Cluster
}

// AssertControlPlaneFailureDomains will look at all control plane machines and see what failure domains they were
// placed in. If machines were placed in unexpected or wrong failure domains the expectation will fail.
func AssertControlPlaneFailureDomains(ctx context.Context, input AssertControlPlaneFailureDomainsInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for AssertControlPlaneFailureDomains")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling AssertControlPlaneFailureDomains")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling AssertControlPlaneFailureDomains")

	By("Checking all the control plane machines are in the expected failure domains")
	controlPlaneFailureDomains := sets.NewString()
	for fd, fdSettings := range input.Cluster.Status.FailureDomains {
		if fdSettings.ControlPlane {
			controlPlaneFailureDomains.Insert(fd)
		}
	}

	// Look up all the control plane machines.
	inClustersNamespaceListOption := client.InNamespace(input.Cluster.Namespace)
	matchClusterListOption := client.MatchingLabels{
		clusterv1.ClusterLabelName:             input.Cluster.Name,
		clusterv1.MachineControlPlaneLabelName: "",
	}

	machineList := &clusterv1.MachineList{}
	Eventually(func() error {
		return input.Lister.List(ctx, machineList, inClustersNamespaceListOption, matchClusterListOption)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Couldn't list control-plane machines for the cluster %q", input.Cluster.Name)

	for _, machine := range machineList.Items {
		if machine.Spec.FailureDomain != nil {
			machineFD := *machine.Spec.FailureDomain
			if !controlPlaneFailureDomains.Has(machineFD) {
				Fail(fmt.Sprintf("Machine %s is in the %q failure domain, expecting one of the failure domain defined at cluster level", machine.Name, machineFD))
			}
		}
	}
}

// DiscoveryAndWaitForControlPlaneInitializedInput is the input type for DiscoveryAndWaitForControlPlaneInitialized.
type DiscoveryAndWaitForControlPlaneInitializedInput struct {
	Lister  Lister
	Cluster *clusterv1.Cluster
}

// DiscoveryAndWaitForControlPlaneInitialized discovers the K3sControlPlane object attached to a cluster and waits for it to be initialized.
func DiscoveryAndWaitForControlPlaneInitialized(ctx context.Context, input DiscoveryAndWaitForControlPlaneInitializedInput, intervals ...interface{}) *infracontrolplanev1.K3sControlPlane {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoveryAndWaitForControlPlaneInitialized")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoveryAndWaitForControlPlaneInitialized")

	var controlPlane *infracontrolplanev1.K3sControlPlane
	Eventually(func(g Gomega) {
		controlPlane = GetK3sControlPlaneByCluster(ctx, GetK3sControlPlaneByClusterInput{
			Lister:      input.Lister,
			ClusterName: input.Cluster.Name,
			Namespace:   input.Cluster.Namespace,
		})
		g.Expect(controlPlane).ToNot(BeNil())
	}, "10s", "1s").Should(Succeed(), "Couldn't get the control plane for the cluster %s", klog.KObj(input.Cluster))

	log.Logf("Waiting for the first control plane machine managed by %s to be provisioned", klog.KObj(controlPlane))
	WaitForOneK3sControlPlaneMachineToExist(ctx, WaitForOneK3sControlPlaneMachineToExistInput{
		Lister:       input.Lister,
		Cluster:      input.Cluster,
		ControlPlane: controlPlane,
	}, intervals...)

	return controlPlane
}

// WaitForControlPlaneAndMachinesReadyInput is the input type for WaitForControlPlaneAndMachinesReady.
type WaitForControlPlaneAndMachinesReadyInput struct {
	GetLister    GetLister
	Cluster      *clusterv1.Cluster
	ControlPlane *infracontrolplanev1.K3sControlPlane
}

// WaitForControlPlaneAndMachinesReady waits for a K3sControlPlane object to be ready (all the machine provisioned and one node ready).
func WaitForControlPlaneAndMachinesReady(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WaitForControlPlaneReady")
	Expect(input.GetLister).ToNot(BeNil(), "Invalid argument. input.GetLister can't be nil when calling WaitForControlPlaneReady")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling WaitForControlPlaneReady")
	Expect(input.ControlPlane).ToNot(BeNil(), "Invalid argument. input.ControlPlane can't be nil when calling WaitForControlPlaneReady")

	if input.ControlPlane.Spec.Replicas != nil && int(*input.ControlPlane.Spec.Replicas) > 1 {
		log.Logf("Waiting for the remaining control plane machines managed by %s to be provisioned", klog.KObj(input.ControlPlane))
		WaitForK3sControlPlaneMachinesToExist(ctx, WaitForK3sControlPlaneMachinesToExistInput{
			Lister:       input.GetLister,
			Cluster:      input.Cluster,
			ControlPlane: input.ControlPlane,
		}, intervals...)
	}

	log.Logf("Waiting for control plane %s to be ready (implies underlying nodes to be ready as well)", klog.KObj(input.ControlPlane))
	waitForControlPlaneToBeReadyInput := WaitForControlPlaneToBeReadyInput{
		Getter:       input.GetLister,
		ControlPlane: input.ControlPlane,
	}
	WaitForControlPlaneToBeReady(ctx, waitForControlPlaneToBeReadyInput, intervals...)

	AssertControlPlaneFailureDomains(ctx, AssertControlPlaneFailureDomainsInput{
		Lister:  input.GetLister,
		Cluster: input.Cluster,
	})
}

// UpgradeControlPlaneAndWaitForUpgradeInput is the input type for UpgradeControlPlaneAndWaitForUpgrade.
type UpgradeControlPlaneAndWaitForUpgradeInput struct {
	ClusterProxy                ClusterProxy
	Cluster                     *clusterv1.Cluster
	ControlPlane                *infracontrolplanev1.K3sControlPlane
	KubernetesUpgradeVersion    string
	UpgradeMachineTemplate      *string
	EtcdImageTag                string
	DNSImageTag                 string
	WaitForMachinesToBeUpgraded []interface{}
	WaitForDNSUpgrade           []interface{}
	WaitForKubeProxyUpgrade     []interface{}
	WaitForEtcdUpgrade          []interface{}
}

// UpgradeControlPlaneAndWaitForUpgrade upgrades a K3sControlPlane and waits for it to be upgraded.
func UpgradeControlPlaneAndWaitForUpgrade(ctx context.Context, input UpgradeControlPlaneAndWaitForUpgradeInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.ControlPlane).ToNot(BeNil(), "Invalid argument. input.ControlPlane can't be nil when calling UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.KubernetesUpgradeVersion).ToNot(BeNil(), "Invalid argument. input.KubernetesUpgradeVersion can't be empty when calling UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.EtcdImageTag).ToNot(BeNil(), "Invalid argument. input.EtcdImageTag can't be empty when calling UpgradeControlPlaneAndWaitForUpgrade")
	Expect(input.DNSImageTag).ToNot(BeNil(), "Invalid argument. input.DNSImageTag can't be empty when calling UpgradeControlPlaneAndWaitForUpgrade")

	mgmtClient := input.ClusterProxy.GetClient()

	log.Logf("Patching the new kubernetes version to KCP")
	patchHelper, err := patch.NewHelper(input.ControlPlane, mgmtClient)
	Expect(err).ToNot(HaveOccurred())

	input.ControlPlane.Spec.Version = input.KubernetesUpgradeVersion
	if input.UpgradeMachineTemplate != nil {
		input.ControlPlane.Spec.MachineTemplate.InfrastructureRef.Name = *input.UpgradeMachineTemplate
	}
	// If the ClusterConfiguration is not specified, create an empty one.
	if input.ControlPlane.Spec.K3sConfigSpec.ServerConfiguration == nil {
		input.ControlPlane.Spec.K3sConfigSpec.ServerConfiguration = new(infrabootstrapv1.ServerConfiguration)
	}

	if input.ControlPlane.Spec.K3sConfigSpec.ServerConfiguration.Database.ClusterInit == nil {
		input.ControlPlane.Spec.K3sConfigSpec.ServerConfiguration.Database.ClusterInit = pointer.Bool(true)
	}

	Eventually(func() error {
		return patchHelper.Patch(ctx, input.ControlPlane)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to patch the new kubernetes version to KCP %s", klog.KObj(input.ControlPlane))

	log.Logf("Waiting for control-plane machines to have the upgraded kubernetes version")
	WaitForControlPlaneMachinesToBeUpgraded(ctx, WaitForControlPlaneMachinesToBeUpgradedInput{
		Lister:                   mgmtClient,
		Cluster:                  input.Cluster,
		MachineCount:             int(*input.ControlPlane.Spec.Replicas),
		KubernetesUpgradeVersion: input.KubernetesUpgradeVersion,
	}, input.WaitForMachinesToBeUpgraded...)

	log.Logf("Waiting for kube-proxy to have the upgraded kubernetes version")
	workloadCluster := input.ClusterProxy.GetWorkloadCluster(ctx, input.Cluster.Namespace, input.Cluster.Name)
	workloadClient := workloadCluster.GetClient()
	WaitForKubeProxyUpgrade(ctx, WaitForKubeProxyUpgradeInput{
		Getter:            workloadClient,
		KubernetesVersion: input.KubernetesUpgradeVersion,
	}, input.WaitForKubeProxyUpgrade...)

	log.Logf("Waiting for CoreDNS to have the upgraded image tag")
	WaitForDNSUpgrade(ctx, WaitForDNSUpgradeInput{
		Getter:     workloadClient,
		DNSVersion: input.DNSImageTag,
	}, input.WaitForDNSUpgrade...)

	log.Logf("Waiting for etcd to have the upgraded image tag")
	lblSelector, err := labels.Parse("component=etcd")
	Expect(err).ToNot(HaveOccurred())
	WaitForPodListCondition(ctx, WaitForPodListConditionInput{
		Lister:      workloadClient,
		ListOptions: &client.ListOptions{LabelSelector: lblSelector},
		Condition:   EtcdImageTagCondition(input.EtcdImageTag, int(*input.ControlPlane.Spec.Replicas)),
	}, input.WaitForEtcdUpgrade...)
}

// controlPlaneMachineOptions returns a set of ListOptions that allows to get all machine objects belonging to control plane.
func controlPlaneMachineOptions() []client.ListOption {
	return []client.ListOption{
		client.HasLabels{clusterv1.MachineControlPlaneLabelName},
	}
}

// ScaleAndWaitControlPlaneInput is the input for ScaleAndWaitControlPlane.
type ScaleAndWaitControlPlaneInput struct {
	ClusterProxy        ClusterProxy
	Cluster             *clusterv1.Cluster
	ControlPlane        *infracontrolplanev1.K3sControlPlane
	Replicas            int32
	WaitForControlPlane []interface{}
}

// ScaleAndWaitControlPlane scales KCP and waits until all machines have node ref and equal to Replicas.
func ScaleAndWaitControlPlane(ctx context.Context, input ScaleAndWaitControlPlaneInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for ScaleAndWaitControlPlane")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling ScaleAndWaitControlPlane")
	Expect(input.Cluster).ToNot(BeNil(), "Invalid argument. input.Cluster can't be nil when calling ScaleAndWaitControlPlane")

	patchHelper, err := patch.NewHelper(input.ControlPlane, input.ClusterProxy.GetClient())
	Expect(err).ToNot(HaveOccurred())
	scaleBefore := pointer.Int32Deref(input.ControlPlane.Spec.Replicas, 0)
	input.ControlPlane.Spec.Replicas = pointer.Int32(input.Replicas)
	log.Logf("Scaling controlplane %s from %v to %v replicas", klog.KObj(input.ControlPlane), scaleBefore, input.Replicas)
	Eventually(func() error {
		return patchHelper.Patch(ctx, input.ControlPlane)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to scale controlplane %s from %v to %v replicas", klog.KObj(input.ControlPlane), scaleBefore, input.Replicas)

	log.Logf("Waiting for correct number of replicas to exist")
	Eventually(func() (int, error) {
		kcpLabelSelector, err := metav1.ParseToLabelSelector(input.ControlPlane.Status.Selector)
		if err != nil {
			return -1, err
		}

		selector, err := metav1.LabelSelectorAsSelector(kcpLabelSelector)
		if err != nil {
			return -1, err
		}
		machines := &clusterv1.MachineList{}
		if err := input.ClusterProxy.GetClient().List(ctx, machines, &client.ListOptions{LabelSelector: selector, Namespace: input.ControlPlane.Namespace}); err != nil {
			return -1, err
		}
		nodeRefCount := 0
		for _, machine := range machines.Items {
			if machine.Status.NodeRef != nil {
				nodeRefCount++
			}
		}
		if len(machines.Items) != nodeRefCount {
			return -1, errors.New("Machine count does not match existing nodes count")
		}
		return nodeRefCount, nil
	}, input.WaitForControlPlane...).Should(Equal(int(input.Replicas)), "Timed out waiting for %d replicas to exist for control-plane %s", int(input.Replicas), klog.KObj(input.ControlPlane))
}
