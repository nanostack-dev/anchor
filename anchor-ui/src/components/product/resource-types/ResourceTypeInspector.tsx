import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Button,
	ButtonLink,
} from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	Field,
	FieldDescription,
	FieldError,
	FieldGroup,
	FieldLabel,
	FieldLegend,
	FieldSet,
} from "@nanostackorg/design-system/components/field";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Input } from "@nanostackorg/design-system/components/input";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { PlusIcon, TrashIcon } from "@phosphor-icons/react";
import { LockIcon } from "lucide-react";

import {
	type TreeType,
	childrenOf,
	parentCandidates,
	plural,
	slugProblem,
} from "./resource-type-tree";

export type TypeUsage =
	| { state: "unpublished" }
	| { state: "loading" }
	| { state: "error"; retry: () => void }
	| { state: "ready"; roles: number; permissions: number };

export interface ResourceTypeInspectorProps {
	type: TreeType | null;
	types: TreeType[];
	editing: boolean;
	naming?: boolean;
	reservedSlugs?: ReadonlySet<string>;
	usage: TypeUsage;
	deleteBlockers: string[];
	addBlocker?: string;
	onRename: (name: string) => void;
	onSlugChange: (slug: string) => void;
	onToggleParent: (parentKey: string) => void;
	onNestsChange: (nests: boolean) => void;
	onCustomRolesChange: (on: boolean) => void;
	onAddChild: () => void;
	onDelete: () => void;
}

function Section({ children }: { children: React.ReactNode }) {
	return <Box className="border-border border-t pt-4">{children}</Box>;
}

function UsageCounts({ usage }: { usage: TypeUsage }) {
	if (usage.state === "loading")
		return (
			<Stack space="xs" aria-label="Loading usage">
				<Skeleton width="3/4" />
			</Stack>
		);
	if (usage.state === "error")
		return (
			<Stack space="xs" align="start">
				<Text tone="critical">Couldn't count the roles and permissions.</Text>
				<Button size="sm" variant="outline" onClick={usage.retry}>
					Try again
				</Button>
			</Stack>
		);
	if (usage.state === "unpublished")
		return (
			<Text tone="muted">
				0 roles · 0 permissions. Roles and permissions can use it once it is
				published.
			</Text>
		);
	return (
		<Text>
			{plural(usage.roles, "role", "roles")} ·{" "}
			{plural(usage.permissions, "permission", "permissions")}
		</Text>
	);
}

export function ResourceTypeInspector({
	type,
	types,
	editing,
	naming = false,
	reservedSlugs,
	usage,
	deleteBlockers,
	addBlocker,
	onRename,
	onSlugChange,
	onToggleParent,
	onNestsChange,
	onCustomRolesChange,
	onAddChild,
	onDelete,
}: ResourceTypeInspectorProps) {
	if (!type)
		return (
			<Box
				as="aside"
				aria-label="Resource type details"
				className="rounded-2xl border border-border bg-card p-4"
			>
				<Stack space="xs">
					<Heading level={2}>No type selected</Heading>
					<Text tone="muted">
						Select a type in the tree to see where it sits and what uses it.
					</Text>
				</Stack>
			</Box>
		);

	if (naming && !type.name.trim())
		return (
			<Box
				as="aside"
				aria-label="Resource type details"
				className="rounded-2xl border border-border bg-card p-4"
			>
				<Stack space="xs">
					<Heading level={2}>New type</Heading>
					<Text tone="muted">
						Name it in the tree. Enter keeps it, Escape drops it.
					</Text>
				</Stack>
			</Box>
		);

	const byKey = new Map(types.map((item) => [item.key, item]));
	const candidates = parentCandidates(types, type.key);
	const children = childrenOf(types, type.key);
	const nameId = `resource-type-name-${type.key}`;
	const slugId = `resource-type-slug-${type.key}`;
	const nestsId = `resource-type-nests-${type.key}`;
	const customRolesId = `resource-type-custom-roles-${type.key}`;
	const filtered = (path: string) =>
		`${path}?${new URLSearchParams({ resource_type: type.slug }).toString()}`;
	const switchLocked = !type.builtIn && !type.customRoles;
	const slugError =
		editing && !type.builtIn && !naming
			? slugProblem(types, type, reservedSlugs)
			: "";
	const nameMissing = !naming && !type.name.trim();
	const canEdit = editing && !type.builtIn;
	const parentNames = type.parents
		.map((key) => byKey.get(key)?.name ?? key)
		.join(", ");

	return (
		<Box
			as="aside"
			aria-label="Resource type details"
			className="min-w-0 rounded-2xl border border-border bg-card p-4"
		>
			<Stack space="md">
				<Stack space="xs">
					<Box className="flex min-w-0 items-start gap-2">
						{type.builtIn && (
							<LockIcon
								aria-hidden
								className="mt-1 size-4 shrink-0 text-muted-foreground"
							/>
						)}
						<Box className="min-w-0 [overflow-wrap:anywhere]">
							<Heading level={2}>{type.name || "New type"}</Heading>
						</Box>
					</Box>
					{type.builtIn && (
						<Text tone="muted">
							Built in. Organization is the root of every tree: it can't be
							renamed, moved or deleted.
						</Text>
					)}
				</Stack>

				{canEdit ? (
					<FieldGroup>
						<Field invalid={nameMissing}>
							<FieldLabel htmlFor={nameId}>Name</FieldLabel>
							<Input
								id={nameId}
								value={type.name}
								maxLength={120}
								aria-invalid={nameMissing || undefined}
								onChange={(event) => onRename(event.target.value)}
							/>
							{nameMissing && <FieldError>Give the type a name.</FieldError>}
						</Field>
						<Field invalid={Boolean(slugError)}>
							<FieldLabel htmlFor={slugId}>Slug</FieldLabel>
							{type.published ? (
								<>
									<Input id={slugId} value={type.slug} font="mono" readOnly />
									<FieldDescription>
										A published slug never changes.
									</FieldDescription>
								</>
							) : (
								<>
									<Input
										id={slugId}
										value={type.slug}
										font="mono"
										maxLength={49}
										aria-invalid={Boolean(slugError) || undefined}
										onChange={(event) => onSlugChange(event.target.value)}
									/>
									{slugError ? (
										<FieldError>{slugError}</FieldError>
									) : (
										<FieldDescription>
											API calls use this. It can't change after you publish.
										</FieldDescription>
									)}
								</>
							)}
						</Field>
					</FieldGroup>
				) : (
					<dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm">
						<dt className="text-muted-foreground">Slug</dt>
						<dd className="min-w-0 font-mono text-foreground [overflow-wrap:anywhere]">
							{type.slug}
						</dd>
						{!type.builtIn && (
							<>
								<dt className="text-muted-foreground">Parents</dt>
								<dd className="min-w-0 text-foreground [overflow-wrap:anywhere]">
									{parentNames}
								</dd>
								<dt className="text-muted-foreground">Nesting</dt>
								<dd className="text-foreground">
									{type.nestsInItself ? "Nests in itself" : "Doesn't nest"}
								</dd>
							</>
						)}
						<dt className="text-muted-foreground">Below</dt>
						<dd className="text-foreground">
							{plural(children.length, "type", "types")}
						</dd>
					</dl>
				)}

				{canEdit && (
					<Section>
						<FieldSet>
							<FieldLegend size="sm">Allowed parent types</FieldLegend>
							<FieldDescription>
								An instance of {type.name || "this type"} can sit under any of
								these.
							</FieldDescription>
							<Stack space="sm" as="ul">
								{candidates.map((candidate) => {
									const id = `parent-${type.key}-${candidate.key}`;
									const checked = type.parents.includes(candidate.key);
									const last = checked && type.parents.length === 1;
									return (
										<Box
											as="li"
											key={candidate.key}
											className="flex min-w-0 items-center gap-2"
										>
											<Checkbox
												id={id}
												checked={checked}
												disabled={last}
												onCheckedChange={() => onToggleParent(candidate.key)}
											/>
											<Box className="min-w-0 [overflow-wrap:anywhere]">
												<FieldLabel htmlFor={id}>
													{candidate.name || "New type"}
												</FieldLabel>
											</Box>
										</Box>
									);
								})}
							</Stack>
						</FieldSet>
					</Section>
				)}

				{canEdit && (
					<Section>
						<Box className="flex items-start gap-2">
							<Box className="pt-0.5">
								<Checkbox
									id={nestsId}
									checked={type.nestsInItself}
									onCheckedChange={(checked) => onNestsChange(checked === true)}
								/>
							</Box>
							<Stack space="xs">
								<FieldLabel htmlFor={nestsId}>Nests in itself</FieldLabel>
								<FieldDescription>
									An instance can sit under another of the same type, like a
									folder inside a folder.
								</FieldDescription>
							</Stack>
						</Box>
					</Section>
				)}

				<Section>
					<Stack space="sm">
						<Stack space="xs">
							<Text weight="medium">Scoped to this type</Text>
							<UsageCounts usage={usage} />
						</Stack>
						{usage.state !== "unpublished" && (
							<Inline space="sm">
								<ButtonLink
									href={filtered(ROUTE_PATHS.PRODUCT_ROLES)}
									variant="outline"
									size="sm"
								>
									View roles
								</ButtonLink>
								<ButtonLink
									href={filtered(ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS)}
									variant="outline"
									size="sm"
								>
									View permissions
								</ButtonLink>
							</Inline>
						)}
					</Stack>
				</Section>

				<Section key={`custom-roles-${type.key}`}>
					{editing ? (
						<Box className="flex items-start justify-between gap-3">
							<Stack space="xs">
								<FieldLabel htmlFor={customRolesId}>Custom roles</FieldLabel>
								<FieldDescription>
									{switchLocked
										? "Anchor accepts custom roles on Organization only, for now."
										: `Organizations can create their own roles scoped to ${type.name || "this type"} and change their copies of its role templates.`}
								</FieldDescription>
							</Stack>
							<Box className="pt-0.5">
								<Switch
									id={customRolesId}
									checked={type.customRoles}
									disabled={switchLocked}
									onCheckedChange={(checked) => onCustomRolesChange(checked)}
								/>
							</Box>
						</Box>
					) : (
						<Stack space="xs">
							<Text weight="medium">
								Custom roles {type.customRoles ? "on" : "off"}
							</Text>
							<Text tone="muted">
								{type.customRoles
									? `Organizations can create their own roles scoped to ${type.name} and change their copies of its role templates.`
									: "Organizations use the role templates as they are."}
							</Text>
						</Stack>
					)}
				</Section>

				{editing && (
					<Section>
						<Stack space="sm" align="start">
							<Button
								variant="outline"
								size="sm"
								icon={PlusIcon}
								disabled={Boolean(addBlocker)}
								onClick={onAddChild}
							>
								Add child type
							</Button>
							{addBlocker && <Text tone="muted">{addBlocker}</Text>}
							{!type.builtIn && (
								<>
									<Button
										variant="outline"
										tone="critical"
										size="sm"
										icon={TrashIcon}
										disabled={deleteBlockers.length > 0}
										onClick={onDelete}
									>
										Delete {type.name || "this type"}
									</Button>
									{deleteBlockers.length > 0 && (
										<Stack
											as="ul"
											space="xs"
											aria-label="Why it can't be deleted"
										>
											{deleteBlockers.map((reason) => (
												<Box as="li" key={reason}>
													<Text as="span" tone="muted">
														{reason}
													</Text>
												</Box>
											))}
										</Stack>
									)}
								</>
							)}
						</Stack>
					</Section>
				)}
			</Stack>
		</Box>
	);
}
