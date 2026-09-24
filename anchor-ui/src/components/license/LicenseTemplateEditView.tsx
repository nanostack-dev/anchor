import {
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
} from "@/client";
import { Page } from "@/components/common/Page";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button, buttonVariants } from "@/components/ui/button";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Link } from "@tanstack/react-router";
import dayjs from "dayjs";
import { ArrowLeft, PenLine } from "lucide-react";
import { LicenseTemplateForm } from "./LicenseTemplateForm";
import { LicenseValueFields } from "./LicenseValueFields";

interface LicenseTemplateEditViewProps {
	productId: string;
	schema: LicenseSchemaResponse;
	template?: LicenseTemplateResponse;
	editing: boolean;
	onEdit: () => void;
	onCancel: () => void;
	onSaved: (template: LicenseTemplateResponse) => void;
}

export function LicenseTemplateBackLink() {
	return (
		<Link
			to={ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES}
			className={buttonVariants({ variant: "outline" })}
		>
			<ArrowLeft data-icon="inline-start" />
			All templates
		</Link>
	);
}

export function LicenseTemplateEditView({
	productId,
	schema,
	template,
	editing,
	onEdit,
	onCancel,
	onSaved,
}: LicenseTemplateEditViewProps) {
	const archived = template?.status === LicenseTemplateStatus.ARCHIVED;
	const showForm = !template || (editing && !archived);
	return (
		<Page
			variant="default"
			breadCrumbs={false}
			title={
				template
					? showForm
						? `Edit ${template.name}`
						: template.name
					: "Create License Template"
			}
			description={
				showForm
					? "A named set of values for this product’s organization licenses."
					: template?.description || "No description."
			}
			actions={
				<>
					<LicenseTemplateBackLink />
					{!showForm && !archived && (
						<Button onClick={onEdit}>
							<PenLine data-icon="inline-start" />
							Edit template
						</Button>
					)}
				</>
			}
		>
			<div className="flex flex-col gap-6">
				{template && (
					<div className="flex flex-wrap items-center gap-3">
						<StatusBadge tone={archived ? "neutral" : "success"}>
							{template.status}
						</StatusBadge>
						<p className="text-xs text-muted-foreground">
							Created {dayjs(template.created_at).format("D MMMM YYYY H:mm")} ·
							Updated {dayjs(template.updated_at).format("D MMMM YYYY H:mm")}
						</p>
					</div>
				)}
				{archived && (
					<p className="text-sm text-muted-foreground">
						This template is archived and can no longer be edited.
					</p>
				)}
				<section
					aria-label="Template details"
					className="rounded-xl border border-border bg-card p-4 shadow-sm sm:p-6"
				>
					{showForm ? (
						<LicenseTemplateForm
							productId={productId}
							schema={schema}
							template={template}
							onCancel={onCancel}
							onSaved={onSaved}
						/>
					) : (
						<div
							aria-labelledby="template-values-heading"
							className="flex flex-col gap-3"
						>
							<h2
								id="template-values-heading"
								className="text-base font-semibold"
							>
								License values
							</h2>
							<LicenseValueFields
								fields={schema.fields}
								values={template.values}
							/>
						</div>
					)}
				</section>
			</div>
		</Page>
	);
}
