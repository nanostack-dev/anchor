import { updateProductResourceType } from "@/client";
import {
	replaceProductResourceTypesMutation,
	searchProductResourcePermissionsOptions,
	searchProductRolesOptions,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { ConfirmDialog } from "@nanostackorg/design-system/blocks/confirm-dialog";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Button } from "@nanostackorg/design-system/components/button";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Text } from "@nanostackorg/design-system/components/text";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { PencilSimpleIcon, WarningCircleIcon } from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { ResourceTypeCanvas } from "./ResourceTypeCanvas";
import { ResourceTypeInspector, type TypeUsage } from "./ResourceTypeInspector";
import type { ActionSource, CanvasActions } from "./ResourceTypeNode";
import { resourceTypesQueryKey, useResourceTypes } from "./ResourceTypeSelect";
import { publishProblems } from "./publish-errors";
import { layoutTree, neighbour } from "./resource-type-layout";
import {
	type TreeType,
	addChild,
	addChildBlocker,
	childrenOf,
	deletedSlugs,
	describeDraft,
	draftProblems,
	hasChanges,
	plural,
	removeType,
	rename,
	setSlug,
	summarizeDraft,
	switchChanges,
	toggleParent,
	treeChanged,
	treeFromResponse,
	treeRequest,
	updateType,
} from "./resource-type-tree";

interface EditorState {
	draft: TreeType[] | null;
	selected: string | null;
	naming: string | null;
	entering: ReadonlySet<string>;
	instant: boolean;
	focus: { key: string; id: number } | null;
	deleting: { key: string; source: ActionSource } | null;
	announcement: string;
	nextKey: number;
	fresh: string | null;
	toolbarFocus: number;
}

const INITIAL: EditorState = {
	draft: null,
	selected: null,
	naming: null,
	entering: new Set(),
	instant: false,
	focus: null,
	deleting: null,
	announcement: "",
	nextKey: 1,
	fresh: null,
	toolbarFocus: 0,
};

const shape = (types: TreeType[]) =>
	types
		.map(
			(type) =>
				`${type.key}:${type.published ? "" : type.slug}<${type.parents.join(",")}`,
		)
		.join(";");

function without(set: ReadonlySet<string>, key: string) {
	const next = new Set(set);
	next.delete(key);
	return next;
}

function useUsage(
	productId: string,
	type: TreeType | null,
	published: boolean,
): TypeUsage {
	const enabled = Boolean(type && published);
	const body = {
		filter: { resource_types: type ? [type.slug] : [] },
		pagination: { limit: 1 },
	};
	const roles = useQuery({
		...searchProductRolesOptions({ path: { product_id: productId }, body }),
		enabled,
		select: (data) => data.total,
	});
	const permissions = useQuery({
		...searchProductResourcePermissionsOptions({
			path: { product_id: productId },
			body,
		}),
		enabled,
		select: (data) => data.total,
	});
	if (!enabled) return { state: "unpublished" };
	if (roles.isError || permissions.isError)
		return {
			state: "error",
			retry: () => {
				void roles.refetch();
				void permissions.refetch();
			},
		};
	if (roles.data === undefined || permissions.data === undefined)
		return { state: "loading" };
	return { state: "ready", roles: roles.data, permissions: permissions.data };
}

function deleteBlockersFor(
	types: TreeType[],
	type: TreeType | null,
	usage: TypeUsage,
) {
	if (!type || type.builtIn) return [];
	const name = type.name || "this type";
	const reasons: string[] = [];
	const children = childrenOf(types, type.key).length;
	if (children)
		reasons.push(
			`${plural(children, "type sits", "types sit")} under ${name}.`,
		);
	if (usage.state === "loading")
		reasons.push("Checking which roles and permissions use it…");
	if (usage.state === "error")
		reasons.push("Couldn't check which roles and permissions use it.");
	if (usage.state === "ready") {
		if (usage.roles)
			reasons.push(
				`${plural(usage.roles, "role is", "roles are")} scoped to ${name}.`,
			);
		if (usage.permissions)
			reasons.push(
				`${plural(usage.permissions, "permission is", "permissions are")} scoped to ${name}.`,
			);
	}
	return reasons;
}

function BlockingRoles({
	productId,
	roleIds,
}: {
	productId: string;
	roleIds: string[];
}) {
	const roles = useQuery({
		...searchProductRolesOptions({
			path: { product_id: productId },
			body: { filter: { ids: roleIds }, pagination: { limit: 50 } },
		}),
		select: (data) => ({
			names: data.items.map((role) => role.name),
			managed: data.total,
		}),
	});
	if (!roles.data) return null;
	const { names, managed } = roles.data;
	const organizationRoles = roleIds.length - managed;
	return (
		<>
			{names.length > 0 && (
				<Box as="span" className="block [overflow-wrap:anywhere]">
					{`${names.length === 1 ? "Role" : "Roles"} to fix: ${names.join(", ")}.`}
				</Box>
			)}
			{organizationRoles > 0 && (
				<Box as="span" className="block">
					{`${plural(organizationRoles, "role", "roles")} that Organizations created also ${organizationRoles === 1 ? "needs" : "need"} changing. Each Organization changes its own roles.`}
				</Box>
			)}
		</>
	);
}

export interface ResourceTypesEditorProps {
	productId: string;
}

export function ResourceTypesEditor({ productId }: ResourceTypesEditorProps) {
	const queryClient = useQueryClient();
	const query = useResourceTypes(productId);
	const published = useMemo(
		() => (query.data ? treeFromResponse(query.data) : []),
		[query.data],
	);
	const [state, setState] = useState<EditorState>(INITIAL);
	const stateRef = useRef(state);
	stateRef.current = state;
	const types = state.draft ?? published;
	const editing = state.draft !== null;
	const typeShape = shape(types);
	// biome-ignore lint/correctness/useExhaustiveDependencies: the layout follows the tree's shape only
	const layout = useMemo(() => layoutTree(types), [typeShape]);
	const layoutRef = useRef(layout);
	layoutRef.current = layout;
	const typesRef = useRef(types);
	typesRef.current = types;
	const publishedRef = useRef(published);
	publishedRef.current = published;
	const reserved = state.draft
		? deletedSlugs(published, state.draft)
		: new Set<string>();
	const toolbar = useRef<HTMLDivElement>(null);
	useEffect(() => {
		if (state.toolbarFocus) toolbar.current?.querySelector("button")?.focus();
	}, [state.toolbarFocus]);

	const selected = types.find((type) => type.key === state.selected) ?? null;
	const selectedPublished = Boolean(
		selected && published.some((type) => type.key === selected.key),
	);
	const usage = useUsage(productId, selected, selectedPublished);
	const deleteBlockers = deleteBlockersFor(types, selected, usage);
	const lastDeleting = useRef<TreeType | undefined>(undefined);
	const usageRef = useRef({
		key: selected?.key,
		blockers: deleteBlockers,
		loading: usage.state === "loading",
	});
	usageRef.current = {
		key: selected?.key,
		blockers: deleteBlockers,
		loading: usage.state === "loading",
	};

	const summary = state.draft ? summarizeDraft(published, state.draft) : null;
	const problems = state.draft
		? draftProblems(
				state.draft.filter((type) => type.key !== state.naming),
				reserved,
			)
		: [];

	const publish = useMutation({
		...replaceProductResourceTypesMutation(),
		onSuccess: (data) => {
			const tree = data.resource_types;
			const draft = stateRef.current.draft ?? [];
			const selectedSlug = draft.find(
				(type) => type.key === stateRef.current.selected,
			)?.slug;
			queryClient.setQueryData(resourceTypesQueryKey(productId), {
				items: tree,
				total: tree.length,
				count: tree.length,
			});
			void queryClient.invalidateQueries({
				predicate: (cached) => {
					const [key] = cached.queryKey as [{ _id?: string }];
					return (
						key?._id === "searchProductResourceTypes" ||
						key?._id === "searchProductRoles" ||
						key?._id === "searchProductResourcePermissions"
					);
				},
			});
			setState((current) => ({
				...current,
				draft: null,
				naming: null,
				fresh: null,
				entering: new Set(),
				instant: true,
				selected: tree.find((type) => type.slug === selectedSlug)?.id ?? null,
				toolbarFocus: current.toolbarFocus + 1,
				announcement: "Resource types published.",
			}));
			toast.add({ type: "success", title: "Resource types published" });
		},
	});
	const switches = useMutation({
		mutationFn: async (changed: TreeType[]) => {
			for (const type of changed)
				await updateProductResourceType({
					path: { product_id: productId, resource_type_id: type.key },
					body: { allows_custom_roles: type.customRoles },
					throwOnError: true,
				});
		},
		onSettled: () =>
			queryClient.invalidateQueries({
				queryKey: resourceTypesQueryKey(productId),
			}),
	});
	const closeDraft = (announcement: string) => {
		setState((current) => ({
			...current,
			draft: null,
			naming: null,
			fresh: null,
			entering: new Set(),
			instant: true,
			toolbarFocus: current.toolbarFocus + 1,
			announcement,
		}));
		toast.add({ type: "success", title: "Resource types published" });
	};
	const publishDraft = async () => {
		const draft = state.draft;
		if (!draft) return;
		const changedSwitches = switchChanges(published, draft);
		const changedTree = treeChanged(summarizeDraft(published, draft));
		if (changedSwitches.length) {
			try {
				await switches.mutateAsync(changedSwitches);
			} catch {
				return;
			}
			if (changedTree)
				toast.add({
					type: "success",
					title: "Custom roles saved",
					description: "Publishing the tree next.",
				});
		}
		if (changedTree)
			publish.mutate({
				path: { product_id: productId },
				body: { resource_types: treeRequest(draft) },
			});
		else closeDraft("Custom roles saved.");
	};
	const announce = useCallback((announcement: string) => {
		setState((current) => ({ ...current, announcement }));
	}, []);

	const actions = useMemo<CanvasActions>(
		() => ({
			select: (key) =>
				setState((current) =>
					current.selected === key ? current : { ...current, selected: key },
				),
			move: (key, direction) => {
				const next = neighbour(
					typesRef.current,
					layoutRef.current,
					key,
					direction,
				);
				if (!next) return;
				setState((current) => ({
					...current,
					selected: next,
					focus: { key: next, id: (current.focus?.id ?? 0) + 1 },
				}));
			},
			addChild: (parentKey, source) =>
				setState((current) => {
					if (!current.draft) return current;
					const blocker = addChildBlocker(current.draft, parentKey);
					if (blocker) return { ...current, announcement: blocker };
					const key = `draft_${current.nextKey}`;
					return {
						...current,
						draft: addChild(current.draft, parentKey, key),
						nextKey: current.nextKey + 1,
						selected: key,
						naming: key,
						fresh: key,
						instant: source === "keyboard",
						entering:
							source === "pointer"
								? new Set([...current.entering, key])
								: current.entering,
					};
				}),
			startRename: (key) =>
				setState((current) =>
					current.draft ? { ...current, naming: key, selected: key } : current,
				),
			commitName: (key, name, source) =>
				setState((current) => {
					if (!current.draft) return current;
					return {
						...current,
						draft: rename(
							current.draft,
							key,
							name,
							deletedSlugs(publishedRef.current, current.draft),
						),
						naming: null,
						fresh: current.fresh === key ? null : current.fresh,
						instant: true,
						entering: without(current.entering, key),
						focus:
							source === "keyboard"
								? { key, id: (current.focus?.id ?? 0) + 1 }
								: current.focus,
					};
				}),
			cancelName: (key) =>
				setState((current) => {
					if (!current.draft) return current;
					const type = current.draft.find((item) => item.key === key);
					const focus = (target: string | undefined) =>
						target
							? { key: target, id: (current.focus?.id ?? 0) + 1 }
							: current.focus;
					if (type && current.fresh === key) {
						const parent = type.parents[0];
						return {
							...current,
							draft: removeType(current.draft, key),
							naming: null,
							fresh: null,
							selected: parent ?? null,
							instant: true,
							entering: without(current.entering, key),
							focus: focus(parent),
						};
					}
					return { ...current, naming: null, focus: focus(key) };
				}),
			requestDelete: (key, source) => {
				const type = typesRef.current.find((item) => item.key === key);
				if (!type) return;
				if (type.builtIn) {
					announce("Organization is built in and can't be deleted.");
					return;
				}
				const { key: usageKey, blockers } = usageRef.current;
				if (usageKey !== key) return;
				if (blockers.length) {
					announce(
						usageRef.current.loading
							? `Still checking what uses ${type.name}. Try again in a moment.`
							: `Can't delete ${type.name}: ${blockers.join(" ")}`,
					);
					return;
				}
				setState((current) => ({ ...current, deleting: { key, source } }));
			},
		}),
		[announce],
	);

	const keyboardInput = useRef(false);
	const changeDraft = (change: (draft: TreeType[]) => TreeType[]) =>
		setState((current) =>
			current.draft
				? {
						...current,
						draft: change(current.draft),
						instant: keyboardInput.current,
					}
				: current,
		);

	const startEditing = () => {
		publish.reset();
		switches.reset();
		setState((current) => {
			const target =
				published.find((type) => type.key === current.selected)?.key ??
				published[0]?.key;
			return {
				...current,
				draft: published,
				focus: target
					? { key: target, id: (current.focus?.id ?? 0) + 1 }
					: current.focus,
				announcement:
					"Editing resource types. Changes stay a draft until you publish.",
			};
		});
	};

	const discard = () => {
		publish.reset();
		switches.reset();
		setState((current) => ({
			...current,
			draft: null,
			naming: null,
			fresh: null,
			entering: new Set(),
			instant: true,
			selected: published.some((type) => type.key === current.selected)
				? current.selected
				: null,
			toolbarFocus: current.toolbarFocus + 1,
			announcement: "Draft discarded.",
		}));
	};

	const confirmDelete = () => {
		const deleting = state.deleting;
		if (!deleting) return;
		setState((current) => {
			if (!current.draft) return current;
			const type = current.draft.find((item) => item.key === deleting.key);
			const parent = type?.parents[0] ?? null;
			return {
				...current,
				draft: removeType(current.draft, deleting.key),
				deleting: null,
				selected: parent,
				instant: deleting.source === "keyboard",
				focus: parent
					? { key: parent, id: (current.focus?.id ?? 0) + 1 }
					: current.focus,
				announcement: `${type?.name ?? "Type"} deleted from the draft.`,
			};
		});
	};

	if (query.isPending)
		return (
			<output
				className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]"
				aria-busy="true"
				aria-label="Loading resource types"
			>
				<Box className="h-[26rem] md:h-[32rem]">
					<Skeleton height="fill" />
				</Box>
				<Skeleton height="xxl" />
			</output>
		);

	if (query.isError)
		return (
			<Alert tone="critical" icon={WarningCircleIcon}>
				<AlertTitle>Couldn't load the resource types</AlertTitle>
				<AlertDescription>
					<Stack space="sm" align="start">
						<span>
							Anchor didn't answer. Check your connection and try again.
						</span>
						<Button size="sm" variant="outline" onClick={() => query.refetch()}>
							Try again
						</Button>
					</Stack>
				</AlertDescription>
			</Alert>
		);

	const nameOf = (slug: string) =>
		types.find((type) => type.slug === slug)?.name ?? slug;
	const failures =
		publish.isError &&
		state.draft &&
		JSON.stringify(publish.variables?.body.resource_types) ===
			JSON.stringify(treeRequest(state.draft))
			? publishProblems(publish.error, nameOf)
			: [];
	const publishing = publish.isPending || switches.isPending;
	const locked = editing && publishing;
	const switchFailure =
		switches.isError &&
		state.draft &&
		switchChanges(published, state.draft).length > 0
			? (getApiErrorMessage(switches.error) ?? "Try again in a moment.")
			: undefined;
	const deletingType = types.find((type) => type.key === state.deleting?.key);
	if (deletingType) lastDeleting.current = deletingType;
	const dialogType = deletingType ?? lastDeleting.current;

	return (
		<Stack
			space="md"
			onKeyDownCapture={() => {
				keyboardInput.current = true;
			}}
			onPointerDownCapture={() => {
				keyboardInput.current = false;
			}}
		>
			<Box className="flex flex-wrap items-center justify-between gap-3">
				<Box className="min-w-0">
					{summary ? (
						<Text weight="medium" aria-live="polite">
							Draft: {describeDraft(summary)}
						</Text>
					) : (
						<Text tone="muted">
							{plural(types.length, "type", "types")}, Organization first.
						</Text>
					)}
				</Box>
				<Box ref={toolbar}>
					{editing ? (
						<Inline space="sm">
							<Button variant="outline" onClick={discard} disabled={publishing}>
								Discard
							</Button>
							<Button
								variant="solid"
								tone="brand"
								loading={publishing}
								disabled={
									!summary ||
									!hasChanges(summary) ||
									problems.length > 0 ||
									state.naming !== null
								}
								onClick={() => void publishDraft()}
							>
								Publish
							</Button>
						</Inline>
					) : (
						<Button icon={PencilSimpleIcon} onClick={startEditing}>
							Edit resource types
						</Button>
					)}
				</Box>
			</Box>

			{editing && (
				<Text tone="muted" size="xs">
					Hover or select a type, then use + to add a type under it. Keys:
					arrows move, Enter renames, N adds a type under, Delete removes.
				</Text>
			)}

			{switchFailure && (
				<Alert tone="critical" icon={WarningCircleIcon}>
					<AlertTitle>Anchor didn't save the custom roles switch</AlertTitle>
					<AlertDescription>{switchFailure}</AlertDescription>
				</Alert>
			)}

			{failures.length > 0 && (
				<Alert tone="critical" icon={WarningCircleIcon}>
					<AlertTitle>Anchor didn't publish this tree</AlertTitle>
					<AlertDescription>
						<Stack as="ul" space="xs">
							{failures.map((failure) => (
								<Box
									as="li"
									key={`${failure.code}-${failure.message}`}
									className="[overflow-wrap:anywhere]"
								>
									{failure.message}
									{failure.roleIds.length > 0 && (
										<BlockingRoles
											productId={productId}
											roleIds={failure.roleIds}
										/>
									)}
								</Box>
							))}
						</Stack>
					</AlertDescription>
				</Alert>
			)}

			{editing && problems.length > 0 && (
				<Alert tone="warning" icon={WarningCircleIcon}>
					<AlertTitle>Fix these before publishing</AlertTitle>
					<AlertDescription>
						<Stack as="ul" space="xs">
							{problems.map((problem) => (
								<Box as="li" key={problem} className="[overflow-wrap:anywhere]">
									{problem}
								</Box>
							))}
						</Stack>
					</AlertDescription>
				</Alert>
			)}

			<Box className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
				<Box className="h-[26rem] min-w-0 overflow-hidden rounded-2xl border border-border bg-surface-subtle md:h-[32rem] lg:h-[36rem]">
					<ResourceTypeCanvas
						types={types}
						layout={layout}
						editing={editing && !locked}
						selectedKey={state.selected}
						namingKey={state.naming}
						entering={state.entering}
						instant={state.instant}
						focusRequest={state.focus}
						actions={actions}
					/>
				</Box>
				<ResourceTypeInspector
					type={selected}
					types={types}
					editing={editing && !locked}
					reservedSlugs={reserved}
					usage={usage}
					naming={selected?.key === state.naming}
					deleteBlockers={deleteBlockers}
					addBlocker={
						editing && selected
							? addChildBlocker(types, selected.key)
							: undefined
					}
					onRename={(name) =>
						selected &&
						changeDraft((draft) =>
							rename(draft, selected.key, name, deletedSlugs(published, draft)),
						)
					}
					onSlugChange={(slug) =>
						selected &&
						changeDraft((draft) => setSlug(draft, selected.key, slug))
					}
					onToggleParent={(parentKey) =>
						selected &&
						changeDraft((draft) => toggleParent(draft, selected.key, parentKey))
					}
					onNestsChange={(nests) =>
						selected &&
						changeDraft((draft) =>
							updateType(draft, selected.key, (type) => ({
								...type,
								nestsInItself: nests,
							})),
						)
					}
					onCustomRolesChange={(on) =>
						selected &&
						changeDraft((draft) =>
							updateType(draft, selected.key, (type) => ({
								...type,
								customRoles: on,
							})),
						)
					}
					onAddChild={() =>
						selected && actions.addChild(selected.key, "pointer")
					}
					onDelete={() =>
						selected && actions.requestDelete(selected.key, "pointer")
					}
				/>
			</Box>

			<ConfirmDialog
				open={Boolean(deletingType)}
				onOpenChange={(open) =>
					!open && setState((current) => ({ ...current, deleting: null }))
				}
				tone="critical"
				title={`Delete ${dialogType?.name ?? "type"}?`}
				description={
					dialogType?.published
						? "It leaves the draft now. Anchor deletes it when you publish."
						: "It hasn't been published, so only the draft changes."
				}
				confirmLabel="Delete type"
				onConfirm={confirmDelete}
			/>

			<output className="sr-only" aria-live="polite">
				{state.announcement}
			</output>
		</Stack>
	);
}
