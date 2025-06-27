# KubeKey Addon 命令使用说明

## 概述

新增的 `kk create phase addon` 命令允许您单独安装特定的 addon，而不需要重新安装整个集群。

## 命令语法

```bash
./kk create phase addon [addon-name] [flags]
```

## 参数说明

- `addon-name`: 可选参数，指定要安装的 addon 名称。如果不提供，将安装配置文件中的所有 addon。
- `-f, --filename`: 指定集群配置文件路径
- `--debug`: 打印详细信息
- `-y, --yes`: 跳过确认检查
- `--ignore-err`: 忽略错误信息，强制继续
- `--namespace`: 指定 KubeKey 命名空间（默认：kubekey-system）

## 使用示例

### 1. 安装所有配置的 addon

```bash
./kk create phase addon -f example-addon-config.yaml
```

### 2. 安装特定的 addon

```bash
./kk create phase addon nfs-client -f example-addon-config.yaml
```

### 3. 使用调试模式安装 addon

```bash
./kk create phase addon snapshot-controller -f example-addon-config.yaml --debug
```

### 4. 跳过确认直接安装

```bash
./kk create phase addon nfs-client -f example-addon-config.yaml -y
```

## 配置文件示例

配置文件中的 addon 部分示例：

```yaml
addons:
- name: nfs-client
  namespace: kube-system
  sources:
    chart:
      name: nfs-subdir-external-provisioner
      repo: https://kubernetes-sigs.github.io/nfs-subdir-external-provisioner/
      values:
      - nfs.server=172.16.0.2
      - nfs.path=/mnt/demo
      - storageClass.defaultClass=false
- name: snapshot-controller
  namespace: kube-system
  sources:
    yaml:
      path:
      - https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.0/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
      - https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.0/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
```

## 支持的 Addon 类型

1. **Helm Chart**: 通过 `sources.chart` 配置
2. **YAML 清单**: 通过 `sources.yaml` 配置

## 注意事项

1. 确保集群已经正常运行
2. 确保有足够的权限访问 Kubernetes 集群
3. 如果指定的 addon 名称在配置文件中不存在，命令会报错
4. 命令会复用现有的 addon 安装逻辑，确保与完整集群安装的一致性

## 错误处理

如果安装过程中遇到错误：

1. 检查配置文件格式是否正确
2. 确认 addon 名称是否存在于配置文件中
3. 检查网络连接和权限设置
4. 使用 `--debug` 参数获取详细错误信息
