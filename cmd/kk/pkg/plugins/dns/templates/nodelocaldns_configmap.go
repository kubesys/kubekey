/*
 Copyright 2021 The KubeSphere Authors.

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

package templates

import (
	"github.com/kubesys/kubekey/cmd/kk/pkg/utils"
	"github.com/lithammer/dedent"
	"text/template"
)

var NodeLocalDNSConfigMap = template.Must(template.New("nodelocaldns-configmap.yaml").Funcs(utils.FuncMap).Parse(
	dedent.Dedent(`---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nodelocaldns
  namespace: kube-system
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists

data:
  Corefile: |
{{- if .ExternalZones }}
{{- range .ExternalZones }}
{{ range .Zones }}{{ . | indent 4 }} {{ end}} {
        errors
        cache {{ .Cache }}
        reload
{{- if .Rewrite }}
{{- range .Rewrite }}
        rewrite {{ . }}
{{- end }}
{{- end }}
        loop
        bind 169.254.25.10
        forward . {{ range .Nameservers }} {{ . }}{{ end }}
        prometheus :9253
        log
{{- if $.DNSEtcHosts }}
        hosts /etc/coredns/hosts {
          fallthrough
        }
{{- end }}
    }
{{- end }}
{{- end }}
    {{ .DNSDomain }}:53 {
        errors
        cache {
            success 9984 30
            denial 9984 5
        }
        reload
        loop
        bind 169.254.25.10
        forward . {{ .ForwardTarget }} {
            force_tcp
        }
        prometheus :9253
        health 169.254.25.10:9254
    }
    in-addr.arpa:53 {
        errors
        cache 30
        reload
        loop
        bind 169.254.25.10
        forward . {{ .ForwardTarget }} {
            force_tcp
        }
        prometheus :9253
    }
    ip6.arpa:53 {
        errors
        cache 30
        reload
        loop
        bind 169.254.25.10
        forward . {{ .ForwardTarget }} {
            force_tcp
        }
        prometheus :9253
    }
    .:53 {
        errors
        cache 30
        reload
        loop
        bind 169.254.25.10
        forward . /etc/resolv.conf
        prometheus :9253
{{- if .DNSEtcHosts }}
        hosts /etc/coredns/hosts {
          fallthrough
        }
{{- end }}
    }
{{- if .DNSEtcHosts }}
  hosts: |
{{ .DNSEtcHosts | indent 4}}
{{- end }}
`)))
