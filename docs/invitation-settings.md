# Invitation settings and Anchor delivery

Each Product has four invitation settings. A Platform User reads and replaces them with a Platform Bearer token at `GET` and `PUT /v1/products/{product_id}/invitation-settings`. A Product API key cannot read or change them. A change emits no Product event.

| Setting | Values | Default |
| --- | --- | --- |
| `invitation_delivery` | `anchor`, `product` | `product` |
| `email_template_id` | An email template of the Product, or null | null |
| `accept_url_template` | A link that contains `{token}`, or null | null |
| `default_expiry_seconds` | 3600 to 7776000 (one hour to 90 days) | 604800 (7 days) |

A Product with no stored settings reads the defaults. `PUT` replaces all four values.

## Default expiry

Create and resend use the default expiry when the caller gives no expiry. A create call with `expires_at` uses it.

## Anchor delivery

Anchor delivery is allowed only when all three conditions hold:

- the SMTP integration of the Product is active (`smtp_integration_active`),
- an email template is chosen and exists (`email_template_set`),
- an accept URL template is set (`accept_url_template_set`).

A `PUT` that chooses Anchor delivery while a condition is false fails with `409 ORGANIZATION_INVITATION_SETTINGS_ANCHOR_DELIVERY_UNAVAILABLE`. The error metadata `unmet_conditions` lists every false condition.

Under Anchor delivery, create and resend send one email to the invitation address through the existing email service, as the last step of the call and inside its transaction. The email service applies the published version of the chosen template.

| Variable | Value |
| --- | --- |
| `accept_url` | The accept URL template with `{token}` replaced by the URL-encoded token |
| `organization_name` | The name of the Organization |
| `role_name` | The name of the role of the invitation |
| `invitee_email` | The invitation address |
| `expires_at` | The expiry, RFC 3339 in UTC |

Failures:

- A condition became false after the settings were saved: create and resend fail with `409 ORGANIZATION_INVITATION_ANCHOR_DELIVERY_UNAVAILABLE`. Anchor does not fall back to Product delivery.
- The send fails: create and resend fail with `500 ORGANIZATION_INVITATION_EMAIL_SEND_FAILED`, and Anchor logs the failure. A failed create leaves no invitation. A failed resend keeps the old token valid with its old expiry. Anchor stores no delivery state.

Create and resend return the token in both modes. Under Product delivery Anchor sends no email.
