output "namespace" {
  value = kubernetes_namespace.statisticsbot
}

output "persistent_volume_claim" {
  value = kubernetes_persistent_volume_claim_v1.duckdb
}

output "external_secret" {
  value = kubernetes_manifest.external_secret.manifest
}