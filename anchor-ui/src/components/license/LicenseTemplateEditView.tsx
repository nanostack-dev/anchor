import {
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
} from "@/client";
import { Page } from "@/components/common/Page";
import { StatusBadge } from "@/components/common/StatusBadge";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Button,
	ButtonLink,
} from "@nanostackorg/design-system/components/button";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ArrowLeftIcon as ArrowLeft,
	PencilLineIcon as PenLine,
} from "@phosphor-icons/react";

import dayjs from "dayjs";
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
		<ButtonLink
			href={ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES}
			variant="outline"
			icon={ArrowLeft}
		>
			All templates
		</ButtonLink>
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
		>
			<Box className="flex max-w-3xl flex-col gap-6">
				<Inline space="md">
					<LicenseTemplateBackLink />
					{!showForm && !archived && (
						<Button
							variant="solid"
							tone="brand"
							icon={PenLine}
							onClick={onEdit}
						>
							Edit template
						</Button>
					)}
				</Inline>
				{template && (
					<Inline space="md">
						<StatusBadge tone={archived ? "neutral" : "success"}>
							{template.status}
						</StatusBadge>
						<Text size="xs" tone="muted">
							Created {dayjs(template.created_at).format("D MMMM YYYY H:mm")} ·
							Updated {dayjs(template.updated_at).format("D MMMM YYYY H:mm")}
						</Text>
					</Inline>
				)}
				{archived && (
					<Text tone="muted">
						This template is archived and can no longer be edited.
					</Text>
				)}
				<Stack as="section" space="lg" aria-label="Template details">
					{showForm ? (
						<LicenseTemplateForm
							productId={productId}
							schema={schema}
							template={template}
							onCancel={onCancel}
							onSaved={onSaved}
						/>
					) : (
						<Stack aria-labelledby="template-values-heading" space="md">
							<Heading id="template-values-heading" level={2}>
								License values
							</Heading>
							<LicenseValueFields
								fields={schema.fields}
								values={template.values}
							/>
						</Stack>
					)}
				</Stack>
			</Box>
		</Page>
	);
}
