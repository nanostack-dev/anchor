import type { ProductResourcePermissionResponse } from "@/client";
import { createProductResourcePermissionMutation } from "@/client/@tanstack/react-query.gen";
import { zCreateProductResourcePermissionRequest } from "@/client/zod.gen";
import { ResourceTypeSelect } from "@/components/product/resource-types/ResourceTypeSelect";
import { getApiErrorMessage } from "@/lib/api-error";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Field,
	FieldDescription,
	FieldError,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

export interface CreateProductResourcePermissionFormProps {
	productId: string;
	initialResourceType?: string;
	onCreated: (permission: ProductResourcePermissionResponse) => void;
	onCancel: () => void;
}

const prefix = (slug: string) => (slug ? `${slug}:` : "");

export function CreateProductResourcePermissionForm({
	productId,
	initialResourceType = "",
	onCreated,
	onCancel,
}: CreateProductResourcePermissionFormProps) {
	const queryClient = useQueryClient();
	const [resourceType, setResourceType] = useState(initialResourceType);
	const [name, setName] = useState(prefix(initialResourceType));
	const [description, setDescription] = useState("");
	const [submitted, setSubmitted] = useState(false);
	const create = useMutation({
		...createProductResourcePermissionMutation(),
		onSuccess: (permission) => {
			void queryClient.invalidateQueries({
				predicate: (cached) => {
					const [key] = cached.queryKey as [{ _id?: string }];
					return key?._id === "searchProductResourcePermissions";
				},
			});
			toast.add({
				type: "success",
				title: "Permission created",
				description: permission.name,
			});
			onCreated(permission);
		},
	});

	const parsed = zCreateProductResourcePermissionRequest.safeParse({
		name: name.trim(),
		description: description.trim() || undefined,
		resource_type: resourceType || undefined,
	});
	const issues = parsed.success ? [] : parsed.error.issues;
	const problem = (field: string) =>
		issues.some((issue) => issue.path[0] === field);
	const nameError = problem("name")
		? name.trim().length < 2
			? "Give the permission a name of at least 2 characters."
			: "Use at most 200 characters."
		: undefined;
	const typeError = problem("resource_type")
		? "Choose the resource type this permission applies to."
		: undefined;
	const descriptionError = problem("description")
		? "Use at most 500 characters."
		: undefined;

	return (
		<Stack
			as="form"
			space="lg"
			onSubmit={(event) => {
				event.preventDefault();
				setSubmitted(true);
				if (!parsed.success) return;
				create.mutate({ path: { product_id: productId }, body: parsed.data });
			}}
		>
			{create.isError && (
				<Alert tone="critical" icon={WarningCircleIcon}>
					<AlertTitle>Anchor didn't create the permission</AlertTitle>
					<AlertDescription>
						{getApiErrorMessage(create.error) ?? "Try again in a moment."}
					</AlertDescription>
				</Alert>
			)}
			<FieldGroup>
				<Field invalid={submitted && Boolean(typeError)}>
					<FieldLabel htmlFor="permission-resource-type">
						Resource type
					</FieldLabel>
					<ResourceTypeSelect
						id="permission-resource-type"
						productId={productId}
						value={resourceType}
						invalid={submitted && Boolean(typeError)}
						onChange={(slug) => {
							if (!name.trim() || name === prefix(resourceType))
								setName(prefix(slug));
							setResourceType(slug);
						}}
					/>
					{submitted && typeError ? (
						<FieldError>{typeError}</FieldError>
					) : (
						<FieldDescription>
							Roles scoped to this type, or to a type above it, can bundle the
							permission. It never changes after you create it.
						</FieldDescription>
					)}
				</Field>
				<Field invalid={submitted && Boolean(nameError)}>
					<FieldLabel htmlFor="permission-name">Name</FieldLabel>
					<Input
						id="permission-name"
						font="mono"
						value={name}
						maxLength={220}
						placeholder="document:view"
						aria-invalid={(submitted && Boolean(nameError)) || undefined}
						onChange={(event) => setName(event.target.value)}
					/>
					{submitted && nameError ? (
						<FieldError>{nameError}</FieldError>
					) : (
						<FieldDescription>
							Unique in the product. A type:action form reads well, but any name
							works.
						</FieldDescription>
					)}
				</Field>
				<Field invalid={Boolean(descriptionError)}>
					<FieldLabel htmlFor="permission-description">Description</FieldLabel>
					<Textarea
						id="permission-description"
						value={description}
						rows={3}
						onChange={(event) => setDescription(event.target.value)}
					/>
					{descriptionError && <FieldError>{descriptionError}</FieldError>}
				</Field>
			</FieldGroup>
			<Inline space="sm">
				<Button
					type="submit"
					variant="solid"
					tone="brand"
					loading={create.isPending}
					disabled={!resourceType || !name.trim()}
				>
					Create permission
				</Button>
				<Button type="button" variant="outline" onClick={onCancel}>
					Cancel
				</Button>
			</Inline>
		</Stack>
	);
}
