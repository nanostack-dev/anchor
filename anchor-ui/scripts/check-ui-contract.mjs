import { readFile, readdir } from "node:fs/promises";
import path from "node:path";
import ts from "typescript";

const palette =
	/(?:^|[\s:!"'`])-?(?:bg|text|border|ring|outline|fill|stroke|from|via|to|shadow|decoration|divide|accent|caret|placeholder)-(?:slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose|white|black)(?:-\d{2,3})?\b/;
const arbitraryColor = /-\[(?:#|rgb|hsl|oklch|color-mix)/;
const failures = [];
async function files(directory) {
	const entries = await readdir(directory, { withFileTypes: true });
	return (
		await Promise.all(
			entries
				.filter((entry) => entry.name !== "client")
				.map((entry) => {
					const name = path.join(directory, entry.name);
					return entry.isDirectory()
						? files(name)
						: /\.tsx?$/.test(name) && !name.endsWith(".gen.ts")
							? [name]
							: [];
				}),
		)
	).flat();
}
for (const file of await files("src")) {
	const sourceText = await readFile(file, "utf8");
	const source = ts.createSourceFile(
		file,
		sourceText,
		ts.ScriptTarget.Latest,
		true,
		file.endsWith("tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
	);
	const closed = new Set();
	const namedLists = new Set();
	const report = (node, message) =>
		failures.push(
			`${file}:${source.getLineAndCharacterOfPosition(node.getStart(source)).line + 1}: ${message}`,
		);
	for (const statement of source.statements) {
		if (
			!ts.isImportDeclaration(statement) ||
			!ts.isStringLiteral(statement.moduleSpecifier)
		)
			continue;
		const name = statement.moduleSpecifier.text;
		if (
			/(?:^@\/components\/ui(?:\/|$)|(?:\.\.\/)+ui\/|design-system\/components\/ui|design-system\/dist)/.test(
				name,
			)
		)
			report(
				statement,
				"Import public design-system entry points; local and private primitive imports are forbidden.",
			);
		if (!name.startsWith("@nanostackorg/design-system")) continue;
		const bindings = statement.importClause?.namedBindings;
		if (bindings && ts.isNamedImports(bindings))
			for (const binding of bindings.elements) {
				const imported = binding.propertyName?.text ?? binding.name.text;
				if (imported !== "Box") closed.add(binding.name.text);
				if (imported === "SelectContent") namedLists.add(binding.name.text);
			}
	}
	const visit = (node) => {
		if (
			(ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) &&
			closed.has(node.tagName.getText(source))
		) {
			if (
				namedLists.has(node.tagName.getText(source)) &&
				!node.attributes.properties.some(
					(attribute) =>
						ts.isJsxAttribute(attribute) &&
						["aria-label", "aria-labelledby"].includes(
							attribute.name.getText(source),
						),
				)
			)
				report(node, "Give the select option list an accessible name.");
			for (const attribute of node.attributes.properties)
				if (
					ts.isJsxAttribute(attribute) &&
					["className", "style"].includes(attribute.name.getText(source))
				)
					report(
						attribute,
						"Design-system parts accept semantic props, never consumer CSS.",
					);
		}
		if (
			(ts.isStringLiteralLike(node) ||
				ts.isTemplateHead(node) ||
				ts.isTemplateMiddle(node) ||
				ts.isTemplateTail(node)) &&
			(palette.test(node.text) || arbitraryColor.test(node.text))
		)
			report(
				node,
				"Use semantic design tokens instead of palette or arbitrary color utilities.",
			);
		if (
			file.startsWith("src/routes/") ||
			(file.startsWith("src/pages/") && !file.includes(".stories."))
		) {
			if (
				ts.isJsxAttribute(node) &&
				["className", "style"].includes(node.name.getText(source))
			)
				report(
					node,
					"Pages and routes compose closed UI; move product visuals into a typed product component.",
				);
		}
		ts.forEachChild(node, visit);
	};
	visit(source);
}
if (failures.length) {
	console.error(failures.join("\n"));
	process.exitCode = 1;
} else
	console.log(
		"UI contract verified: public imports, closed component styling, semantic tokens, and page composition.",
	);
