/*
Copyright 2020 The KubeSphere Authors.

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

package delete

import (
	"github.com/spf13/cobra"

	"github.com/kubesys/kubekey/cmd/kk/cmd/options"
)

type DeleteOptions struct {
	CommonOptions *options.CommonOptions
}

func NewDeleteOptions() *DeleteOptions {
	return &DeleteOptions{
		CommonOptions: options.NewCommonOptions(),
	}
}

// NewCmdDelete creates a new delete command
func NewCmdDelete() *cobra.Command {
	o := NewDeleteOptions()
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete node or cluster",
	}

	o.CommonOptions.AddCommonFlag(cmd)

	cmd.AddCommand(NewCmdDeleteCluster())
	cmd.AddCommand(NewCmdDeleteNode())
	return cmd
}
