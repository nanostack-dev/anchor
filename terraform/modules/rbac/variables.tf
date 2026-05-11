variable "nanostack_product_id" {
  description = "Existing Anchor product ID managed by this stack"
  type        = string
}

variable "permissions_admin" {
  description = "Permissions granted to the admin role"
  type        = list(string)
  default     = []
}

variable "api_only_permissions" {
  description = "API-only permissions to create without granting them to the admin role"
  type        = list(string)
  default     = []
}

variable "sync_runtime_fields" {
  description = "Whether to sync NANOSTACK_* runtime fields to 1Password"
  type        = bool
  default     = false
}

variable "op_vault_id" {
  description = "1Password vault ID where runtime fields are stored"
  type        = string
  default     = "yjntbdb73no2xsinyuzpwizmpm"
}

variable "op_item_ref" {
  description = "1Password item reference containing runtime fields"
  type        = string
  default     = "anchor-infra-env"
}

variable "product_api_key_for_sync" {
  description = "Product API key value written to NANOSTACK_PRODUCT_API_KEY when sync_runtime_fields is enabled"
  type        = string
  default     = ""
  sensitive   = true
}
