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

package cri

import (
	"github.com/spf13/cobra"

	"github.com/kubesys/kubekey/cmd/kk/cmd/options"
)

type MigrateOptions struct {
	CommonOptions *options.CommonOptions
}

func NewMigrateOptions() *MigrateOptions {
	return &MigrateOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdCri creates a new Migrate command
func NewCmdCri() *cobra.Command {
	o := NewMigrateOptions()
	cmd := &cobra.Command{
		Use:   "cri",
		Short: "cri",
	}

	o.CommonOptions.AddCommonFlag(cmd)
	cmd.AddCommand(NewCmdMigrateCri())
	return cmd
}
