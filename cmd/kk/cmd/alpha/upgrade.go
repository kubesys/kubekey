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

package alpha

import (
	"github.com/spf13/cobra"

	"github.com/kubesys/kubekey/cmd/kk/cmd/options"
	"github.com/kubesys/kubekey/cmd/kk/cmd/upgrade/phase"
)

type UpgradeOptions struct {
	CommonOptions *options.CommonOptions
}

func NewUpgradeOptions() *UpgradeOptions {
	return &UpgradeOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdUpgrade creates a new upgrade command
func NewCmdUpgrade() *cobra.Command {
	o := NewUpgradeOptions()
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade your cluster by phase cmd for testing",
	}
	o.CommonOptions.AddCommonFlag(cmd)
	cmd.AddCommand(phase.NewPhaseCommand())
	return cmd
}
