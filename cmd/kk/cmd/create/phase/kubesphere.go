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

package phase

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/kubesys/kubekey/cmd/kk/cmd/options"
	"github.com/kubesys/kubekey/cmd/kk/cmd/util"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	alpha "github.com/kubesys/kubekey/cmd/kk/pkg/phase/kubesphere"
	"github.com/kubesys/kubekey/cmd/kk/pkg/version/kubesphere"
)

type CreateKubeSphereOptions struct {
	CommonOptions    *options.CommonOptions
	ClusterCfgFile   string
	EnableKubeSphere bool
	KubeSphere       string
}

func NewCreateKubeSphereOptions() *CreateKubeSphereOptions {
	return &CreateKubeSphereOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdCreateKubeSphere creates a new CreateKubeSphere command
func NewCmdCreateKubeSphere() *cobra.Command {
	o := NewCreateKubeSphereOptions()
	cmd := &cobra.Command{
		Use:   "kubesphere",
		Short: "Install the kubesphere with the input version",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(cmd, args))
			util.CheckErr(o.Run())
		},
	}
	o.CommonOptions.AddCommonFlag(cmd)
	o.AddFlags(cmd)

	if err := ksCompletionSetting(cmd); err != nil {
		panic(fmt.Sprintf("Got error with the completion setting"))
	}
	return cmd
}

func (o *CreateKubeSphereOptions) Complete(cmd *cobra.Command, args []string) error {
	var ksVersion string
	if o.EnableKubeSphere && len(args) > 0 {
		ksVersion = args[0]
	} else {
		ksVersion = kubesphere.Latest().Version
	}
	o.KubeSphere = ksVersion
	return nil
}

func (o *CreateKubeSphereOptions) Run() error {
	arg := common.Argument{
		FilePath:         o.ClusterCfgFile,
		KsEnable:         o.EnableKubeSphere,
		KsVersion:        o.KubeSphere,
		SkipConfirmCheck: o.CommonOptions.SkipConfirmCheck,
		Debug:            o.CommonOptions.Verbose,
	}
	return alpha.CreateKubeSphere(arg)
}

func (o *CreateKubeSphereOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.ClusterCfgFile, "filename", "f", "", "Path to a configuration file")
	cmd.Flags().BoolVarP(&o.EnableKubeSphere, "with-kubesphere", "", false, fmt.Sprintf("Deploy a specific version of kubesphere (default %s)", kubesphere.Latest().Version))
}

func ksCompletionSetting(cmd *cobra.Command) (err error) {
	cmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) (
		strings []string, directive cobra.ShellCompDirective) {
		versionArray := kubesphere.VersionsStringArr()
		versionArray = append(versionArray, time.Now().Add(-time.Hour*24).Format("nightly-20060102"))
		return versionArray, cobra.ShellCompDirectiveNoFileComp
	}

	return
}
