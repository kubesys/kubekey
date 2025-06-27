/*
Copyright (2024, ) Institute of Software, Chinese Academy of Sciences

@author: liujiexin@otcaix.iscas.ac.cn

* Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
*/

package phase

import (
	"github.com/spf13/cobra"

	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/options"
	"github.com/kubesphere/kubekey/v3/cmd/kk/cmd/util"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/phase/addons"
)

type CreateAddonOptions struct {
	CommonOptions  *options.CommonOptions
	ClusterCfgFile string
	AddonName      string
}

func NewCreateAddonOptions() *CreateAddonOptions {
	return &CreateAddonOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdCreateAddon creates a new create addon command
func NewCmdCreateAddon() *cobra.Command {
	o := NewCreateAddonOptions()
	cmd := &cobra.Command{
		Use:   "addon [addon-name]",
		Short: "Install a specific addon",
		Long:  "Install a specific addon by name. If no addon name is provided, all configured addons will be installed.",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(cmd, args))
			util.CheckErr(o.Run())
		},
	}

	o.CommonOptions.AddCommonFlag(cmd)
	o.AddFlags(cmd)
	return cmd
}

func (o *CreateAddonOptions) Complete(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.AddonName = args[0]
	}
	return nil
}

func (o *CreateAddonOptions) Run() error {
	var enabledAddons []string
	if o.AddonName != "" {
		enabledAddons = []string{o.AddonName}
	}

	arg := common.Argument{
		FilePath:      o.ClusterCfgFile,
		Debug:         o.CommonOptions.Verbose,
		EnabledAddons: enabledAddons,
	}
	return addons.ApplyClusterAddons(arg)
}

func (o *CreateAddonOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ClusterCfgFile, "filename", "f", "", "Path to a configuration file")
}
