import type { EditorProps } from "@monaco-editor/react";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Suspense, lazy } from "react";

type LocalEditorProps = Pick<
	EditorProps,
	"value" | "onChange" | "height" | "options"
> & { language?: "html"; theme?: "vs-light" };

const Editor = lazy(async () => {
	const [reactEditor, monaco, editorWorker, htmlWorker] = await Promise.all([
		import("@monaco-editor/react"),
		import("monaco-editor/esm/vs/editor/editor.api.js"),
		import("monaco-editor/esm/vs/editor/editor.worker.js?worker"),
		import("monaco-editor/esm/vs/language/html/html.worker.js?worker"),
	]);
	await Promise.all([
		import("monaco-editor/esm/vs/basic-languages/html/html.contribution.js"),
		import("monaco-editor/esm/vs/language/html/monaco.contribution.js"),
	]);
	self.MonacoEnvironment = {
		getWorker(_moduleId, label) {
			return label === "html" || label === "handlebars" || label === "razor"
				? new htmlWorker.default()
				: new editorWorker.default();
		},
	};
	reactEditor.loader.config({ monaco });
	const context = document.createElement("canvas").getContext("2d");
	if (!context) throw new Error("Cannot resolve email editor text color.");
	context.fillStyle = getComputedStyle(document.documentElement)
		.getPropertyValue("--foreground")
		.trim();
	context.fillRect(0, 0, 1, 1);
	const foreground = Array.from(context.getImageData(0, 0, 1, 1).data)
		.slice(0, 3)
		.map((channel) => channel.toString(16).padStart(2, "0"))
		.join("");
	monaco.editor.defineTheme("anchor-email-light", {
		base: "vs",
		inherit: true,
		rules: ["metatag.html", "metatag.content.html", "attribute.name.html"].map(
			(token) => ({ token, foreground }),
		),
		colors: {},
	});
	return { default: reactEditor.default };
});

export default function LocalEditor(props: LocalEditorProps) {
	const loading = <Spinner aria-label="Loading HTML editor" />;
	return (
		<Suspense fallback={loading}>
			<Editor
				{...props}
				language="html"
				theme="anchor-email-light"
				loading={loading}
			/>
		</Suspense>
	);
}
