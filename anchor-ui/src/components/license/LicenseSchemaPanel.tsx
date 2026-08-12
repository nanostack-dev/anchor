import { getLicenseSchemaOptions } from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { apiErrorHasCode, getApiErrorMessage } from "@/lib/api-error";
import { useQuery } from "@tanstack/react-query";
import { PenLine, Plus, ScrollText, TriangleAlert } from "lucide-react";
import { LicenseSchemaFormDialog } from "./LicenseSchemaFormDialog";
import { FIELD_TYPE_LABELS, summarizeRules } from "./license-field-format";

const LICENSE_SCHEMA_NOT_FOUND = "LICENSE_SCHEMA_NOT_FOUND";

interface LicenseSchemaPanelProps {
	productId: string;
}

export function LicenseSchemaPanel({ productId }: LicenseSchemaPanelProps) {
	const {
		data: schema,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId } }),
		retry: false,
	});

	if (isLoading) {
		return (
			<div className="flex flex-col gap-2">
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
			</div>
		);
	}

	const notDeclared = apiErrorHasCode(error, LICENSE_SCHEMA_NOT_FOUND);

	if (error && !notDeclared) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon" className="text-destructive">
						<TriangleAlert />
					</EmptyMedia>
					<EmptyTitle>Couldn&rsquo;t load the license schema</EmptyTitle>
					<EmptyDescription>
						{getApiErrorMessage(error) ??
							"The request did not complete. Check your connection and try again."}
					</EmptyDescription>
				</EmptyHeader>
				<Button variant="outline" size="sm" onClick={() => void refetch()}>
					Try again
				</Button>
			</Empty>
		);
	}

	if (!schema) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<ScrollText />
					</EmptyMedia>
					<EmptyTitle>No license schema declared</EmptyTitle>
					<EmptyDescription>
						This product has not declared what a license may contain. Create a
						schema to start defining limits, features, and plan values.
					</EmptyDescription>
				</EmptyHeader>
				<LicenseSchemaFormDialog
					productId={productId}
					mode="create"
					trigger={
						<Button>
							<Plus />
							Create Schema
						</Button>
					}
				/>
			</Empty>
		);
	}

	return (
		<div className="flex flex-col gap-4">
			<div className="flex items-start justify-between gap-4">
				<p className="max-w-2xl text-sm text-muted-foreground">
					{schema.description || "No description."}
				</p>
				<LicenseSchemaFormDialog
					productId={productId}
					mode="edit"
					existingSchema={schema}
					trigger={
						<Button variant="outline">
							<PenLine />
							Edit Schema
						</Button>
					}
				/>
			</div>

			<div className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
				<Table>
					<TableHeader>
						<TableRow>
							<TableHead>Name</TableHead>
							<TableHead>Type</TableHead>
							<TableHead>Description</TableHead>
							<TableHead>Rules</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{schema.fields.map((field) => (
							<TableRow key={field.id}>
								<TableCell className="font-mono text-sm">
									{field.name}
								</TableCell>
								<TableCell>
									<Badge variant="outline">
										{FIELD_TYPE_LABELS[field.type]}
									</Badge>
								</TableCell>
								<TableCell className="max-w-[280px] truncate text-sm text-muted-foreground">
									{field.description || "—"}
								</TableCell>
								<TableCell className="text-sm text-muted-foreground">
									{summarizeRules(field.type, field.rules)}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</div>
		</div>
	);
}
