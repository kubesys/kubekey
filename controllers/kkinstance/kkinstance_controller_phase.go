/*
 Copyright 2022 The kubesys Authors.

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

package kkinstance

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	infrav1 "github.com/kubesys/kubekey/api/v1beta1"
	"github.com/kubesys/kubekey/pkg/clients/ssh"
	"github.com/kubesys/kubekey/pkg/scope"
	"github.com/kubesys/kubekey/pkg/service"
)

func (r *Reconciler) phaseFactory(kkInstanceScope scope.KKInstanceScope) []func(context.Context, ssh.Interface,
	*scope.InstanceScope, scope.KKInstanceScope, scope.LBScope) error {
	var phases []func(context.Context, ssh.Interface, *scope.InstanceScope, scope.KKInstanceScope, scope.LBScope) error
	switch kkInstanceScope.Distribution() {
	case infrav1.KUBERNETES:
		phases = append(phases,
			r.reconcileBootstrap,
			r.reconcileRepository,
			r.reconcileBinaryService,
			r.reconcileContainerManager,
			r.reconcileProvisioning,
		)
	case infrav1.K3S:
		phases = append(phases,
			r.reconcileBootstrap,
			r.reconcileRepository,
			r.reconcileBinaryService,
			r.reconcileProvisioning,
		)
	}
	return phases
}

func (r *Reconciler) reconcilePing(_ context.Context, instanceScope *scope.InstanceScope) error {
	instanceScope.Info("Reconcile ping")

	sshClient := r.getSSHClient(instanceScope)
	var err error
	for i := 0; i < 3; i++ {
		err = sshClient.Ping()
		if err == nil {
			break
		}
	}
	return err
}

func (r *Reconciler) reconcileDeletingBootstrap(_ context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	lbScope scope.LBScope) (err error) {
	instanceScope.Info("Reconcile deleting bootstrap")

	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceDeletingBootstrapCondition,
				infrav1.KKInstanceClearEnvironmentFailedReason,
				clusterv1.ConditionSeverityWarning,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceDeletingBootstrapCondition)
		}
	}()

	instanceScope.SetState(infrav1.InstanceStateCleaning)

	svc := r.getBootstrapService(sshClient, lbScope, instanceScope)
	if err := svc.KubeadmReset(instanceScope.ContainerManager().CRISocket); err != nil {
		return err
	}
	if err := svc.ResetNetwork(); err != nil {
		return err
	}
	if err := svc.RemoveFiles(); err != nil {
		return err
	}
	if err := svc.DaemonReload(); err != nil {
		return err
	}
	if err := svc.UninstallK3s(); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileBootstrap(_ context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	_ scope.KKInstanceScope, lbScope scope.LBScope) (err error) {
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceBootstrappedCondition,
				infrav1.KKInstanceInitOSFailedReason,
				clusterv1.ConditionSeverityWarning,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceBootstrappedCondition)
		}
	}()
	if conditions.IsTrue(instanceScope.KKInstance, infrav1.KKInstanceBootstrappedCondition) {
		instanceScope.Info("Instance has been bootstrapped")
		return nil
	}

	instanceScope.Info("Reconcile bootstrap")

	instanceScope.SetState(infrav1.InstanceStateBootstrapping)

	svc := r.getBootstrapService(sshClient, lbScope, instanceScope)

	if err := svc.AddUsers(); err != nil {
		return err
	}
	if err := svc.SetHostname(); err != nil {
		return err
	}
	if err := svc.CreateDirectory(); err != nil {
		return err
	}
	if err := svc.ResetTmpDirectory(); err != nil {
		return err
	}
	if err := svc.ExecInitScript(); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileRepository(_ context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	scope scope.KKInstanceScope, _ scope.LBScope) (err error) {
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceRepositoryReadyCondition,
				infrav1.KKInstanceRepositoryFailedReason,
				clusterv1.ConditionSeverityWarning,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceRepositoryReadyCondition)
		}
	}()
	if conditions.IsTrue(instanceScope.KKInstance, infrav1.KKInstanceRepositoryReadyCondition) {
		instanceScope.Info("Instance has been repository ready")
		return nil
	}

	instanceScope.Info("Reconcile repository")

	svc := r.getRepositoryService(sshClient, scope, instanceScope)
	if err = svc.Check(); err != nil {
		return err
	}
	if err = svc.Get(r.WaitKKInstanceTimeout); err != nil {
		return err
	}

	err = svc.MountISO()
	defer func(svc service.Repository) {
		_ = svc.UmountISO()
	}(svc)
	if err != nil {
		return err
	}

	if err = svc.UpdateAndInstall(); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileBinaryService(_ context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	kkInstanceScope scope.KKInstanceScope, _ scope.LBScope) (err error) {
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceBinariesReadyCondition,
				infrav1.KKInstanceGetBinaryFailedReason,
				clusterv1.ConditionSeverityError,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceBinariesReadyCondition)
		}
	}()
	if conditions.IsTrue(instanceScope.KKInstance, infrav1.KKInstanceBinariesReadyCondition) {
		instanceScope.Info("Instance's binaries is already ready")
		return nil
	}

	instanceScope.Info("Reconcile binary service")

	svc := r.getBinaryService(sshClient, kkInstanceScope, instanceScope, kkInstanceScope.Distribution())
	if err := svc.Download(r.WaitKKInstanceTimeout); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileContainerManager(_ context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	scope scope.KKInstanceScope, _ scope.LBScope) (err error) {
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceCRIReadyCondition,
				infrav1.KKInstanceInstallCRIFailedReason,
				clusterv1.ConditionSeverityError,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceCRIReadyCondition)
		}
	}()

	if conditions.IsTrue(instanceScope.KKInstance, infrav1.KKInstanceCRIReadyCondition) {
		instanceScope.Info("Instance's CRI is already ready")
		return nil
	}

	instanceScope.Info("Reconcile container manager")

	svc := r.getContainerManager(sshClient, scope, instanceScope)
	if svc.IsExist() {
		instanceScope.V(2).Info(fmt.Sprintf("container manager %s is exist, skip installation", svc.Type()))
		return nil
	}

	if err := svc.Get(r.WaitKKInstanceTimeout); err != nil {
		return err
	}
	if err := svc.Install(); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcileProvisioning(ctx context.Context, sshClient ssh.Interface, instanceScope *scope.InstanceScope,
	_ scope.KKInstanceScope, _ scope.LBScope) (err error) {
	defer func() {
		if err != nil {
			conditions.MarkFalse(
				instanceScope.KKInstance,
				infrav1.KKInstanceProvisionedCondition,
				infrav1.KKInstanceRunCloudConfigFailedReason,
				clusterv1.ConditionSeverityError,
				err.Error(),
			)
		} else {
			conditions.MarkTrue(instanceScope.KKInstance, infrav1.KKInstanceProvisionedCondition)
		}
	}()

	if conditions.IsTrue(instanceScope.KKInstance, infrav1.KKInstanceProvisionedCondition) {
		instanceScope.Info("Instance has been provisioned")
		return nil
	}

	instanceScope.Info("Reconcile provisioning")

	bootstrapData, format, err := instanceScope.GetRawBootstrapDataWithFormat(ctx)
	if err != nil {
		instanceScope.Error(err, "failed to get bootstrap data")
		r.Recorder.Event(instanceScope.KKInstance, corev1.EventTypeWarning, "FailedGetBootstrapData", err.Error())
		return err
	}

	svc := r.getProvisioningService(sshClient, format)

	commands, err := svc.RawBootstrapDataToProvisioningCommands(bootstrapData)
	if err != nil {
		instanceScope.Error(err, "provisioning code failed to parse", "bootstrap-data", base64.StdEncoding.EncodeToString(bootstrapData))
		return errors.Wrap(err, "failed to join a control plane node with kubeadm")
	}

	for _, command := range commands {
		if _, err := sshClient.SudoCmd(command.String()); err != nil {
			return errors.Wrapf(err, "failed to run cloud config")
		}
	}
	return nil
}
