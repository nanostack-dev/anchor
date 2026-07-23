import { EntitlementType, type EntitlementsMap } from "@/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Plus, Trash2 } from "lucide-react";

export interface EntitlementRow {
	/** Stable client-side id so key edits keep row identity. */
	id: string;
	key: string;
	type: EntitlementType;
	value: boolean | number | "";
}

let rowIdCounter = 0;
const nextRowId = () => {
	rowIdCounter += 1;
	return `entitlement-row-${rowIdCounter}`;
};

export function entitlementsToRows(map: EntitlementsMap): EntitlementRow[] {
	return Object.entries(map).map(([key, entitlement]) => ({
		id: nextRowId(),
		key,
		type: entitlement.type,
		value:
			entitlement.type === EntitlementType.BOOLEAN
				? entitlement.value === true
				: typeof entitlement.value === "number"
					? entitlement.value
					: "",
	}));
}

export function rowsToEntitlements(rows: EntitlementRow[]): EntitlementsMap {
	const map: EntitlementsMap = {};
	for (const row of rows) {
		const key = row.key.trim();
		if (!key) continue;
		map[key] = {
			type: row.type,
			value:
				row.type === EntitlementType.BOOLEAN
					? row.value === true
					: Number(row.value),
		};
	}
	return map;
}

/**
 * Returns a validation error for the given rows, or undefined when valid.
 * Rows with an empty key or duplicate keys, and numeric rows without a
 * number, are invalid.
 */
export function validateEntitlementRows(
	rows: EntitlementRow[],
): string | undefined {
	const seen = new Set<string>();
	for (const row of rows) {
		const key = row.key.trim();
		if (!key) {
			return "Every entitlement needs a key (e.g. flows.max_per_run).";
		}
		if (seen.has(key)) {
			return `Duplicate entitlement key "${key}".`;
		}
		seen.add(key);
		if (
			row.type === EntitlementType.NUMERIC &&
			(row.value === "" || Number.isNaN(Number(row.value)))
		) {
			return `Entitlement "${key}" needs a numeric value.`;
		}
	}
	return undefined;
}

const typeItems = [
	{ value: EntitlementType.BOOLEAN, label: "Boolean" },
	{ value: EntitlementType.NUMERIC, label: "Numeric" },
];

interface EntitlementsEditorProps {
	rows: EntitlementRow[];
	onChange: (rows: EntitlementRow[]) => void;
	disabled?: boolean;
	/** Shown under the add button, e.g. what overrides do. */
	emptyHint?: string;
}

/**
 * Row-based editor for an `EntitlementsMap`: each row is a stable key, a
 * type (boolean gate or numeric limit) and a matching value control.
 * Shared between the plan dialog (plan defaults) and the license dialog
 * (per-organization overrides).
 */
export function EntitlementsEditor({
	rows,
	onChange,
	disabled = false,
	emptyHint,
}: EntitlementsEditorProps) {
	const updateRow = (id: string, patch: Partial<EntitlementRow>) => {
		onChange(rows.map((row) => (row.id === id ? { ...row, ...patch } : row)));
	};

	const addRow = () => {
		onChange([
			...rows,
			{
				id: nextRowId(),
				key: "",
				type: EntitlementType.BOOLEAN,
				value: true,
			},
		]);
	};

	const removeRow = (id: string) => {
		onChange(rows.filter((row) => row.id !== id));
	};

	return (
		<div className="flex flex-col gap-2">
			{rows.length === 0 && emptyHint && (
				<p className="text-sm text-muted-foreground">{emptyHint}</p>
			)}
			{rows.map((row) => (
				<div key={row.id} className="flex items-center gap-2">
					<Input
						value={row.key}
						onChange={(e) => updateRow(row.id, { key: e.target.value })}
						placeholder="entitlement.key"
						className="flex-1 font-mono"
						disabled={disabled}
						aria-label="Entitlement key"
					/>
					<Select
						items={typeItems}
						value={row.type}
						onValueChange={(value) => {
							const type = value as EntitlementType;
							updateRow(row.id, {
								type,
								value: type === EntitlementType.BOOLEAN ? true : "",
							});
						}}
						disabled={disabled}
					>
						<SelectTrigger className="w-28" aria-label="Entitlement type">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{typeItems.map((item) => (
								<SelectItem key={item.value} value={item.value}>
									{item.label}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<div className="flex w-32 items-center justify-start">
						{row.type === EntitlementType.BOOLEAN ? (
							<Switch
								checked={row.value === true}
								onCheckedChange={(checked) =>
									updateRow(row.id, { value: checked })
								}
								disabled={disabled}
								aria-label="Entitlement enabled"
							/>
						) : (
							<Input
								type="number"
								value={row.value === "" ? "" : String(row.value)}
								onChange={(e) =>
									updateRow(row.id, {
										value: e.target.value === "" ? "" : Number(e.target.value),
									})
								}
								placeholder="Limit"
								disabled={disabled}
								aria-label="Entitlement limit"
							/>
						)}
					</div>
					<Button
						type="button"
						variant="ghost"
						size="icon"
						onClick={() => removeRow(row.id)}
						disabled={disabled}
					>
						<span className="sr-only">Remove entitlement</span>
						<Trash2 className="size-4" />
					</Button>
				</div>
			))}
			<div>
				<Button
					type="button"
					variant="outline"
					size="sm"
					onClick={addRow}
					disabled={disabled}
				>
					<Plus />
					Add entitlement
				</Button>
			</div>
		</div>
	);
}
