#!/usr/bin/env bash
## run whatever auth you need to connect
port=8200
service="service/vault"
config="$1"
echo -en 'kubectl port-forward --kubeconfig=$config -n vault $service $port:$port\n disown' >>run_service.sh && chmod +x run_service.sh
config=$config service=$service port=$port nohup bash run_service.sh </dev/null >/dev/null 2>&1 &

sleep 30s
# sleep was to give it a chance to finish establishing the conneciont before terragrunt/terraform starts running
