module "rbac" {
  source = "../rbac"

  nanostack_product_id = var.nanostack_product_id
  permissions_admin    = var.permissions_admin
  api_only_permissions = var.api_only_permissions
  sync_runtime_fields  = var.sync_runtime_fields
  op_vault_id          = var.op_vault_id
  op_item_ref          = var.op_item_ref

  product_api_key_for_sync = var.product_api_key_for_sync
}
