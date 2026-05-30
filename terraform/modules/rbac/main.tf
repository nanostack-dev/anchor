locals {
  all_permissions = sort(distinct(concat(var.permissions_admin, var.api_only_permissions)))
}

resource "nanostack_product_permission" "permission" {
  for_each = toset(local.all_permissions)

  product_id  = var.nanostack_product_id
  name        = each.value
  description = replace(title(replace(each.value, ":", " ")), "_", " ")
}

resource "nanostack_product_role" "admin" {
  product_id  = var.nanostack_product_id
  name        = "admin"
  description = "Administrator role"
  permissions = sort(var.permissions_admin)
}

resource "terraform_data" "sync_echopoint_runtime_fields" {
  count = var.sync_runtime_fields ? 1 : 0

  triggers_replace = [
    var.nanostack_product_id,
    nanostack_product_role.admin.id,
    var.product_api_key_for_sync,
  ]

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]

    command = <<-EOT
      set -euo pipefail

      runtime_env="$(op read "op://${var.op_vault_id}/${var.op_item_ref}/notesPlain" 2>/dev/null || true)"

      update_runtime_key() {
        local key="$1"
        local value="$2"

        runtime_env="$(printf '%s' "$runtime_env" | awk -v key="$key" -v value="$value" '
          BEGIN { updated = 0 }
          $0 ~ ("^" key "=") {
            print key "=" value
            updated = 1
            next
          }
          { print }
          END {
            if (!updated) {
              print key "=" value
            }
          }
        ')"
      }

      update_runtime_key "ANCHOR_PRODUCT_ID" "${var.nanostack_product_id}"
      update_runtime_key "ANCHOR_ADMIN_ROLE_ID" "${nanostack_product_role.admin.id}"
      update_runtime_key "ANCHOR_PRODUCT_API_KEY" "$ANCHOR_PRODUCT_API_KEY"

      op item edit "${var.op_item_ref}" --vault "${var.op_vault_id}" \
        "notesPlain=$runtime_env"
    EOT

    environment = {
      ANCHOR_PRODUCT_API_KEY = var.product_api_key_for_sync
    }
  }
}
