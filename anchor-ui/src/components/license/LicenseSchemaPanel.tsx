import { getLicenseSchema } from "@/client";
import { getLicenseSchemaQueryKey } from "@/client/@tanstack/react-query.gen";
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError, unwrapQuery } from "@/lib/http-query-error";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@nanostackorg/design-system/components/table";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	PencilLineIcon as PenLine,
	PlusIcon as Plus,
	ScrollIcon as ScrollText,
	WarningIcon as TriangleAlert,
} from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import { LicenseSchemaFormDialog } from "./LicenseSchemaFormDialog";
import { FIELD_TYPE_LABELS, summarizeRules } from "./license-field-format";

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
		queryKey: getLicenseSchemaQueryKey({ path: { product_id: productId } }),
		// A raw (non-throwing) SDK call rather than getLicenseSchemaOptions():
		// that helper's throwOnError mode discards the HTTP status once the
		// body doesn't parse as an ApiErrorResponse, which is exactly what
		// happens on some deployments' 404s (an empty or plain-text body from
		// an earlier middleware layer, not the handler's own structured
		// error). Matching on status here is reliable either way.
		queryFn: () =>
			unwrapQuery(getLicenseSchema({ path: { product_id: productId } })),
		retry: false,
	});

	if (isLoading) {
		return (
			<Stack space="sm">
				<Skeleton height="lg" />
				<Skeleton height="lg" />
				<Skeleton height="lg" />
			</Stack>
		);
	}

	// This route documents exactly one 404 case — no schema declared yet —
	// so any 404 here is treated as that, whether or not its body happened to
	// parse into the specific LICENSE_SCHEMA_NOT_FOUND shape.
	const notDeclared = isHttpQueryError(error) && error.status === 404;

	if (error && !notDeclared) {
		const detail = isHttpQueryError(error)
			? (getErrorDetail(error.body) ??
				`The server responded with HTTP ${error.status}.`)
			: (getErrorDetail(error) ??
				"No response was received at all — the request never reached a server, or a browser-level failure (offline, DNS, CORS) stopped it before one could answer.");

		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia icon={TriangleAlert} />
					<EmptyTitle>Couldn&rsquo;t load the license schema</EmptyTitle>
					<EmptyDescription>{detail}</EmptyDescription>
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
					<EmptyMedia icon={ScrollText} />
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
						<Button variant="solid" tone="brand" icon={Plus}>
							Create Schema
						</Button>
					}
				/>
			</Empty>
		);
	}

	return (
		<Stack space="lg">
			<Spread space="lg" alignY="start">
				<Box as="p" className="max-w-2xl text-sm text-muted-foreground">
					{schema.description || "No description."}
				</Box>
				<LicenseSchemaFormDialog
					productId={productId}
					mode="edit"
					existingSchema={schema}
					trigger={
						<Button icon={PenLine} variant="outline">
							Edit Schema
						</Button>
					}
				/>
			</Spread>

			<Box className="overflow-hidden rounded-xl border border-border bg-card shadow-sm">
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
								<TableCell font="mono">{field.name}</TableCell>
								<TableCell>
									<Badge variant="outline">
										{FIELD_TYPE_LABELS[field.type]}
									</Badge>
								</TableCell>
								<TableCell tone="muted">{field.description || "—"}</TableCell>
								<TableCell tone="muted">
									{summarizeRules(field.type, field.rules)}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</Box>
		</Stack>
	);
}
