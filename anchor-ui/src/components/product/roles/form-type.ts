import { z } from "zod";

export const basicInfo = z.object({
	name: z
		.string()
		.min(2, "Role name must be at least 2 characters")
		.max(100, "Role name must be less than 100 characters"),
	description: z
		.string()
		.max(500, "Description must be less than 500 characters")
		.optional(),
});

export const selectedPermissions = z.object({
	selectedPermissions: z
		.array(z.string())
		.min(1, "At least one permission is required"),
});

export const formSchema = basicInfo.extend(selectedPermissions.shape);

export type BasicInfoFormData = z.infer<typeof basicInfo>;
export type SelectedPermissionsFormData = z.infer<typeof selectedPermissions>;
export type RoleFormData = z.infer<typeof formSchema>;
