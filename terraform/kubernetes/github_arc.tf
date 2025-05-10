resource "helm_release" "github_arc_runner_statisticsbot" {
  name       = "arc-runner-statisticsbot"
  namespace  = data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.namespace.metadata.0.name
  repository = "oci://ghcr.io/actions/actions-runner-controller-charts"
  chart      = "gha-runner-scale-set"
  version    =  data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.version
  values     = []

  set {
    name  = "githubConfigUrl"
    value = "https://github.com/STollenaar/statisticsBot"
  }
  set {
    name  = "githubConfigSecret"
    value =  data.terraform_remote_state.kubernetes_cluster.outputs.github_arc.secret_name
  }
  set {
    name = "containerMode.type"
    value = "dind"
  }
}
