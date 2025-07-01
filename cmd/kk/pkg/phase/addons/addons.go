package addons

import (
	"github.com/pkg/errors"

	"github.com/kubesys/kubekey/cmd/kk/pkg/addons"
	"github.com/kubesys/kubekey/cmd/kk/pkg/bootstrap/precheck"
	"github.com/kubesys/kubekey/cmd/kk/pkg/common"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/module"
	"github.com/kubesys/kubekey/cmd/kk/pkg/core/pipeline"
	"github.com/kubesys/kubekey/cmd/kk/pkg/kubernetes"
)

func ApplyClusterAddons(args common.Argument) error {

	var loaderType string
	if args.FilePath != "" {
		loaderType = common.File
	} else {
		loaderType = common.AllInOne
	}

	runtime, err := common.NewKubeRuntime(loaderType, args)
	if err != nil {
		return err
	}

	switch runtime.Cluster.Kubernetes.Type {
	case common.Kubernetes:
		if err := ApplyClusterAddonsPipeline(runtime); err != nil {
			return err
		}
	default:
		return errors.New("unsupported cluster kubernetes type")
	}

	return nil
}

func ApplyClusterAddonsPipeline(runtime *common.KubeRuntime) error {
	m := []module.Module{
		&precheck.GreetingsModule{},
		&precheck.NodePreCheckModule{},
		&kubernetes.StatusModule{},
		&addons.AddonsModule{},
	}

	p := pipeline.Pipeline{
		Name:    "ApplyClusterAddonsPipeline",
		Modules: m,
		Runtime: runtime,
	}
	if err := p.Start(); err != nil {
		return err
	}
	return nil
}
