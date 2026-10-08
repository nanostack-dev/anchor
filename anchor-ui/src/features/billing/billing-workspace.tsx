import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Alert,
	AlertDescription,
	AlertTitle,
	Badge,
	Button,
	ButtonLink,
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
	Inline,
	Skeleton,
	Stack,
	Text,
} from "@nanostackorg/design-system";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { BillingActivity } from "./billing-activity";
import { type BillingAPI, billingError } from "./billing-api";
import { OrganizationBilling } from "./organization-workspace";
import { PricingWorkspace, type RunBillingAction } from "./pricing-workspace";

export function BillingWorkspace({
	api,
	productId,
	organizationId,
	onLicenseChanged,
}: {
	api: BillingAPI;
	productId: string;
	organizationId?: string;
	onLicenseChanged?: () => void;
}) {
	const [notice, setNotice] = useState("");
	const [actionError, setActionError] = useState("");
	const [actionRunning, setActionRunning] = useState(false);
	const stateQuery = useQuery({
		queryKey: ["product-stripe-billing", productId],
		queryFn: () => api.load(),
		retry: false,
		refetchOnWindowFocus: true,
		refetchInterval: 5000,
	});
	const mutation = useMutation({
		mutationFn: (action: () => Promise<unknown>) => action(),
	});
	const run: RunBillingAction = async (action, message) => {
		if (stateQuery.error) {
			setActionError("Reload billing successfully before making changes.");
			return false;
		}
		setActionRunning(true);
		setActionError("");
		setNotice("");
		try {
			await mutation.mutateAsync(action);
			const readBack = await stateQuery.refetch();
			if (readBack.error) {
				setActionError(
					"The change completed, but refreshed state could not be loaded. Reload to verify it.",
				);
				return false;
			}
			setNotice(message);
			return true;
		} catch (error) {
			setActionError(billingError(error));
			return false;
		} finally {
			setActionRunning(false);
		}
	};
	const state = stateQuery.data;
	const organization = state?.organizations.find(
		(item) => item.id === organizationId,
	);
	const licenseVersion = organization
		? JSON.stringify([organization.template_id, organization.license_values])
		: "";
	const previousLicenseVersion = useRef("");
	useEffect(() => {
		if (!licenseVersion) return;
		if (
			previousLicenseVersion.current &&
			previousLicenseVersion.current !== licenseVersion
		)
			onLicenseChanged?.();
		previousLicenseVersion.current = licenseVersion;
	}, [licenseVersion, onLicenseChanged]);
	const pending = actionRunning || stateQuery.isPending;
	const writesDisabled = pending || !!stateQuery.error;
	const integrationHref = ROUTE_PATHS.PRODUCT_INTEGRATION_STRIPE.replace(
		"$productId",
		encodeURIComponent(productId),
	);

	if (state && state.product.id !== productId) {
		return (
			<Alert tone="critical">
				<AlertTitle>Billing state does not match this product</AlertTitle>
				<AlertDescription>
					Reload the selected product before managing its billing.
				</AlertDescription>
			</Alert>
		);
	}

	return (
		<Stack space="lg">
			<Inline>
				<Inline>
					<Badge tone="warning">Stripe sandbox</Badge>
					{state && <Text tone="muted">{state.account.name}</Text>}
				</Inline>
				<Inline>
					<ButtonLink href={integrationHref}>Stripe integration</ButtonLink>
					<Button
						size="sm"
						disabled={pending}
						loading={stateQuery.isFetching}
						onClick={() => void stateQuery.refetch()}
					>
						Reload billing
					</Button>
				</Inline>
			</Inline>
			{stateQuery.isPending && (
				<Stack space="lg">
					<Skeleton height="xl" />
					<Skeleton height="xxl" />
				</Stack>
			)}
			{stateQuery.error && (
				<Alert tone="critical">
					<AlertTitle>Couldn’t load billing</AlertTitle>
					<AlertDescription>
						<Stack>
							<Text>{billingError(stateQuery.error)}</Text>
							<Inline>
								<Button
									disabled={stateQuery.isFetching}
									onClick={() => void stateQuery.refetch()}
								>
									Try again
								</Button>
							</Inline>
						</Stack>
					</AlertDescription>
				</Alert>
			)}
			{actionError && (
				<Alert tone="critical">
					<AlertTitle>Action needs attention</AlertTitle>
					<AlertDescription>{actionError}</AlertDescription>
				</Alert>
			)}
			{notice && (
				<Alert tone="success">
					<AlertTitle>Saved</AlertTitle>
					<AlertDescription>{notice}</AlertDescription>
				</Alert>
			)}
			{state && !organizationId && (
				<PricingWorkspace
					state={state}
					api={api}
					run={run}
					disabled={writesDisabled}
					loading={actionRunning}
				/>
			)}
			{state && organizationId && organization && (
				<OrganizationBilling
					key={organization.id}
					state={state}
					organization={organization}
					api={api}
					run={run}
					disabled={writesDisabled}
				/>
			)}
			{state && organizationId && !organization && (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Organization not found</EmptyTitle>
						<EmptyDescription>
							This organization does not belong to the selected product.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
			{state && (
				<BillingActivity state={state} organizationId={organizationId} />
			)}
		</Stack>
	);
}
