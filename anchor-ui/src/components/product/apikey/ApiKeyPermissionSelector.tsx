import { SortDirection } from "@/client";
import { searchProductPermissionsOptions } from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { ExternalLink, Search, Settings, X } from "lucide-react";
import { useMemo, useState } from "react";

interface ApiKeyPermissionSelectorProps {
	productId: string;
	value: string[];
	onChange: (next: string[]) => void;
	/** Permission names that were already assigned (edit mode), shown with a badge. */
	originalPermissions?: string[];
}

/**
 * Controlled, page-friendly permission picker for product API keys.
 * Unlike the dialog/stepper `PermissionsStep`, this component carries no
 * Dialog or stepper coupling and lays out responsively for mobile.
 */
export function ApiKeyPermissionSelector({
	productId,
	value,
	onChange,
	originalPermissions,
}: ApiKeyPermissionSelectorProps) {
	const navigate = useNavigate();
	const [searchTerm, setSearchTerm] = useState("");
	const [filter, setFilter] = useState<"all" | "selected">("all");

	const { data, isLoading } = useQuery({
		...searchProductPermissionsOptions({
			path: { product_id: productId },
			body: {
				pagination: { limit: 1000, offset: 0 },
				sort_by: "name",
				sort_direction: SortDirection.ASC,
			},
		}),
	});

	const allPermissions = useMemo(() => data?.items ?? [], [data]);

	const visiblePermissions = useMemo(() => {
		const term = searchTerm.trim().toLowerCase();
		return allPermissions
			.filter((p) => {
				if (filter === "selected" && !value.includes(p.name)) return false;
				if (!term) return true;
				return (
					p.name.toLowerCase().includes(term) ||
					(p.description ?? "").toLowerCase().includes(term)
				);
			})
			.sort((a, b) => a.name.localeCompare(b.name));
	}, [allPermissions, searchTerm, filter, value]);

	const toggle = (name: string) => {
		onChange(
			value.includes(name)
				? value.filter((n) => n !== name)
				: [...new Set([...value, name])],
		);
	};

	const allVisibleSelected =
		visiblePermissions.length > 0 &&
		visiblePermissions.every((p) => value.includes(p.name));

	const toggleAllVisible = () => {
		const visibleNames = visiblePermissions.map((p) => p.name);
		if (allVisibleSelected) {
			onChange(value.filter((n) => !visibleNames.includes(n)));
		} else {
			onChange([...new Set([...value, ...visibleNames])]);
		}
	};

	if (isLoading) {
		return (
			<div className="flex flex-col items-center justify-center h-40 rounded-xl border border-border bg-muted/40 gap-3">
				<Spinner className="size-7 text-current" />
				<p className="text-sm text-muted-foreground">Loading permissions…</p>
			</div>
		);
	}

	if (allPermissions.length === 0) {
		return (
			<div className="flex flex-col items-center justify-center rounded-xl border border-border bg-muted/40 text-center px-6 py-10">
				<div className="p-3 rounded-2xl bg-muted mb-4">
					<Settings className="size-8 text-muted-foreground" />
				</div>
				<p className="text-sm font-semibold text-foreground">
					No permissions configured
				</p>
				<p className="text-xs text-muted-foreground mt-1 max-w-xs">
					Set up permissions for this product before assigning them to an API
					key.
				</p>
				<Button
					type="button"
					variant="outline"
					onClick={() => {
						void navigate({ to: ROUTE_PATHS.PRODUCT_PERMISSIONS });
					}}
					className="mt-4"
				>
					<ExternalLink className="mr-2 size-3.5" />
					Go to Permissions
				</Button>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-4">
			{/* Selected summary */}
			{value.length > 0 && (
				<div className="rounded-xl border border-border bg-muted/50 p-3">
					<div className="flex items-center justify-between gap-2 mb-2">
						<span className="text-sm font-medium text-foreground">
							{value.length} selected
						</span>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							className="h-7 px-2 text-xs text-muted-foreground hover:text-destructive"
							onClick={() => onChange([])}
						>
							Clear all
						</Button>
					</div>
					<div className="flex flex-wrap gap-1.5">
						{value.map((name) => (
							<Badge
								key={name}
								variant="secondary"
								className="text-xs flex items-center gap-1 px-2 py-1 font-mono cursor-pointer hover:bg-destructive/10 hover:text-destructive transition-colors"
								onClick={() => toggle(name)}
							>
								{name}
								<X className="size-3" />
							</Badge>
						))}
					</div>
				</div>
			)}

			{/* Search + filter */}
			<div className="flex flex-col gap-2 sm:flex-row">
				<div className="relative flex-1">
					<Search className="absolute left-3 top-1/2 -translate-y-1/2 size-3.5 text-muted-foreground" />
					<Input
						placeholder="Search permissions…"
						value={searchTerm}
						onChange={(e) => setSearchTerm(e.target.value)}
						className="pl-9"
					/>
				</div>
				<div className="flex gap-1.5">
					<Button
						type="button"
						variant={filter === "all" ? "default" : "outline"}
						size="sm"
						className="flex-1 sm:flex-none"
						onClick={() => setFilter("all")}
					>
						All
					</Button>
					<Button
						type="button"
						variant={filter === "selected" ? "default" : "outline"}
						size="sm"
						className="flex-1 sm:flex-none"
						onClick={() => setFilter("selected")}
					>
						Selected
					</Button>
				</div>
			</div>

			{/* List */}
			<div className="rounded-xl border border-border overflow-hidden">
				<div className="flex items-center justify-between px-3 py-2 bg-muted/60 border-b border-border">
					<button
						type="button"
						className="flex items-center gap-2"
						onClick={toggleAllVisible}
					>
						<Checkbox
							checked={allVisibleSelected}
							className="size-4 pointer-events-none"
						/>
						<span className="text-xs font-medium text-muted-foreground">
							Select all visible
						</span>
					</button>
					<Badge variant="outline" className="text-xs">
						{visiblePermissions.filter((p) => value.includes(p.name)).length}/
						{visiblePermissions.length}
					</Badge>
				</div>
				<div className="max-h-[22rem] overflow-y-auto divide-y divide-border">
					{visiblePermissions.length === 0 ? (
						<div className="flex flex-col items-center justify-center py-10 text-center">
							<Search className="size-7 text-muted-foreground mb-2" />
							<p className="text-sm text-muted-foreground">
								No permissions match your search
							</p>
						</div>
					) : (
						visiblePermissions.map((permission) => {
							const isSelected = value.includes(permission.name);
							const wasOriginal = originalPermissions?.includes(
								permission.name,
							);
							return (
								<button
									key={permission.name}
									type="button"
									onClick={() => toggle(permission.name)}
									className={`flex w-full items-start gap-3 px-3 py-3 text-left transition-colors ${
										isSelected ? "bg-accent-soft" : "bg-card hover:bg-muted/60"
									}`}
								>
									<Checkbox
										checked={isSelected}
										onCheckedChange={() => toggle(permission.name)}
										className="size-4 mt-0.5 shrink-0 pointer-events-none"
									/>
									<div className="flex-1 min-w-0">
										<div className="flex flex-wrap items-center gap-2">
											<code className="text-xs bg-muted text-foreground px-2 py-0.5 rounded-md font-mono break-all">
												{permission.name}
											</code>
											{wasOriginal && (
												<Badge
													variant="secondary"
													className="text-[10px] h-5 px-1.5 text-muted-foreground"
												>
													Currently assigned
												</Badge>
											)}
										</div>
										{permission.description && (
											<p className="text-xs text-muted-foreground mt-1 leading-relaxed">
												{permission.description}
											</p>
										)}
									</div>
								</button>
							);
						})
					)}
				</div>
			</div>
		</div>
	);
}
