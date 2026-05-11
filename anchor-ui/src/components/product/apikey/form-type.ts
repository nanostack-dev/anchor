import {
	type ProductApiKeyCreateRequest,
	zProductApiKeyCreateRequest,
} from "@/client";
import { z } from "zod";

const createApiKeyRequestSchema = zProductApiKeyCreateRequest;

export const basicInfo = createApiKeyRequestSchema
	.pick({
		name: true,
		description: true,
		mutable: true,
	})
	.extend({
		name: z
			.string()
			.min(2, "API key name must be at least 2 characters")
			.max(100, "API key name must be less than 100 characters"),
		description: z
			.string()
			.max(500, "Description must be less than 500 characters")
			.optional(),
		mutable: z.boolean(),
	});

export const selectedPermissions = z
	.array(z.string())
	.min(1, "At least one permission is required");

export const formSchema = basicInfo.extend({
	selectedPermissions,
});

export type BasicInfoFormData = z.infer<typeof basicInfo>;
export type ApiKeyFormData = {
	name: ProductApiKeyCreateRequest["name"];
	description: string;
	mutable: NonNullable<ProductApiKeyCreateRequest["mutable"]>;
	selectedPermissions: NonNullable<ProductApiKeyCreateRequest["permissions"]>;
};

export type ApiKeyFormErrors = Record<string, string>;
