import { StripeIntegrationAuthMethod } from "@/client";
import { BillingSelect } from "@/features/billing/billing-fields";
import {
	Box,
	Field,
	FieldDescription,
	FieldError,
	FieldLabel,
	Input,
	Stack,
	Switch,
	Text,
} from "@nanostackorg/design-system";

export interface StripeFormState {
	authMethod: StripeIntegrationAuthMethod;
	accountId: string;
	apiKey: string;
	webhookSecret: string;
	returnUrl: string;
	enabled: boolean;
}

export function StripeConfigForm({
	form,
	setField,
	errors,
	disabled,
	existing,
	apiKeyConfigured,
	webhookSecretConfigured,
}: {
	form: StripeFormState;
	setField: <K extends keyof StripeFormState>(
		key: K,
		value: StripeFormState[K],
	) => void;
	errors: Partial<Record<keyof StripeFormState, string>>;
	disabled: boolean;
	existing: boolean;
	apiKeyConfigured: boolean;
	webhookSecretConfigured: boolean;
}) {
	return (
		<Stack space="lg">
			<BillingSelect
				label="Connection method"
				value={form.authMethod}
				onChange={(value) =>
					setField(
						"authMethod",
						value === StripeIntegrationAuthMethod.LOCAL_CLI
							? StripeIntegrationAuthMethod.LOCAL_CLI
							: StripeIntegrationAuthMethod.API_KEY,
					)
				}
				options={[
					{
						value: StripeIntegrationAuthMethod.API_KEY,
						label: "Stripe test API key",
					},
					{
						value: StripeIntegrationAuthMethod.LOCAL_CLI,
						label: "Connected Stripe CLI (local sandbox)",
					},
				]}
				disabled={disabled}
				description={
					form.authMethod === StripeIntegrationAuthMethod.LOCAL_CLI
						? "Uses the Stripe CLI account already authorized on the local backend. Requires a localhost return address."
						: "Connect this product using an encrypted Stripe sandbox key."
				}
			/>
			<Field invalid={!!errors.accountId}>
				<FieldLabel htmlFor="stripe-account">Stripe account ID</FieldLabel>
				<Input
					id="stripe-account"
					font="mono"
					value={form.accountId}
					onChange={(event) => setField("accountId", event.target.value)}
					placeholder="acct_…"
					disabled={disabled}
					aria-invalid={!!errors.accountId}
				/>
				<FieldError>{errors.accountId}</FieldError>
			</Field>
			{form.authMethod === StripeIntegrationAuthMethod.API_KEY && (
				<Field invalid={!!errors.apiKey}>
					<FieldLabel htmlFor="stripe-api-key">Stripe test API key</FieldLabel>
					<Input
						id="stripe-api-key"
						type="password"
						autoComplete="new-password"
						value={form.apiKey}
						onChange={(event) => setField("apiKey", event.target.value)}
						placeholder={
							apiKeyConfigured
								? "Saved · leave blank to keep"
								: "sk_test_… or rk_test_…"
						}
						disabled={disabled}
						aria-invalid={!!errors.apiKey}
					/>
					<FieldDescription>
						{apiKeyConfigured
							? "A key is saved. Leave this blank to keep it, or enter a replacement."
							: "Use a test secret or restricted key. Live keys are not accepted."}
					</FieldDescription>
					<FieldError>{errors.apiKey}</FieldError>
				</Field>
			)}
			<Field invalid={!!errors.webhookSecret}>
				<FieldLabel htmlFor="stripe-webhook-secret">
					Webhook signing secret
				</FieldLabel>
				<Input
					id="stripe-webhook-secret"
					type="password"
					autoComplete="new-password"
					value={form.webhookSecret}
					onChange={(event) => setField("webhookSecret", event.target.value)}
					placeholder={
						webhookSecretConfigured ? "Saved · leave blank to keep" : "whsec_…"
					}
					disabled={disabled}
					aria-invalid={!!errors.webhookSecret}
				/>
				<FieldDescription>
					{webhookSecretConfigured
						? "A signing secret is saved. Leave this blank to keep it."
						: "Use the signing secret for this product’s Stripe webhook endpoint."}
				</FieldDescription>
				<FieldError>{errors.webhookSecret}</FieldError>
			</Field>
			<Field invalid={!!errors.returnUrl}>
				<FieldLabel htmlFor="stripe-return-url">
					Anchor return address
				</FieldLabel>
				<Input
					id="stripe-return-url"
					type="url"
					value={form.returnUrl}
					onChange={(event) => setField("returnUrl", event.target.value)}
					disabled={disabled}
					aria-invalid={!!errors.returnUrl}
				/>
				<FieldDescription>
					Anchor’s base address. Checkout and the customer portal return to the
					organization’s Billing tab.
				</FieldDescription>
				<FieldError>{errors.returnUrl}</FieldError>
			</Field>
			{existing && (
				<Box className="flex items-center justify-between gap-4 rounded-lg border border-border bg-muted/20 p-4">
					<Stack space="xs">
						<FieldLabel htmlFor="stripe-enabled">Enabled</FieldLabel>
						<Text size="sm" tone="muted">
							Pause billing operations and webhook ingestion.
						</Text>
					</Stack>
					<Switch
						id="stripe-enabled"
						checked={form.enabled}
						onCheckedChange={(checked) => setField("enabled", checked)}
						disabled={disabled}
					/>
				</Box>
			)}
		</Stack>
	);
}
