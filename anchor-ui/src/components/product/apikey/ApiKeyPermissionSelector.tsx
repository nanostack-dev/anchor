import { SortDirection } from "@/client";
import { searchProductPermissionsOptions } from "@/client/@tanstack/react-query.gen";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	InputGroup,
	InputGroupAddon,
	InputGroupInput,
} from "@nanostackorg/design-system/components/input-group";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Box } from "@nanostackorg/design-system/layout/box";
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
			<Box className="flex flex-col items-center justify-center h-40 rounded-xl border border-border bg-muted/40 gap-3">
				<Spinner size="lg" />
				<Box as="p" className="text-sm text-muted-foreground">
					Loading permissions…
				</Box>
			</Box>
		);
	}

	if (allPermissions.length === 0) {
		return (
			<Box className="flex flex-col items-center justify-center rounded-xl border border-border bg-muted/40 text-center px-6 py-10">
				<Box className="p-3 rounded-2xl bg-muted mb-4">
					<Settings className="size-8 text-muted-foreground" />
				</Box>
				<Box as="p" className="text-sm font-semibold text-foreground">
					No permissions configured
				</Box>
				<Box as="p" className="text-xs text-muted-foreground mt-1 max-w-xs">
					Set up permissions for this product before assigning them to an API
					key.
				</Box>
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
			</Box>
		);
	}

	return (
		<Box className="flex flex-col gap-4">
			{/* Selected summary */}
			{value.length > 0 && (
				<Box className="rounded-xl border border-border bg-muted/50 p-3">
					<Box className="flex items-center justify-between gap-2 mb-2">
						<Box as="span" className="text-sm font-medium text-foreground">
							{value.length} selected
						</Box>
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onClick={() => onChange([])}
						>
							Clear all
						</Button>
					</Box>
					<Box className="flex flex-wrap gap-1.5">
						{value.map((name) => (
							<Badge
								tone="neutral"
								key={name}
								variant="soft"
								onClick={() => toggle(name)}
							>
								{name}
								<X className="size-3" />
							</Badge>
						))}
					</Box>
				</Box>
			)}

			{/* Search + filter */}
			<Box className="flex flex-col gap-2 sm:flex-row">
				<InputGroup>
					<InputGroupAddon>
						<Search aria-hidden />
					</InputGroupAddon>
					<InputGroupInput
						placeholder="Search permissions…"
						value={searchTerm}
						onChange={(e) => setSearchTerm(e.target.value)}
					/>
				</InputGroup>
				<Box className="flex gap-1.5">
					<Button
						tone="brand"
						type="button"
						variant={filter === "all" ? "solid" : "outline"}
						size="sm"
						onClick={() => setFilter("all")}
					>
						All
					</Button>
					<Button
						tone="brand"
						type="button"
						variant={filter === "selected" ? "solid" : "outline"}
						size="sm"
						onClick={() => setFilter("selected")}
					>
						Selected
					</Button>
				</Box>
			</Box>

			{/* List */}
			<Box className="rounded-xl border border-border overflow-hidden">
				<Box className="flex items-center justify-between px-3 py-2 bg-muted/60 border-b border-border">
					<button
						type="button"
						className="flex items-center gap-2"
						onClick={toggleAllVisible}
					>
						<Checkbox checked={allVisibleSelected} />
						<Box
							as="span"
							className="text-xs font-medium text-muted-foreground"
						>
							Select all visible
						</Box>
					</button>
					<Badge variant="outline">
						{visiblePermissions.filter((p) => value.includes(p.name)).length}/
						{visiblePermissions.length}
					</Badge>
				</Box>
				<Box className="max-h-[22rem] overflow-y-auto divide-y divide-border">
					{visiblePermissions.length === 0 ? (
						<Box className="flex flex-col items-center justify-center py-10 text-center">
							<Search className="size-7 text-muted-foreground mb-2" />
							<Box as="p" className="text-sm text-muted-foreground">
								No permissions match your search
							</Box>
						</Box>
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
										isSelected ? "bg-accent" : "bg-card hover:bg-muted/60"
									}`}
								>
									<Checkbox
										checked={isSelected}
										onCheckedChange={() => toggle(permission.name)}
									/>
									<Box className="flex-1 min-w-0">
										<Box className="flex flex-wrap items-center gap-2">
											<code className="text-xs bg-muted text-foreground px-2 py-0.5 rounded-md font-mono break-all">
												{permission.name}
											</code>
											{wasOriginal && (
												<Badge tone="neutral" variant="soft">
													Currently assigned
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
								</button>
							);
						})
					)}
				</Box>
			</Box>
		</Box>
	);
}
