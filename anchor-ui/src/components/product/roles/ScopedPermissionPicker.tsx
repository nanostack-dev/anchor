import type { ProductResourcePermissionResponse } from "@/client";
import {
	type ScopeType,
	typeName,
	typesInScope,
	typesOutsideScope,
} from "@/components/product/resource-types/resource-type-scope";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Badge } from "@nanostackorg/design-system/components/badge";
import {
	Button,
	ButtonLink,
} from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { FieldLabel } from "@nanostackorg/design-system/components/field";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "@nanostackorg/design-system/components/input-group";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { MagnifyingGlassIcon, WarningCircleIcon } from "@phosphor-icons/react";
import { useId, useMemo, useState } from "react";

export type PermissionListState =
	| { status: "loading" }
	| { status: "error"; retry: () => void }
	| { status: "ready"; permissions: ProductResourcePermissionResponse[] };

export interface ScopedPermissionPickerProps {
	types: ScopeType[];
	scope: string;
	state: PermissionListState;
	selected: string[];
	onChange: (selected: string[]) => void;
	newPermissionHref?: string;
}

function listNames(names: string[], limit = 5) {
	if (names.length <= limit) return names.join(", ");
	return `${names.slice(0, limit).join(", ")} and ${names.length - limit} more`;
}

export function ScopedPermissionPicker({
	types,
	scope,
	state,
	selected,
	onChange,
	newPermissionHref,
}: ScopedPermissionPickerProps) {
	const id = useId();
	const [search, setSearch] = useState("");
	const scopeName = typeName(types, scope);
	const inScope = useMemo(() => typesInScope(types, scope), [types, scope]);
	const outside = useMemo(
		() => typesOutsideScope(types, scope).map((type) => type.name),
		[types, scope],
	);
	const permissions = state.status === "ready" ? state.permissions : [];
	const offered = useMemo(() => {
		const slugs = new Set(inScope.map((type) => type.slug));
		return permissions.filter((permission) =>
			slugs.has(permission.resource_type),
		);
	}, [permissions, inScope]);
	const offeredNames = new Set(offered.map((permission) => permission.name));
	const unavailable =
		state.status === "ready"
			? selected.filter((name) => !offeredNames.has(name))
			: [];
	const term = search.trim().toLowerCase();
	const groups = inScope.map((type) => ({
		type,
		all: offered.filter((permission) => permission.resource_type === type.slug),
	}));
	const visibleGroups = groups
		.map((group) => ({
			...group,
			visible: term
				? group.all.filter(
						(permission) =>
							permission.name.toLowerCase().includes(term) ||
							(permission.description ?? "").toLowerCase().includes(term),
					)
				: group.all,
		}))
		.filter((group) => group.visible.length > 0);
	const emptyTypes = groups
		.filter((group) => group.all.length === 0)
		.map((group) => group.type.name);
	const chosen = new Set(selected);

	const toggle = (name: string, checked: boolean) =>
		onChange(
			checked
				? [...new Set([...selected, name])]
				: selected.filter((item) => item !== name),
		);
	const setGroup = (names: string[], checked: boolean) =>
		onChange(
			checked
				? [...new Set([...selected, ...names])]
				: selected.filter((item) => !names.includes(item)),
		);

	const intro = (
		<Box className="min-w-0 [overflow-wrap:anywhere]">
			<Stack space="xs">
				<Text>
					A role scoped to{" "}
					<Text as="span" weight="semibold">
						{scopeName}
					</Text>{" "}
					can hold the permissions of{" "}
					{inScope.length === 1
						? "that type only"
						: `${inScope.length} types: ${listNames(inScope.map((type) => type.name))}`}
					.
				</Text>
				{outside.length > 0 && (
					<Text tone="muted">
						{listNames(outside)} {outside.length === 1 ? "isn't" : "aren't"}{" "}
						offered: {outside.length === 1 ? "it sits" : "they sit"} above or
						beside {scopeName}.
					</Text>
				)}
			</Stack>
		</Box>
	);

	if (state.status === "loading")
		return (
			<Stack space="md">
				{intro}
				<output aria-busy="true" aria-label="Loading permissions">
					<Stack space="sm">
						<Skeleton height="lg" />
						<Skeleton height="lg" />
						<Skeleton height="lg" />
					</Stack>
				</output>
			</Stack>
		);

	if (state.status === "error")
		return (
			<Stack space="md">
				{intro}
				<Alert tone="critical" icon={WarningCircleIcon}>
					<AlertTitle>Couldn't load the permissions</AlertTitle>
					<AlertDescription>
						<Stack space="sm" align="start">
							<span>
								Anchor didn't answer. Check your connection and try again.
							</span>
							<Button size="sm" variant="outline" onClick={state.retry}>
								Try again
							</Button>
						</Stack>
					</AlertDescription>
				</Alert>
			</Stack>
		);

	return (
		<Stack space="md">
			{intro}

			{unavailable.length > 0 && (
				<Alert tone="warning" icon={WarningCircleIcon}>
					<AlertTitle>
						Saving removes{" "}
						{unavailable.length === 1
							? "a permission"
							: `${unavailable.length} permissions`}
					</AlertTitle>
					<AlertDescription>
						<Box as="span" className="[overflow-wrap:anywhere]">
							{listNames(unavailable)} {unavailable.length === 1 ? "is" : "are"}{" "}
							scoped above or beside {scopeName}, so this role can't keep{" "}
							{unavailable.length === 1 ? "it" : "them"}.
						</Box>
					</AlertDescription>
				</Alert>
			)}

			{offered.length === 0 ? (
				<Empty variant="outline">
					<EmptyHeader>
						<EmptyTitle>No permissions to add</EmptyTitle>
						<EmptyDescription>
							No permission is scoped to {scopeName}
							{inScope.length > 1 ? " or the types below it" : ""} yet. You can
							create the role without permissions and add them later.
						</EmptyDescription>
					</EmptyHeader>
					{newPermissionHref && (
						<EmptyContent>
							<ButtonLink href={newPermissionHref} variant="outline" size="sm">
								Create a permission
							</ButtonLink>
						</EmptyContent>
					)}
				</Empty>
			) : (
				<>
					<Box className="flex flex-wrap items-center justify-between gap-3">
						<Box className="min-w-0 flex-1 basis-64">
							<InputGroup>
								<InputGroupAddon>
									<MagnifyingGlassIcon aria-hidden />
								</InputGroupAddon>
								<InputGroupInput
									aria-label="Search permissions"
									placeholder="Search permissions"
									value={search}
									onChange={(event) => setSearch(event.target.value)}
								/>
							</InputGroup>
						</Box>
						<Text tone="muted" aria-live="polite">
							{selected.filter((name) => offeredNames.has(name)).length} of{" "}
							{offered.length} selected
						</Text>
					</Box>

					{visibleGroups.length === 0 ? (
						<Text tone="muted">No permission matches “{search.trim()}”.</Text>
					) : (
						<Stack space="md">
							{visibleGroups.map(({ type, visible }) => {
								const names = visible.map((permission) => permission.name);
								const all = names.every((name) => chosen.has(name));
								const headingId = `${id}-${type.slug}`;
								return (
									<Box
										as="section"
										key={type.slug}
										aria-labelledby={headingId}
										className="min-w-0 overflow-hidden rounded-xl border border-border"
									>
										<Box className="flex flex-wrap items-center justify-between gap-2 border-border border-b bg-muted/60 px-4 py-2.5">
											<Box className="flex min-w-0 items-center gap-2">
												<h3
													id={headingId}
													className="min-w-0 font-medium text-foreground text-sm [overflow-wrap:anywhere]"
												>
													{type.name}
												</h3>
												<Badge variant="outline">
													{names.filter((name) => chosen.has(name)).length}/
													{names.length}
												</Badge>
												{type.slug === scope && (
													<Badge tone="brand">Role scope</Badge>
												)}
											</Box>
											<Button
												size="xs"
												variant="ghost"
												aria-label={`${all ? "Clear" : "Select all"} ${type.name} permissions`}
												onClick={() => setGroup(names, !all)}
											>
												{all ? "Clear" : "Select all"}
											</Button>
										</Box>
										<Box as="ul" className="divide-y divide-border">
											{visible.map((permission) => {
												const checkboxId = `${id}-${permission.name}`;
												return (
													<Box
														as="li"
														key={permission.name}
														className="flex items-start gap-3 px-4 py-3"
													>
														<Box className="pt-0.5">
															<Checkbox
																id={checkboxId}
																checked={chosen.has(permission.name)}
																onCheckedChange={(checked) =>
																	toggle(permission.name, checked === true)
																}
															/>
														</Box>
														<Box className="min-w-0 flex-1">
															<FieldLabel htmlFor={checkboxId}>
																<Box
																	as="span"
																	className="font-mono [overflow-wrap:anywhere]"
																>
																	{permission.name}
																</Box>
															</FieldLabel>
															{permission.description && (
																<Box className="[overflow-wrap:anywhere]">
																	<Text tone="muted" size="xs">
																		{permission.description}
																	</Text>
																</Box>
															)}
														</Box>
													</Box>
												);
											})}
										</Box>
									</Box>
								);
							})}
						</Stack>
					)}

					{!term && emptyTypes.length > 0 && (
						<Text tone="muted" size="xs">
							No permissions scoped to {listNames(emptyTypes)} yet.
						</Text>
					)}
				</>
			)}
		</Stack>
	);
}
