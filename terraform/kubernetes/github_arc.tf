resource "helm_release" "github_arc_runner_statisticsbot" {
  name       = "arc-runner-statisticsbot"
  namespace  = data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.namespace.metadata.0.name
  repository = "oci://ghcr.io/actions/actions-runner-controller-charts"
  chart      = "gha-runner-scale-set"
  version    = data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.version
  values = [templatefile("${path.module}/conf/arc-runner-values.yaml", {
    github_config_url = "https://github.com/STollenaar/statisticsBot"
    github_secret     = data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.secret_name
  })]
}
