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
  description = "Whether to sync ANCHOR_* values into the target 1Password runtime note"
  type        = bool
  default     = false
}

variable "op_vault_id" {
  description = "1Password vault ID where the target runtime item is stored"
  type        = string
  default     = "d6744hn5rykbbbynw6zgm2ttmy"
}

variable "op_item_ref" {
  description = "1Password item reference containing the runtime note to update"
  type        = string
  default     = "echopoint-prod-runtime"
}

variable "product_api_key_for_sync" {
  description = "Product API key value written into ANCHOR_PRODUCT_API_KEY inside the runtime note when sync_runtime_fields is enabled"
  type        = string
  default     = ""
  sensitive   = true
}
