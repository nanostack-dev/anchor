import { SortDirection } from "@/client";
import {
	searchProductPermissionsOptions,
	searchProductResourcePermissionsOptions,
} from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "@nanostackorg/design-system/components/input-group";
import { ScrollArea } from "@nanostackorg/design-system/components/scroll-area";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { TabsContent } from "@nanostackorg/design-system/components/tabs";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useForm, useStore } from "@tanstack/react-form";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	ChevronUp,
	ExternalLink,
	Search,
	Settings,
	Shield,
	X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import z from "zod";

const selectPermissionsSchema = z.object({
	selectedPermissions: z
		.array(z.string())
		.min(1, "At least one permission must be selected"),
});

type PermissionsFormData = z.infer<typeof selectPermissionsSchema>;

export interface ExistingItem {
	name: string;
	permissions?: Array<{ permission_name: string }>;
}

interface PermissionsStepProps {
	productId: string;
	onFormDataChange: (data: PermissionsFormData) => void;
	isEditMode?: boolean;
	existingItem?: ExistingItem;
	initialSelectedPermissions?: string[];
	onNext: () => void;
	onPrevious: () => void;
	variant?: "product" | "resource";
	config?: {
		title?: string;
		description?: string;
		stepId?: string;
		showNavigateToPermissions?: boolean;
		itemType?: "API key" | "role";
		showSelectedPermissionsScrollArea?: boolean;
		showFormValidationError?: boolean;
	};
}

export function PermissionsStep({
	productId,
	onFormDataChange,
	isEditMode = false,
	existingItem,
	initialSelectedPermissions,
	onNext,
	onPrevious,
	variant,
	config = {},
}: PermissionsStepProps) {
	const navigate = useNavigate();
	const [searchTerm, setSearchTerm] = useState("");
	const [selectedPermissionsExpanded, setSelectedPermissionsExpanded] =
		useState(false);
	const [filterSelected, setFilterSelected] = useState<"all" | "selected">(
		"all",
	);
	const [sortBy, setSortBy] = useState<"name" | "selected">("name");
	const [sortDirection, setSortDirection] = useState<"ASC" | "DESC">("ASC");

	const seededPermissions = useMemo(
		() => [...new Set(initialSelectedPermissions ?? [])],
		[initialSelectedPermissions],
	);

	const seedKey = useMemo(
		() =>
			`${productId}:${existingItem?.name ?? ""}:${seededPermissions.join("|")}`,
		[productId, existingItem?.name, seededPermissions],
	);

	const lastAppliedSeedRef = useRef<string>("");

	const form = useForm({
		defaultValues: {
			selectedPermissions: seededPermissions,
		} as PermissionsFormData,
		onSubmit: async ({ value }) => {
			onFormDataChange(value);
			onNext();
		},
		validators: {
			onChange: selectPermissionsSchema,
			onSubmit: selectPermissionsSchema,
		},
	});

	const selectedPermissions = useStore(
		form.store,
		(state) => state.values.selectedPermissions,
	);

	useEffect(() => {
		if (lastAppliedSeedRef.current === seedKey) {
			return;
		}

		form.setFieldValue("selectedPermissions", seededPermissions);
		lastAppliedSeedRef.current = seedKey;
	}, [form, seedKey, seededPermissions]);

	const {
		title = "Permissions",
		description = `Select permissions to assign to this ${config.itemType ?? "item"}`,
		stepId = "permissions",
		showNavigateToPermissions = false,
		itemType = "item",
		showFormValidationError = false,
	} = config;

	const queryBody = {
		pagination: { limit: 1000, offset: 0 },
		sort_by: sortBy === "selected" ? "name" : sortBy,
		sort_direction:
			sortDirection === "ASC" ? SortDirection.ASC : SortDirection.DESC,
		full_text_search: searchTerm.trim() || undefined,
		filter:
			filterSelected === "selected"
				? {
						names:
							selectedPermissions.length > 0 ? selectedPermissions : undefined,
					}
				: undefined,
	};

	const { data: permissionsData, isLoading: permissionsLoading } =
		variant === "product"
			? useQuery({
					...searchProductPermissionsOptions({
						path: { product_id: productId },
						body: queryBody,
					}),
				})
			: useQuery({
					...searchProductResourcePermissionsOptions({
						path: { product_id: productId },
						body: queryBody,
					}),
				});

	const availablePermissions = permissionsData?.items ?? [];

	const filteredPermissions = useMemo(() => {
		return [...availablePermissions].sort((a, b) => {
			if (sortBy === "selected") {
				const aSelected = selectedPermissions.includes(a.name);
				const bSelected = selectedPermissions.includes(b.name);

				if (aSelected && !bSelected) return sortDirection === "ASC" ? -1 : 1;
				if (!aSelected && bSelected) return sortDirection === "ASC" ? 1 : -1;
				return a.name.localeCompare(b.name);
			}

			const comparison = a.name.localeCompare(b.name);
			return sortDirection === "ASC" ? comparison : -comparison;
		});
	}, [availablePermissions, selectedPermissions, sortBy, sortDirection]);

	const handlePermissionToggle = (permissionName: string) => {
		form.setFieldValue("selectedPermissions", (prev: string[]) => {
			if (prev.includes(permissionName)) {
				return prev.filter((name) => name !== permissionName);
			}
			return [...new Set([...prev, permissionName])];
		});
	};

	const handleSelectAll = (checked?: boolean | "indeterminate") => {
		const visiblePermissionNames = filteredPermissions.map((p) => p.name);
		const fieldValue = form.getFieldValue("selectedPermissions");
		const shouldSelect =
			checked === true ||
			(checked === undefined &&
				!filteredPermissions.every((p) => fieldValue.includes(p.name)));
		form.setFieldValue(
			"selectedPermissions",
			shouldSelect
				? [...new Set([...fieldValue, ...visiblePermissionNames])]
				: fieldValue.filter((name) => !visiblePermissionNames.includes(name)),
		);
	};

	const handleSortChange = (newSortBy: "name" | "selected") => {
		if (newSortBy === sortBy) {
			setSortDirection(sortDirection === "ASC" ? "DESC" : "ASC");
		} else {
			setSortBy(newSortBy);
			setSortDirection("ASC");
		}
	};

	const renderSelectedPermissionsSummary = () => {
		const currentSelected = form.state.values.selectedPermissions;
		if (currentSelected.length === 0) return null;

		return (
			<Box className="rounded-xl border border-border bg-muted/60 overflow-hidden">
				<button
					type="button"
					onClick={() =>
						setSelectedPermissionsExpanded(!selectedPermissionsExpanded)
					}
					className="flex items-center justify-between w-full px-4 py-3 text-left hover:bg-muted transition-colors"
				>
					<Box className="flex items-center gap-2.5">
						<Badge tone="neutral" variant="soft">
							{currentSelected.length}
						</Badge>
						<Box as="span" className="text-sm font-medium text-foreground">
							Selected Permissions
						</Box>
					</Box>
					{selectedPermissionsExpanded ? (
						<ChevronUp className="size-4 text-muted-foreground" />
					) : (
						<ChevronDown className="size-4 text-muted-foreground" />
					)}
				</button>

				{selectedPermissionsExpanded && (
					<Box className="px-4 pb-4 pt-1 border-t border-border">
						<Box className="flex flex-wrap gap-1.5 mt-2">
							{currentSelected.map((permissionName: string) => (
								<Badge
									tone="neutral"
									key={permissionName}
									variant="soft"
									onClick={() => handlePermissionToggle(permissionName)}
								>
									{permissionName}
									<X className="size-3" />
								</Badge>
							))}
						</Box>
					</Box>
				)}
			</Box>
		);
	};

	const renderPermissionsList = () => {
		if (permissionsLoading) {
			return (
				<Box className="flex flex-col items-center justify-center h-40 rounded-xl border border-border bg-muted/40 gap-3">
					<Spinner size="lg" />
					<Box as="p" className="text-sm text-muted-foreground">
						Loading permissions...
					</Box>
				</Box>
			);
		}

		if (availablePermissions.length === 0) {
			return (
				<Box className="flex flex-col items-center justify-center h-64 rounded-xl border border-border bg-muted/40 text-center px-6">
					<Box className="p-3 rounded-2xl bg-muted mb-4">
						<Settings className="size-8 text-muted-foreground" />
					</Box>
					<Box as="p" className="text-sm font-semibold text-foreground">
						No permissions configured
					</Box>
					<Box as="p" className="text-xs text-muted-foreground mt-1">
						Set up permissions first to assign them to {itemType}s
					</Box>
					{showNavigateToPermissions && (
						<Button
							type="button"
							variant="outline"
							onClick={() => {
								void navigate({ to: ROUTE_PATHS.PRODUCT_PERMISSIONS });
							}}
						>
							<ExternalLink className="mr-2 size-3.5" />
							Go to Permissions
						</Button>
					)}
				</Box>
			);
		}

		if (filteredPermissions.length === 0) {
			return (
				<Box className="flex flex-col items-center justify-center h-40 rounded-xl border border-border bg-muted/40">
					<Search className="size-8 text-muted-foreground mb-3" />
					<Box as="p" className="text-sm font-medium text-muted-foreground">
						No permissions found
					</Box>
					<Box as="p" className="text-xs text-muted-foreground mt-1">
						Try adjusting your search or filter
					</Box>
				</Box>
			);
		}

		const currentSelected = form.state.values.selectedPermissions;
		const allVisibleSelected = filteredPermissions.every((p) =>
			currentSelected.includes(p.name),
		);
		const someVisibleSelected = filteredPermissions.some((p) =>
			currentSelected.includes(p.name),
		);

		return (
			<Box className="space-y-3">
				{/* Toolbar */}
				<Box className="flex items-center justify-between px-3 py-2.5 bg-muted rounded-xl border border-border">
					<Box className="flex items-center gap-2.5">
						<Checkbox
							checked={allVisibleSelected}
							indeterminate={someVisibleSelected && !allVisibleSelected}
							onCheckedChange={handleSelectAll}
						/>
						<Box
							as="span"
							className="text-xs font-medium text-muted-foreground"
						>
							All visible
						</Box>
						<Badge variant="outline">
							{
								filteredPermissions.filter((p) =>
									currentSelected.includes(p.name),
								).length
							}
							/{filteredPermissions.length}
						</Badge>
					</Box>
					<Box className="flex items-center gap-1">
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => handleSortChange("name")}
						>
							Name {sortBy === "name" && (sortDirection === "ASC" ? "↑" : "↓")}
						</Button>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => handleSortChange("selected")}
						>
							Selected{" "}
							{sortBy === "selected" && (sortDirection === "ASC" ? "↑" : "↓")}
						</Button>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => handleSelectAll()}
							disabled={filteredPermissions.length === 0}
						>
							{allVisibleSelected ? "Deselect All" : "Select All"}
						</Button>
					</Box>
				</Box>

				{/* Permissions list */}
				<Box className="rounded-xl border border-border overflow-hidden">
					<Box className="max-h-80 overflow-y-auto">
						<Box className="divide-y divide-border">
							{filteredPermissions.map((permission) => {
								const isSelected = currentSelected.includes(permission.name);
								const wasOriginallyAssigned =
									isEditMode &&
									existingItem?.permissions?.some(
										(p) => p.permission_name === permission.name,
									);

								return (
									<Box
										key={permission.name}
										className={`flex items-start gap-3 px-4 py-3.5 cursor-pointer transition-colors ${
											isSelected
												? "bg-accent border-l-2 border-primary"
												: "bg-card hover:bg-muted/60"
										}`}
										onClick={() => handlePermissionToggle(permission.name)}
										onKeyDown={(e) => {
											if (e.key === "Enter" || e.key === " ") {
												e.preventDefault();
												handlePermissionToggle(permission.name);
											}
										}}
									>
										<Checkbox
											checked={isSelected}
											onCheckedChange={() =>
												handlePermissionToggle(permission.name)
											}
										/>
										<Box className="flex-1 min-w-0">
											<Box className="flex items-center flex-wrap gap-2">
												<code className="text-xs bg-muted text-foreground px-2 py-0.5 rounded-md font-mono">
													{permission.name}
												</code>
												{isSelected && (
													<Badge tone="success" variant="soft">
														Selected
													</Badge>
												)}
												{wasOriginallyAssigned && (
													<Badge tone="neutral" variant="soft">
														Previously assigned
													</Badge>
												)}
											</Box>
											{permission.description && (
												<Box
													as="p"
													className="text-xs text-muted-foreground mt-1 leading-relaxed"
												>
													{permission.description}
												</Box>
											)}
										</Box>
									</Box>
								);
							})}
						</Box>
					</Box>
				</Box>
			</Box>
		);
	};

	return (
		<TabsContent value={stepId}>
			<Box className="flex flex-col h-full">
				{/* Header */}
				<Box className="px-7 pt-7 pb-5">
					<Box className="flex flex-col space-y-2">
						<h2 className="flex items-center gap-3">
							<Box className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
								<Shield className="size-4" />
							</Box>
							<Box
								as="span"
								className="text-xl font-semibold tracking-tight text-foreground"
							>
								{title}
							</Box>
							{isEditMode && existingItem && (
								<Badge variant="outline">Editing: {existingItem.name}</Badge>
							)}
						</h2>
						<Box
							as="p"
							className="text-sm text-muted-foreground leading-relaxed"
						>
							{description}
						</Box>
					</Box>
				</Box>

				<ScrollArea maxHeight="md">
					<Box className="space-y-4 px-7 pb-6">
						{/* Summary bar */}
						<Box className="flex items-center justify-between px-4 py-3 rounded-xl border border-border bg-muted/60">
							<div>
								<Box as="p" className="text-sm font-medium text-foreground">
									Permissions
								</Box>
								<Box as="p" className="text-xs text-muted-foreground mt-0.5">
									{form.state.values.selectedPermissions.length} of{" "}
									{availablePermissions.length} selected
								</Box>
							</div>
							<Badge tone="neutral" variant="soft">
								{form.state.values.selectedPermissions.length} selected
							</Badge>
						</Box>

						{/* Search + filter */}
						<Box className="flex gap-2">
							<InputGroup>
								<InputGroupAddon>
									<Search aria-hidden />
								</InputGroupAddon>
								<InputGroupInput
									placeholder="Search permissions..."
									value={searchTerm}
									onChange={(e) => setSearchTerm(e.target.value)}
								/>
							</InputGroup>
							<Box className="flex gap-1.5">
								<Button
									tone="brand"
									type="button"
									variant={filterSelected === "all" ? "solid" : "outline"}
									size="sm"
									onClick={() => setFilterSelected("all")}
								>
									All
								</Button>
								<Button
									tone="brand"
									type="button"
									variant={filterSelected === "selected" ? "solid" : "outline"}
									size="sm"
									onClick={() => setFilterSelected("selected")}
								>
									Selected
								</Button>
							</Box>
						</Box>

						{renderSelectedPermissionsSummary()}
						{renderPermissionsList()}
					</Box>
				</ScrollArea>

				{showFormValidationError && (
					<form.Field name="selectedPermissions">
						{(field) => <FormValidationError field={field} />}
					</form.Field>
				)}

				{/* Footer */}
				<Box className="px-7 py-5 border-t border-border mt-auto">
					<Box className="p-0">
						<Box className="flex items-center justify-between w-full">
							<Button type="button" variant="ghost" onClick={onPrevious}>
								<ChevronLeft className="mr-1.5 size-4" />
								Back
							</Button>

							<form.Subscribe
								selector={(state) => [
									state.canSubmit,
									state.isSubmitting,
									state.isValid,
									state.isDirty,
								]}
							>
								{([canSubmit, isSubmitting, isValid, isDirty]) => (
									<Button
										variant="solid"
										tone="brand"
										type="button"
										onClick={async () => {
											await form.handleSubmit();
										}}
										disabled={
											!canSubmit ||
											isSubmitting ||
											!isValid ||
											(!isEditMode && !isDirty) ||
											selectedPermissions.length === 0
										}
									>
										Continue
										<ChevronRight className="ml-1.5 size-4" />
									</Button>
								)}
							</form.Subscribe>
						</Box>
					</Box>
				</Box>
			</Box>
		</TabsContent>
	);
}
