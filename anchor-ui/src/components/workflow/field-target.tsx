import {
	type ReactNode,
	createContext,
	useContext,
	useMemo,
	useState,
} from "react";

export interface FieldTarget {
	label: string;
	insert: (path: string) => void;
}

interface FieldTargets {
	active?: FieldTarget;
	focus: (target: FieldTarget) => void;
	release: (insert: FieldTarget["insert"]) => void;
}

const FieldTargetContext = createContext<FieldTargets | null>(null);

/**
 * Remembers the input a person used last in a step, so a click on a field
 * chip inserts the field there.
 */
export function FieldTargetProvider({ children }: { children: ReactNode }) {
	const [active, setActive] = useState<FieldTarget>();
	const value = useMemo<FieldTargets>(
		() => ({
			active,
			focus: (target) =>
				setActive((current) =>
					current?.label === target.label && current.insert === target.insert
						? current
						: target,
				),
			release: (insert) =>
				setActive((current) =>
					current?.insert === insert ? undefined : current,
				),
		}),
		[active],
	);
	return (
		<FieldTargetContext.Provider value={value}>
			{children}
		</FieldTargetContext.Provider>
	);
}

export const useFieldTargets = () => useContext(FieldTargetContext);
