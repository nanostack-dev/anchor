import "./resource-type-canvas.css";

import {
	type AccessEvaluationResponse,
	AccessSubjectType,
	type OrganizationMemberResponse,
	type ProductOrganizationResponse,
	type ProductResourcePermissionResponse,
	type ResourceResponse,
	ResourceSortField,
	evaluateAccess,
	getResource,
	searchOrganizationMembers,
	searchProductOrganizations,
	searchProductResourcePermissions,
	searchResources,
} from "@/client";
import { apiErrorHasCode, getErrorDetail } from "@/lib/api-error";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Field,
	FieldDescription,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	CheckCircleIcon,
	CloudSlashIcon,
	ProhibitIcon,
	WarningCircleIcon,
} from "@phosphor-icons/react";
import { useMutation } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { ResourceTypeSelect } from "./ResourceTypeSelect";
import { SEARCH_PAGE, SearchPicker } from "./SearchPicker";
import {
	type AccessHighlight,
	type Chain,
	type ChainLink,
	type CheckQuestion,
	type Explanation,
	ORGANIZATION,
	explain,
	highlightOf,
	subjectLink,
	typeName,
	walkChain,
} from "./access-explainer";
import type { TreeType } from "./resource-type-tree";

const UNAVAILABLE = "ACCESS_DECISIONS_UNAVAILABLE";

export interface CheckResult {
	question: CheckQuestion;
	decision: AccessEvaluationResponse;
	chain: Chain | null;
}

function memberLabel(member: OrganizationMemberResponse) {
	return member.name?.trim() || member.email;
}

function PathLine({
	links,
	complete,
	grants,
	types,
}: {
	links: ChainLink[];
	complete: boolean;
	grants: Map<string, string[]>;
	types: TreeType[];
}) {
	return (
		<Box
			as="ol"
			aria-label="Path from the resource to the Organization"
			className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-foreground text-sm"
		>
			{links.map((link, index) => {
				const holders = grants.get(`${link.type}:${link.id}`) ?? [];
				return (
					<Box
						as="li"
						key={`${link.type}:${link.id}`}
						className="inline-flex min-w-0 max-w-full flex-wrap items-center gap-1.5"
					>
						<Box as="span" className="min-w-0 [overflow-wrap:anywhere]">
							{index > 0 && (
								<Box as="span" aria-hidden className="text-muted-foreground">
									{"→ "}
								</Box>
							)}
							<Box as="span" className="font-medium">
								{link.name}
							</Box>
							<Box as="span" className="text-muted-foreground">
								{` (${typeName(types, link.type)})`}
							</Box>
						</Box>
						{holders.map((holder) => (
							<Box
								as="span"
								key={holder}
								className="inline-flex min-w-0 max-w-full items-center gap-1 rounded-full bg-success/10 px-2 py-0.5 font-medium text-success-on-tint text-xs"
							>
								<Box as="span" aria-hidden>
									◀
								</Box>
								<Box as="span" className="sr-only">
									granted by
								</Box>
								<Box as="span" className="min-w-0 [overflow-wrap:anywhere]">
									{holder}
								</Box>
							</Box>
						))}
					</Box>
				);
			})}
			{!complete && (
				<Box as="li" className="inline-flex items-center gap-1.5">
					<Box as="span" aria-hidden className="text-muted-foreground">
						→
					</Box>
					<Text as="span" tone="muted" size="sm">
						more levels above
					</Text>
				</Box>
			)}
		</Box>
	);
}

export function AccessDecision({
	result,
	explanation,
	types,
}: {
	result: CheckResult;
	explanation: Explanation;
	types: TreeType[];
}) {
	const { question, chain } = result;
	const subject = subjectLink(question);
	const links = chain?.links ?? [subject];
	const grants = new Map<string, string[]>();
	if (explanation.decision === "allowed")
		for (const grant of explanation.grants) {
			const key = `${grant.link.type}:${grant.link.id}`;
			grants.set(key, [...(grants.get(key) ?? []), grant.holder]);
		}
	const path = (
		<Stack space="xs">
			<Text weight="medium" size="sm">
				Path, highlighted on the canvas
			</Text>
			<PathLine
				links={links}
				complete={chain?.complete ?? true}
				grants={grants}
				types={types}
			/>
			{!chain && (
				<Text tone="muted" size="sm">
					Anchor didn't return the resource's ancestors, so only its own type is
					highlighted.
				</Text>
			)}
		</Stack>
	);

	if (explanation.decision === "allowed")
		return (
			<Box className="access-result">
				<Stack space="md">
					{/* biome-ignore lint/a11y/useSemanticElements: the shared Alert owns the region; a decision is announced politely, not as an alert */}
					<Alert tone="success" icon={CheckCircleIcon} role="status">
						<AlertTitle>Allowed</AlertTitle>
						<AlertDescription>
							<Stack space="sm">
								<Box as="span" className="[overflow-wrap:anywhere]">
									{`${question.member.label} has ${question.permission.name} on ${subject.name}.`}
								</Box>
								{explanation.grants.length > 0 ? (
									<Stack as="ul" space="xs" aria-label="Granted by">
										{explanation.grants.map((grant) => (
											<Box
												as="li"
												key={`${grant.holder}-${grant.link.type}-${grant.link.id}`}
												className="[overflow-wrap:anywhere]"
											>
												<Box as="span" className="font-medium">
													{grant.holder}
												</Box>
												{` ${grant.kind === "api_key" ? "API key" : "role"} on ${typeName(types, grant.link.type)} ${grant.link.name}`}
											</Box>
										))}
									</Stack>
								) : (
									<Box as="span">
										Anchor couldn't name the role yet: a change is still
										reaching the decisions. Check again in a moment.
									</Box>
								)}
							</Stack>
						</AlertDescription>
					</Alert>
					{path}
				</Stack>
			</Box>
		);

	return (
		<Box className="access-result">
			<Stack space="md">
				{/* biome-ignore lint/a11y/useSemanticElements: the shared Alert owns the region; a decision is announced politely, not as an alert */}
				<Alert tone="critical" icon={ProhibitIcon} role="status">
					<AlertTitle>Denied</AlertTitle>
					<AlertDescription>
						<Stack space="xs">
							<Box as="span" className="[overflow-wrap:anywhere]">
								{explanation.title}
							</Box>
							<Box as="span" className="[overflow-wrap:anywhere]">
								{explanation.detail}
							</Box>
							<Box as="span" className="font-mono text-xs">
								{explanation.reason}
							</Box>
						</Stack>
					</AlertDescription>
				</Alert>
				{path}
			</Stack>
		</Box>
	);
}

export interface CheckAccessPanelProps {
	productId: string;
	types: TreeType[];
	onHighlight: (highlight: AccessHighlight | null) => void;
}

export function CheckAccessPanel({
	productId,
	types,
	onHighlight,
}: CheckAccessPanelProps) {
	const [organization, setOrganization] =
		useState<ProductOrganizationResponse | null>(null);
	const [member, setMember] = useState<OrganizationMemberResponse | null>(null);
	const [permission, setPermission] =
		useState<ProductResourcePermissionResponse | null>(null);
	const [resourceType, setResourceType] = useState("");
	const [resource, setResource] = useState<ResourceResponse | null>(null);

	const check = useMutation({
		mutationFn: async (question: CheckQuestion): Promise<CheckResult> => {
			const organizationId = question.organization.id;
			const subject = question.resource;
			const [decision, chain] = await Promise.allSettled([
				evaluateAccess({
					path: { product_id: productId },
					body: {
						subject: {
							type: AccessSubjectType.USER,
							id: question.member.externalId,
						},
						action: { name: question.permission.name },
						resource: subject
							? {
									type: subject.type,
									id: subject.id,
									properties: { organization_id: organizationId },
								}
							: { type: ORGANIZATION, id: organizationId },
						options: { explain: true },
					},
					throwOnError: true,
				}),
				walkChain(question, async (parent) => {
					const { data } = await getResource({
						path: {
							product_id: productId,
							organization_id: organizationId,
							resource_type: parent.type,
							resource_id: parent.id,
						},
						throwOnError: true,
					});
					return data;
				}),
			]);
			if (decision.status === "rejected") throw decision.reason;
			return {
				question,
				decision: decision.value.data,
				chain: chain.status === "fulfilled" ? chain.value : null,
			};
		},
	});

	const explanation = check.data
		? explain(
				check.data.decision,
				check.data.question,
				check.data.chain?.links ?? [subjectLink(check.data.question)],
				types,
			)
		: null;
	const highlight =
		explanation && check.data
			? highlightOf(
					explanation,
					check.data.chain?.links ?? [subjectLink(check.data.question)],
				)
			: null;
	const highlightKey = highlight ? JSON.stringify(highlight) : "";
	const latestHighlight = useRef(highlight);
	latestHighlight.current = highlight;
	// biome-ignore lint/correctness/useExhaustiveDependencies: the key captures the highlight's content
	useEffect(() => {
		onHighlight(latestHighlight.current);
	}, [highlightKey, onHighlight]);
	useEffect(() => () => onHighlight(null), [onHighlight]);

	const changed = () => check.reset();
	const choosesOrganization = resourceType === ORGANIZATION;
	const question: CheckQuestion | null =
		organization &&
		member?.external_id &&
		permission &&
		resourceType &&
		(choosesOrganization || resource)
			? {
					organization: { id: organization.id, name: organization.name },
					member: {
						label: memberLabel(member),
						externalId: member.external_id,
					},
					permission: {
						name: permission.name,
						scope: permission.resource_type,
					},
					resource: choosesOrganization ? null : resource,
				}
			: null;
	const unavailable =
		check.isError && apiErrorHasCode(check.error, UNAVAILABLE);
	const typeLabel = resourceType ? typeName(types, resourceType) : "";

	return (
		<Box
			as="section"
			aria-labelledby="check-access-heading"
			className="min-w-0 rounded-2xl border border-border bg-card p-4"
		>
			<Stack space="md">
				<Stack space="xs">
					<Heading level={2} id="check-access-heading">
						Check access
					</Heading>
					<Text tone="muted">
						Ask whether a member holds a permission on a resource, and see the
						role that grants it.
					</Text>
				</Stack>

				<FieldGroup>
					<Field>
						<FieldLabel htmlFor="check-access-organization">
							Organization
						</FieldLabel>
						<SearchPicker<ProductOrganizationResponse>
							id="check-access-organization"
							placeholder="Search organizations"
							listLabel="Organizations"
							value={organization}
							onChange={(next) => {
								setOrganization(next);
								setMember(null);
								setResource(null);
								changed();
							}}
							queryKey={["check-access", productId, "organizations"]}
							search={async (term, signal) => {
								const { data } = await searchProductOrganizations({
									path: { product_id: productId },
									body: {
										full_text_search: term || undefined,
										sort_by: "name",
										pagination: { limit: SEARCH_PAGE },
									},
									signal,
									throwOnError: true,
								});
								return data.items;
							}}
							itemKey={(item) => item.id}
							itemLabel={(item) => item.name}
							noMatch="No organization matches."
							nothing="This product has no organizations yet."
							failure="Anchor didn't return the organizations."
						/>
					</Field>

					<Field>
						<FieldLabel htmlFor="check-access-member">Member</FieldLabel>
						<SearchPicker<OrganizationMemberResponse>
							id="check-access-member"
							placeholder={
								organization
									? "Search members by name or email"
									: "Choose an organization first"
							}
							listLabel="Members"
							value={member}
							disabled={!organization}
							onChange={(next) => {
								setMember(next);
								changed();
							}}
							queryKey={[
								"check-access",
								productId,
								"members",
								organization?.id,
							]}
							search={async (term, signal) => {
								if (!organization) return [];
								const { data } = await searchOrganizationMembers({
									path: {
										product_id: productId,
										organization_id: organization.id,
									},
									body: {
										full_text_search: term || undefined,
										sort_by: "email",
										pagination: { limit: SEARCH_PAGE },
									},
									signal,
									throwOnError: true,
								});
								return data.items;
							}}
							itemKey={(item) => item.product_user_id}
							itemLabel={memberLabel}
							itemDetail={(item) => (item.name ? item.email : undefined)}
							itemBlocked={(item) =>
								item.external_id
									? undefined
									: `${item.email} · No external ID yet, so Anchor can't name them in a check.`
							}
							noMatch="No member matches."
							nothing={
								organization
									? `${organization.name} has no members yet.`
									: "Choose an organization first."
							}
							failure="Anchor didn't return the members."
						/>
					</Field>

					<Field>
						<FieldLabel htmlFor="check-access-permission">
							Permission
						</FieldLabel>
						<SearchPicker<ProductResourcePermissionResponse>
							id="check-access-permission"
							placeholder="Search permissions"
							listLabel="Permissions"
							value={permission}
							onChange={(next) => {
								setPermission(next);
								changed();
							}}
							queryKey={["check-access", productId, "permissions"]}
							search={async (term, signal) => {
								const { data } = await searchProductResourcePermissions({
									path: { product_id: productId },
									body: {
										full_text_search: term || undefined,
										sort_by: "name",
										pagination: { limit: SEARCH_PAGE },
									},
									signal,
									throwOnError: true,
								});
								return data.items;
							}}
							itemKey={(item) => item.name}
							itemLabel={(item) => item.name}
							itemDetail={(item) =>
								`Scoped to ${typeName(types, item.resource_type)}`
							}
							noMatch="No permission matches."
							nothing="This product has no permissions yet."
							failure="Anchor didn't return the permissions."
						/>
					</Field>

					<Field>
						<FieldLabel htmlFor="check-access-resource-type">
							Resource type
						</FieldLabel>
						<ResourceTypeSelect
							id="check-access-resource-type"
							productId={productId}
							value={resourceType}
							placeholder="Choose the resource's type"
							onChange={(slug) => {
								setResourceType(slug);
								setResource(null);
								changed();
							}}
						/>
						{choosesOrganization && (
							<FieldDescription>
								{organization
									? `Checks ${organization.name} itself.`
									: "Checks the Organization itself."}
							</FieldDescription>
						)}
					</Field>

					{resourceType && !choosesOrganization && (
						<Field>
							<FieldLabel htmlFor="check-access-resource">Resource</FieldLabel>
							<SearchPicker<ResourceResponse>
								id="check-access-resource"
								placeholder={
									organization
										? `Search ${typeLabel} resources by name`
										: "Choose an organization first"
								}
								listLabel="Resources"
								value={resource}
								disabled={!organization}
								onChange={(next) => {
									setResource(next);
									changed();
								}}
								queryKey={[
									"check-access",
									productId,
									"resources",
									organization?.id,
									resourceType,
								]}
								search={async (term, signal) => {
									if (!organization) return [];
									const { data } = await searchResources({
										path: {
											product_id: productId,
											organization_id: organization.id,
										},
										body: {
											filter: {
												resource_types: [resourceType],
												name_contains: term || undefined,
											},
											sort_by: ResourceSortField.NAME,
											pagination: { limit: SEARCH_PAGE },
										},
										signal,
										throwOnError: true,
									});
									return data.items;
								}}
								itemKey={(item) => `${item.type}:${item.id}`}
								itemLabel={(item) => item.name}
								itemDetail={(item) => item.id}
								noMatch={`No ${typeLabel} matches.`}
								nothing={
									organization
										? `${organization.name} has no registered ${typeLabel} resources.`
										: "Choose an organization first."
								}
								failure="Anchor didn't return the resources."
							/>
						</Field>
					)}
				</FieldGroup>

				<Box className="flex flex-wrap items-center gap-3">
					<Button
						variant="solid"
						tone="brand"
						loading={check.isPending}
						disabled={!question}
						onClick={() => question && check.mutate(question)}
					>
						Check
					</Button>
					{!question && !check.isPending && (
						<Text tone="muted" size="sm">
							Choose all four to check.
						</Text>
					)}
				</Box>

				{check.isPending && (
					<output aria-live="polite" className="sr-only">
						Checking access…
					</output>
				)}

				{unavailable && check.variables && (
					<Alert tone="warning" icon={CloudSlashIcon}>
						<AlertTitle>Access decisions are unavailable right now</AlertTitle>
						<AlertDescription>
							<Stack space="sm" align="start">
								<Box as="span">
									Anchor's decision store didn't answer, so it made no decision.
									Try again in a moment.
								</Box>
								<Button
									size="sm"
									variant="outline"
									onClick={() =>
										check.variables && check.mutate(check.variables)
									}
								>
									Try again
								</Button>
							</Stack>
						</AlertDescription>
					</Alert>
				)}

				{check.isError && !unavailable && check.variables && (
					<Alert tone="critical" icon={WarningCircleIcon}>
						<AlertTitle>Anchor didn't check access</AlertTitle>
						<AlertDescription>
							<Stack space="sm" align="start">
								<Box as="span" className="[overflow-wrap:anywhere]">
									{getErrorDetail(check.error) ??
										"Check your connection and try again."}
								</Box>
								<Button
									size="sm"
									variant="outline"
									onClick={() =>
										check.variables && check.mutate(check.variables)
									}
								>
									Try again
								</Button>
							</Stack>
						</AlertDescription>
					</Alert>
				)}

				{check.data && explanation && (
					<AccessDecision
						result={check.data}
						explanation={explanation}
						types={types}
					/>
				)}
			</Stack>
		</Box>
	);
}
