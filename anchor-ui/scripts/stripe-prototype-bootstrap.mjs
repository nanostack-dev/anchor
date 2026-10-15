export async function seedStripeDemoOrganizations(
	request,
	{ productPath, token, fallbackTemplateID, privateValues },
) {
	const credential = await request(`${productPath}/api-keys`, {
		method: "POST",
		token,
		body: {
			name: "Stripe demo organization bootstrap",
			permissions: ["organization:create"],
		},
	});
	if (typeof credential?.value !== "string" || !credential.value)
		throw new Error("Anchor did not return a product bootstrap API key.");
	privateValues.add(credential.value);
	for (const name of ["Maple Studio", "Northstar Labs"])
		await request(`${productPath}/organizations`, {
			method: "POST",
			key: credential.value,
			body: { name, license: { template_id: fallbackTemplateID } },
		});
}
