import { SortDirection } from "@/client";
import {
	searchProductPermissionsOptions,
	searchProductResourcePermissionsOptions,
} from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import { productPermissionsRoute } from "@/routes/products/permissions";
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
	const checkboxRef = useRef<HTMLButtonElement>(null);

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

	useEffect(() => {
		if (checkboxRef.current) {
			const currentSelected = form.state.values.selectedPermissions;
			const allVisibleSelected =
				filteredPermissions.length > 0 &&
				filteredPermissions.every((p) => currentSelected.includes(p.name));
			const someVisibleSelected = filteredPermissions.some((p) =>
				currentSelected.includes(p.name),
			);
			(
				checkboxRef.current as HTMLButtonElement & { indeterminate: boolean }
			).indeterminate = someVisibleSelected && !allVisibleSelected;
		}
	}, [filteredPermissions, form.state.values.selectedPermissions]);

	const renderSelectedPermissionsSummary = () => {
		const currentSelected = form.state.values.selectedPermissions;
		if (currentSelected.length === 0) return null;

		return (
			<div className="rounded-xl border border-slate-200 bg-slate-50/60 overflow-hidden">
				<button
					type="button"
					onClick={() =>
						setSelectedPermissionsExpanded(!selectedPermissionsExpanded)
					}
					className="flex items-center justify-between w-full px-4 py-3 text-left hover:bg-slate-100/60 transition-colors"
				>
					<div className="flex items-center gap-2.5">
						<Badge
							variant="secondary"
							className="px-2 py-0.5 text-xs bg-slate-900 text-white border-0"
						>
							{currentSelected.length}
						</Badge>
						<span className="text-sm font-medium text-slate-700">
							Selected Permissions
						</span>
					</div>
					{selectedPermissionsExpanded ? (
						<ChevronUp className="h-4 w-4 text-slate-400" />
					) : (
						<ChevronDown className="h-4 w-4 text-slate-400" />
					)}
				</button>

				{selectedPermissionsExpanded && (
					<div className="px-4 pb-4 pt-1 border-t border-slate-200">
						<div className="flex flex-wrap gap-1.5 mt-2">
							{currentSelected.map((permissionName: string) => (
								<Badge
									key={permissionName}
									variant="secondary"
									className="text-xs flex items-center gap-1 px-2.5 py-1 bg-white border border-slate-200 text-slate-700 hover:bg-red-50 hover:border-red-200 hover:text-red-600 transition-colors cursor-pointer rounded-lg font-mono"
									onClick={() => handlePermissionToggle(permissionName)}
								>
									{permissionName}
									<X className="h-3 w-3" />
								</Badge>
							))}
						</div>
					</div>
				)}
			</div>
		);
	};

	const renderPermissionsList = () => {
		if (permissionsLoading) {
			return (
				<div className="flex flex-col items-center justify-center h-40 rounded-xl border border-slate-200 bg-slate-50/40">
					<div className="animate-spin rounded-full h-7 w-7 border-2 border-slate-200 border-t-slate-900 mb-3" />
					<p className="text-sm text-slate-400">Loading permissions...</p>
				</div>
			);
		}

		if (availablePermissions.length === 0) {
			return (
				<div className="flex flex-col items-center justify-center h-64 rounded-xl border border-slate-200 bg-slate-50/40 text-center px-6">
					<div className="p-3 rounded-2xl bg-slate-100 mb-4">
						<Settings className="h-8 w-8 text-slate-400" />
					</div>
					<p className="text-sm font-semibold text-slate-700">
						No permissions configured
					</p>
					<p className="text-xs text-slate-400 mt-1">
						Set up permissions first to assign them to {itemType}s
					</p>
					{showNavigateToPermissions && (
						<Button
							type="button"
							variant="outline"
							onClick={() => {
								void navigate({ to: productPermissionsRoute.fullPath });
							}}
							className="mt-4 h-9 rounded-xl border-slate-200 text-slate-600 hover:bg-slate-50 text-sm"
						>
							<ExternalLink className="mr-2 h-3.5 w-3.5" />
							Go to Permissions
						</Button>
					)}
				</div>
			);
		}

		if (filteredPermissions.length === 0) {
			return (
				<div className="flex flex-col items-center justify-center h-40 rounded-xl border border-slate-200 bg-slate-50/40">
					<Search className="h-8 w-8 text-slate-300 mb-3" />
					<p className="text-sm font-medium text-slate-500">
						No permissions found
					</p>
					<p className="text-xs text-slate-400 mt-1">
						Try adjusting your search or filter
					</p>
				</div>
			);
		}

		const currentSelected = form.state.values.selectedPermissions;
		const allVisibleSelected = filteredPermissions.every((p) =>
			currentSelected.includes(p.name),
		);

		return (
			<div className="space-y-3">
				{/* Toolbar */}
				<div className="flex items-center justify-between px-3 py-2.5 bg-slate-50 rounded-xl border border-slate-200">
					<div className="flex items-center gap-2.5">
						<Checkbox
							checked={allVisibleSelected}
							onCheckedChange={handleSelectAll}
							ref={checkboxRef}
							className="h-4 w-4 rounded border-slate-300"
						/>
						<span className="text-xs font-medium text-slate-600">
							All visible
						</span>
						<Badge
							variant="outline"
							className="text-xs border-slate-200 text-slate-500 bg-white"
						>
							{
								filteredPermissions.filter((p) =>
									currentSelected.includes(p.name),
								).length
							}
							/{filteredPermissions.length}
						</Badge>
					</div>
					<div className="flex items-center gap-1">
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => handleSortChange("name")}
							className="text-xs h-7 px-2.5 text-slate-500 hover:text-slate-900 hover:bg-slate-100 rounded-lg"
						>
							Name {sortBy === "name" && (sortDirection === "ASC" ? "↑" : "↓")}
						</Button>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => handleSortChange("selected")}
							className="text-xs h-7 px-2.5 text-slate-500 hover:text-slate-900 hover:bg-slate-100 rounded-lg"
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
							className="text-xs h-7 px-2.5 text-slate-500 hover:text-slate-900 hover:bg-slate-100 rounded-lg"
						>
							{allVisibleSelected ? "Deselect All" : "Select All"}
						</Button>
					</div>
				</div>

				{/* Permissions list */}
				<div className="rounded-xl border border-slate-200 overflow-hidden">
					<div className="max-h-80 overflow-y-auto">
						<div className="divide-y divide-slate-100">
							{filteredPermissions.map((permission) => {
								const isSelected = currentSelected.includes(permission.name);
								const wasOriginallyAssigned =
									isEditMode &&
									existingItem?.permissions?.some(
										(p) => p.permission_name === permission.name,
									);

								return (
									<div
										key={permission.name}
										className={`flex items-start gap-3 px-4 py-3.5 cursor-pointer transition-colors ${
											isSelected
												? "bg-slate-50"
												: "bg-white hover:bg-slate-50/60"
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
											className="h-4 w-4 mt-0.5 rounded border-slate-300 shrink-0"
										/>
										<div className="flex-1 min-w-0">
											<div className="flex items-center flex-wrap gap-2">
												<code className="text-xs bg-slate-100 text-slate-700 px-2 py-0.5 rounded-md font-mono">
													{permission.name}
												</code>
												{isSelected && (
													<Badge
														variant="outline"
														className="text-xs h-5 px-1.5 border-slate-200 text-slate-500 bg-white"
													>
														Selected
													</Badge>
												)}
												{wasOriginallyAssigned && (
													<Badge
														variant="secondary"
														className="text-xs h-5 px-1.5 bg-slate-100 text-slate-500 border-0"
													>
														Previously assigned
													</Badge>
												)}
											</div>
											{permission.description && (
												<p className="text-xs text-slate-400 mt-1 leading-relaxed">
													{permission.description}
												</p>
											)}
										</div>
									</div>
								);
							})}
						</div>
					</div>
				</div>
			</div>
		);
	};

	return (
		<VerticalStepperStep id={stepId}>
			<div className="flex flex-col h-full">
				{/* Header */}
				<div className="px-7 pt-7 pb-5">
					<DialogHeader className="space-y-2">
						<DialogTitle className="flex items-center gap-3">
							<div className="p-2 rounded-xl bg-slate-900 text-white shadow-sm">
								<Shield className="h-4 w-4" />
							</div>
							<span className="text-xl font-semibold tracking-tight text-slate-900">
								{title}
							</span>
							{isEditMode && existingItem && (
								<Badge
									variant="outline"
									className="ml-1 text-xs font-normal border-slate-200 text-slate-500 bg-slate-50"
								>
									Editing: {existingItem.name}
								</Badge>
							)}
						</DialogTitle>
						<DialogDescription className="text-sm text-slate-500 leading-relaxed">
							{description}
						</DialogDescription>
					</DialogHeader>
				</div>

				<ScrollArea className="flex-1 px-7">
					<div className="space-y-4 pb-6">
						{/* Summary bar */}
						<div className="flex items-center justify-between px-4 py-3 rounded-xl border border-slate-200 bg-slate-50/60">
							<div>
								<p className="text-sm font-medium text-slate-700">
									Permissions
								</p>
								<p className="text-xs text-slate-400 mt-0.5">
									{form.state.values.selectedPermissions.length} of{" "}
									{availablePermissions.length} selected
								</p>
							</div>
							<Badge
								variant="secondary"
								className="text-xs bg-slate-900 text-white border-0 px-2.5 py-1"
							>
								{form.state.values.selectedPermissions.length} selected
							</Badge>
						</div>

						{/* Search + filter */}
						<div className="flex gap-2">
							<div className="relative flex-1">
								<Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-slate-400" />
								<Input
									placeholder="Search permissions..."
									value={searchTerm}
									onChange={(e) => setSearchTerm(e.target.value)}
									className="pl-9 h-9 text-sm border-slate-200 bg-white placeholder:text-slate-400 focus-visible:ring-slate-900/20 focus-visible:border-slate-400 rounded-xl"
								/>
							</div>
							<div className="flex gap-1.5">
								<Button
									type="button"
									variant={filterSelected === "all" ? "default" : "outline"}
									size="sm"
									onClick={() => setFilterSelected("all")}
									className={`h-9 px-3.5 rounded-xl text-xs font-medium ${
										filterSelected === "all"
											? "bg-slate-900 hover:bg-slate-800 text-white border-0"
											: "border-slate-200 text-slate-600 hover:bg-slate-50"
									}`}
								>
									All
								</Button>
								<Button
									type="button"
									variant={
										filterSelected === "selected" ? "default" : "outline"
									}
									size="sm"
									onClick={() => setFilterSelected("selected")}
									className={`h-9 px-3.5 rounded-xl text-xs font-medium ${
										filterSelected === "selected"
											? "bg-slate-900 hover:bg-slate-800 text-white border-0"
											: "border-slate-200 text-slate-600 hover:bg-slate-50"
									}`}
								>
									Selected
								</Button>
							</div>
						</div>

						{renderSelectedPermissionsSummary()}
						{renderPermissionsList()}
					</div>
				</ScrollArea>

				{showFormValidationError && (
					<form.Field name="selectedPermissions">
						{(field) => <FormValidationError field={field} />}
					</form.Field>
				)}

				{/* Footer */}
				<div className="px-7 py-5 border-t border-slate-100 mt-auto">
					<DialogFooter className="p-0">
						<div className="flex items-center justify-between w-full">
							<Button
								type="button"
								variant="ghost"
								onClick={onPrevious}
								className="px-4 h-10 rounded-xl text-slate-600 hover:text-slate-900 hover:bg-slate-100"
							>
								<ChevronLeft className="mr-1.5 h-4 w-4" />
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
										type="button"
										onClick={async () => {
											await form.handleSubmit();
										}}
										className="px-6 h-10 rounded-xl bg-slate-900 hover:bg-slate-800 text-white font-medium shadow-sm transition-all"
										disabled={
											!canSubmit ||
											isSubmitting ||
											!isValid ||
											(!isEditMode && !isDirty) ||
											selectedPermissions.length === 0
										}
									>
										Continue
										<ChevronRight className="ml-1.5 h-4 w-4" />
									</Button>
								)}
							</form.Subscribe>
						</div>
					</DialogFooter>
				</div>
			</div>
		</VerticalStepperStep>
	);
}
