import type { ProductRoleResponse } from "@/client";
import {
	createProductRoleMutation,
	getProductRoleQueryKey,
	updateProductRoleMutation,
} from "@/client/@tanstack/react-query.gen";
import { zProductRoleCreateRequest } from "@/client/zod.gen";
import {
	ResourceTypeSelect,
	useScopeTypes,
} from "@/components/product/resource-types/ResourceTypeSelect";
import {
	ORGANIZATION_SLUG,
	type ScopeType,
	scopeFilter,
	typeName,
	typesInScope,
	typesOutsideScope,
} from "@/components/product/resource-types/resource-type-scope";
import { getApiErrorMessage } from "@/lib/api-error";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	Field,
	FieldDescription,
	FieldError,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Input } from "@nanostackorg/design-system/components/input";
import {
	Progress,
	ProgressLabel,
} from "@nanostackorg/design-system/components/progress";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { Text } from "@nanostackorg/design-system/components/text";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactNode, useState } from "react";

import { ScopedPermissionPicker } from "./ScopedPermissionPicker";
import { useScopedPermissions } from "./role-queries";

type StepId = "scope" | "details" | "permissions" | "review";

const STEP_TITLES: Record<StepId, string> = {
	scope: "Scope",
	details: "Details",
	permissions: "Permissions",
	review: "Review",
};

const detailsSchema = zProductRoleCreateRequest.pick({
	name: true,
	description: true,
	key: true,
});

interface RoleDraft {
	scope: string;
	name: string;
	key: string;
	description: string;
	isDefault: boolean;
	permissions: string[];
}

export interface ProductRoleEditorProps {
	productId: string;
	mode: "create" | "edit";
	existingRole?: ProductRoleResponse;
	onSaved: (role: ProductRoleResponse) => void;
	onCancel: () => void;
}

function fieldErrors(draft: RoleDraft, editing: boolean) {
	const result = detailsSchema.safeParse({
		name: draft.name.trim(),
		description: draft.description || undefined,
		key: editing ? undefined : draft.key.trim() || undefined,
	});
	const errors: Partial<Record<"name" | "description" | "key", string>> = {};
	if (result.success) return errors;
	for (const issue of result.error.issues) {
		const field = issue.path[0];
		if (field === "name")
			errors.name =
				draft.name.trim().length < 2
					? "Give the role a name of at least 2 characters."
					: "Use at most 100 characters.";
		if (field === "description")
			errors.description = "Use at most 500 characters.";
		if (field === "key") errors.key = "Use at most 100 characters.";
	}
	return errors;
}

function TypesError({ retry }: { retry: () => void }) {
	return (
		<Alert tone="critical" icon={WarningCircleIcon}>
			<AlertTitle>Couldn't load the resource types</AlertTitle>
			<AlertDescription>
				<Stack space="sm" align="start">
					<span>
						Anchor didn't answer. Check your connection and try again.
					</span>
					<Button size="sm" variant="outline" onClick={retry}>
						Try again
					</Button>
				</Stack>
			</AlertDescription>
		</Alert>
	);
}

function BadgeLabel({ children }: { children: string }) {
	return (
		<Box as="span" className="block max-w-[14rem] truncate" title={children}>
			{children}
		</Box>
	);
}

function StepFooter({
	onBack,
	children,
}: {
	onBack?: () => void;
	children: ReactNode;
}) {
	return (
		<Box className="flex flex-wrap items-center justify-between gap-3 border-border border-t pt-4">
			<Box>
				{onBack && (
					<Button variant="ghost" onClick={onBack}>
						Back
					</Button>
				)}
			</Box>
			<Inline space="sm">{children}</Inline>
		</Box>
	);
}

function ScopeSummary({ types, scope }: { types: ScopeType[]; scope: string }) {
	const inScope = typesInScope(types, scope);
	const outside = typesOutsideScope(types, scope);
	return (
		<Box className="rounded-xl border border-border bg-muted/40 p-4">
			<Stack space="sm">
				<Text weight="medium">
					It can bundle permissions of{" "}
					{inScope.length === 1 ? "this type" : `these ${inScope.length} types`}
				</Text>
				<Box className="flex flex-wrap gap-1.5">
					{inScope.map((type) => (
						<Badge
							key={type.slug}
							tone={type.slug === scope ? "brand" : "neutral"}
						>
							<BadgeLabel>{type.name}</BadgeLabel>
						</Badge>
					))}
				</Box>
				{outside.length > 0 && (
					<Text tone="muted" size="xs">
						Not offered: {outside.map((type) => type.name).join(", ")}. A role
						bundles only permissions of its own type and the types below it.
					</Text>
				)}
			</Stack>
		</Box>
	);
}

export function ProductRoleEditor({
	productId,
	mode,
	existingRole,
	onSaved,
	onCancel,
}: ProductRoleEditorProps) {
	const queryClient = useQueryClient();
	const editing = mode === "edit";
	const steps: StepId[] = editing
		? ["details", "permissions", "review"]
		: ["scope", "details", "permissions", "review"];
	const [step, setStep] = useState(0);
	const [touched, setTouched] = useState(false);
	const [draft, setDraft] = useState<RoleDraft>(() => ({
		scope: existingRole?.resource_type ?? "",
		name: existingRole?.name ?? "",
		key: existingRole?.key ?? "",
		description: existingRole?.description ?? "",
		isDefault: existingRole?.is_default ?? false,
		permissions:
			existingRole?.permissions.map((grant) => grant.permission_name) ?? [],
	}));
	const typesQuery = useScopeTypes(productId);
	const types = typesQuery.data ?? [];
	const scopeKnown = types.some((type) => type.slug === draft.scope);
	const permissionsQuery = useScopedPermissions(
		productId,
		scopeKnown ? scopeFilter(types, draft.scope) : undefined,
		scopeKnown,
	);
	const errors = fieldErrors(draft, editing);
	const current = steps[step];
	const scopeName = typeName(types, draft.scope);
	const organizationScope = draft.scope === ORGANIZATION_SLUG;

	const finish = (role: ProductRoleResponse, verb: string) => {
		void queryClient.invalidateQueries({
			predicate: (cached) => {
				const [key] = cached.queryKey as [{ _id?: string }];
				return key?._id === "searchProductRoles";
			},
		});
		void queryClient.invalidateQueries({
			queryKey: getProductRoleQueryKey({
				path: { product_id: productId, role_id: role.id },
			}),
		});
		toast.add({
			type: "success",
			title: `Role ${verb}`,
			description: role.name,
		});
		onSaved(role);
	};
	const create = useMutation({
		...createProductRoleMutation(),
		onSuccess: (role) => finish(role, "created"),
	});
	const update = useMutation({
		...updateProductRoleMutation(),
		onSuccess: (role) => finish(role, "updated"),
	});
	const saving = create.isPending || update.isPending;
	const saveError = create.error ?? update.error;

	const offeredNames = new Set(
		(permissionsQuery.data ?? []).map((permission) => permission.name),
	);
	const keptPermissions = draft.permissions.filter((name) =>
		offeredNames.has(name),
	);

	const save = () => {
		const description = draft.description.trim() || undefined;
		if (editing && existingRole)
			update.mutate({
				path: { product_id: productId, role_id: existingRole.id },
				body: {
					name: draft.name.trim(),
					description: description ?? null,
					permissions: keptPermissions,
					...(organizationScope && draft.isDefault !== existingRole.is_default
						? { is_default: draft.isDefault }
						: {}),
				},
			});
		else
			create.mutate({
				path: { product_id: productId },
				body: {
					name: draft.name.trim(),
					description,
					key: draft.key.trim() || undefined,
					resource_type: draft.scope,
					permissions: keptPermissions,
					is_default: organizationScope && draft.isDefault,
				},
			});
	};

	const next = () => setStep((value) => Math.min(value + 1, steps.length - 1));
	const back = () => setStep((value) => Math.max(value - 1, 0));
	const change = (patch: Partial<RoleDraft>) =>
		setDraft((value) => ({ ...value, ...patch }));

	const permissionState = permissionsQuery.isError
		? {
				status: "error" as const,
				retry: () => {
					void permissionsQuery.refetch();
				},
			}
		: permissionsQuery.data
			? { status: "ready" as const, permissions: permissionsQuery.data }
			: { status: "loading" as const };

	const cancel = (
		<Button variant="outline" onClick={onCancel}>
			Cancel
		</Button>
	);

	return (
		<Tabs value={current}>
			<Stack space="lg">
				<Stack space="sm">
					<TabsList width="fill" aria-label="Role setup steps">
						{steps.map((id, index) => (
							<TabsTrigger key={id} value={id} disabled={index !== step}>
								{index + 1}. {STEP_TITLES[id]}
							</TabsTrigger>
						))}
					</TabsList>
					<Progress value={step + 1} max={steps.length}>
						<ProgressLabel>
							Step {step + 1} of {steps.length}
						</ProgressLabel>
					</Progress>
				</Stack>

				{!editing && (
					<TabsContent value="scope">
						<Stack space="lg">
							<Stack space="xs">
								<Heading level={2}>Where is this role held?</Heading>
								<Text tone="muted">
									Choose the resource type members hold this role on. It decides
									which permissions the role can bundle, and it never changes
									after you create the role.
								</Text>
							</Stack>
							<Field>
								<FieldLabel htmlFor="role-scope">Resource type</FieldLabel>
								<ResourceTypeSelect
									id="role-scope"
									productId={productId}
									value={draft.scope}
									onChange={(scope) =>
										change({
											scope,
											isDefault: scope === ORGANIZATION_SLUG && draft.isDefault,
										})
									}
								/>
								{!draft.scope && (
									<FieldDescription>
										Choose a resource type to continue.
									</FieldDescription>
								)}
							</Field>
							{scopeKnown && <ScopeSummary types={types} scope={draft.scope} />}
							<StepFooter>
								{cancel}
								<Button
									variant="solid"
									tone="brand"
									disabled={!scopeKnown}
									onClick={next}
								>
									Continue
								</Button>
							</StepFooter>
						</Stack>
					</TabsContent>
				)}

				<TabsContent value="details">
					<Stack space="lg">
						<Stack space="xs">
							<Heading level={2}>Details</Heading>
							<Text tone="muted">
								{editing
									? "Rename the role or change its description."
									: `Name the ${scopeName} role.`}
							</Text>
						</Stack>
						{editing && typesQuery.isError && (
							<TypesError retry={() => void typesQuery.refetch()} />
						)}
						<FieldGroup>
							<Field invalid={touched && Boolean(errors.name)}>
								<FieldLabel htmlFor="role-name">Name</FieldLabel>
								<Input
									id="role-name"
									value={draft.name}
									maxLength={120}
									placeholder="e.g. Project admin"
									aria-invalid={(touched && Boolean(errors.name)) || undefined}
									onChange={(event) => change({ name: event.target.value })}
									onBlur={() => setTouched(true)}
								/>
								{touched && errors.name && (
									<FieldError>{errors.name}</FieldError>
								)}
							</Field>
							<Field invalid={Boolean(errors.key)}>
								<FieldLabel htmlFor="role-key">Key</FieldLabel>
								<Input
									id="role-key"
									font="mono"
									value={draft.key}
									readOnly={editing}
									maxLength={120}
									placeholder={
										draft.name.trim().toLowerCase() || "project-admin"
									}
									onChange={(event) => change({ key: event.target.value })}
								/>
								{errors.key ? (
									<FieldError>{errors.key}</FieldError>
								) : (
									<FieldDescription>
										{editing
											? "A role's key never changes."
											: "Your product names the role by this key in every organization. Left empty, it is the name in lower case. It never changes."}
									</FieldDescription>
								)}
							</Field>
							<Field invalid={Boolean(errors.description)}>
								<FieldLabel htmlFor="role-description">Description</FieldLabel>
								<Textarea
									id="role-description"
									value={draft.description}
									rows={3}
									onChange={(event) =>
										change({ description: event.target.value })
									}
								/>
								{errors.description && (
									<FieldError>{errors.description}</FieldError>
								)}
							</Field>
							{editing && (
								<Field>
									<FieldLabel htmlFor="role-scope-fixed">
										Resource type
									</FieldLabel>
									<Input id="role-scope-fixed" value={scopeName} readOnly />
									<FieldDescription>
										A role's resource type never changes.
									</FieldDescription>
								</Field>
							)}
							{organizationScope && (
								<Field orientation="horizontal">
									<Checkbox
										id="role-default"
										checked={draft.isDefault}
										onCheckedChange={(checked) =>
											change({ isDefault: checked === true })
										}
									/>
									<Stack space="xs">
										<FieldLabel htmlFor="role-default">Default role</FieldLabel>
										<FieldDescription>
											A member or an invitation that names no role gets this
											role. Only one Organization role can be the default.
										</FieldDescription>
									</Stack>
								</Field>
							)}
						</FieldGroup>
						<StepFooter onBack={editing ? undefined : back}>
							{cancel}
							<Button
								variant="solid"
								tone="brand"
								disabled={Object.keys(errors).length > 0}
								onClick={() => {
									setTouched(true);
									if (Object.keys(errors).length === 0) next();
								}}
							>
								Continue
							</Button>
						</StepFooter>
					</Stack>
				</TabsContent>

				<TabsContent value="permissions">
					<Stack space="lg">
						<Stack space="xs">
							<Heading level={2}>Permissions</Heading>
							<Text tone="muted">
								Grouped by resource type, from {scopeName} down.
							</Text>
						</Stack>
						{scopeKnown ? (
							<ScopedPermissionPicker
								types={types}
								scope={draft.scope}
								state={permissionState}
								selected={draft.permissions}
								onChange={(permissions) => change({ permissions })}
								newPermissionHref={`${ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_NEW}?resource_type=${encodeURIComponent(draft.scope)}`}
							/>
						) : typesQuery.isError ? (
							<TypesError retry={() => void typesQuery.refetch()} />
						) : (
							<Text tone="muted">Loading the resource types…</Text>
						)}
						<StepFooter onBack={back}>
							{cancel}
							<Button
								variant="solid"
								tone="brand"
								disabled={permissionState.status !== "ready"}
								onClick={next}
							>
								Continue
							</Button>
						</StepFooter>
					</Stack>
				</TabsContent>

				<TabsContent value="review">
					<Stack space="lg">
						<Stack space="xs">
							<Heading level={2}>Review</Heading>
							<Text tone="muted">
								{editing
									? "Check the changes, then save."
									: "Check the role, then create it."}
							</Text>
						</Stack>
						{saveError && (
							<Alert tone="critical" icon={WarningCircleIcon}>
								<AlertTitle>
									Anchor didn't {editing ? "save" : "create"} the role
								</AlertTitle>
								<AlertDescription>
									{getApiErrorMessage(saveError) ?? "Try again in a moment."}
								</AlertDescription>
							</Alert>
						)}
						<RoleReview
							draft={draft}
							types={types}
							permissions={permissionsQuery.data ?? []}
							kept={keptPermissions}
							original={
								existingRole?.permissions.map(
									(grant) => grant.permission_name,
								) ?? []
							}
							editing={editing}
						/>
						<StepFooter onBack={back}>
							{cancel}
							<Button
								variant="solid"
								tone="brand"
								loading={saving}
								onClick={save}
							>
								{editing ? "Save changes" : "Create role"}
							</Button>
						</StepFooter>
					</Stack>
				</TabsContent>
			</Stack>
		</Tabs>
	);
}

function RoleReview({
	draft,
	types,
	permissions,
	kept,
	original,
	editing,
}: {
	draft: RoleDraft;
	types: ScopeType[];
	permissions: { name: string; resource_type: string }[];
	kept: string[];
	original: string[];
	editing: boolean;
}) {
	const scopeOf = new Map(
		permissions.map((permission) => [
			permission.name,
			permission.resource_type,
		]),
	);
	const removed = editing
		? original.filter((name) => !kept.includes(name))
		: [];
	const groups = typesInScope(types, draft.scope)
		.map((type) => ({
			type,
			names: kept.filter((name) => scopeOf.get(name) === type.slug),
		}))
		.filter((group) => group.names.length > 0);
	return (
		<Box className="rounded-xl border border-border">
			<dl className="grid gap-x-6 gap-y-4 p-4 sm:grid-cols-[10rem_minmax(0,1fr)]">
				<dt className="text-muted-foreground text-sm">Resource type</dt>
				<dd className="min-w-0 text-sm [overflow-wrap:anywhere]">
					{typeName(types, draft.scope)}
				</dd>
				<dt className="text-muted-foreground text-sm">Name</dt>
				<dd className="min-w-0 font-medium text-sm [overflow-wrap:anywhere]">
					{draft.name.trim()}
					{draft.isDefault && draft.scope === ORGANIZATION_SLUG && (
						<Box as="span" className="ml-2 inline-flex align-middle">
							<Badge tone="brand">Default</Badge>
						</Box>
					)}
				</dd>
				<dt className="text-muted-foreground text-sm">Key</dt>
				<dd className="min-w-0 font-mono text-sm [overflow-wrap:anywhere]">
					{draft.key.trim() || draft.name.trim().toLowerCase()}
				</dd>
				<dt className="text-muted-foreground text-sm">Description</dt>
				<dd className="min-w-0 whitespace-pre-wrap text-sm [overflow-wrap:anywhere]">
					{draft.description.trim() || "No description"}
				</dd>
				<dt className="text-muted-foreground text-sm">Permissions</dt>
				<dd className="min-w-0 text-sm">
					{groups.length === 0 ? (
						"None. Members holding this role get no permission from it."
					) : (
						<Stack space="sm">
							{groups.map((group) => (
								<Stack key={group.type.slug} space="xs">
									<Text size="xs" tone="muted">
										{group.type.name}
									</Text>
									<Box className="flex flex-wrap gap-1.5">
										{group.names.map((name) => (
											<Badge key={name} variant="outline">
												<BadgeLabel>
													{`${name}${editing && !original.includes(name) ? " · new" : ""}`}
												</BadgeLabel>
											</Badge>
										))}
									</Box>
								</Stack>
							))}
						</Stack>
					)}
				</dd>
				{removed.length > 0 && (
					<>
						<dt className="text-muted-foreground text-sm">Removed</dt>
						<dd className="min-w-0 text-sm">
							<Box className="flex flex-wrap gap-1.5">
								{removed.map((name) => (
									<Badge key={name} tone="critical" variant="outline">
										<BadgeLabel>{name}</BadgeLabel>
									</Badge>
								))}
							</Box>
						</dd>
					</>
				)}
			</dl>
		</Box>
	);
}
