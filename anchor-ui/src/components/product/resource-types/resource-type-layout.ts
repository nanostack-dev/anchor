import dagre from "@dagrejs/dagre";
import type { TreeType } from "./resource-type-tree";

export const NODE_WIDTH = 208;
export const NODE_HEIGHT = 60;
const RANK_GAP = 72;
const NODE_GAP = 28;

export interface Point {
	x: number;
	y: number;
}

export type Layout = Map<string, Point>;

/**
 * Anchor returns a published tree in creation order, and types created by one
 * publish by slug. Laying draft types out in that same order keeps every node
 * in its place when the draft is published.
 */
export function layoutOrder(types: TreeType[]) {
	return [
		...types.filter((type) => type.published),
		...types
			.filter((type) => !type.published)
			.sort((left, right) => left.slug.localeCompare(right.slug)),
	];
}

export function layoutTree(unordered: TreeType[]): Layout {
	const types = layoutOrder(unordered);
	const graph = new dagre.graphlib.Graph();
	graph.setGraph({ rankdir: "TB", ranksep: RANK_GAP, nodesep: NODE_GAP });
	graph.setDefaultEdgeLabel(() => ({}));
	const keys = new Set(types.map((type) => type.key));
	for (const type of types)
		graph.setNode(type.key, { width: NODE_WIDTH, height: NODE_HEIGHT });
	for (const type of types)
		for (const parent of type.parents)
			if (parent !== type.key && keys.has(parent))
				graph.setEdge(parent, type.key);
	dagre.layout(graph);
	const layout: Layout = new Map();
	for (const type of types) {
		const { x, y } = graph.node(type.key);
		layout.set(type.key, {
			x: Math.round(x - NODE_WIDTH / 2),
			y: Math.round(y - NODE_HEIGHT / 2),
		});
	}
	return layout;
}

export function sameLayout(left: Layout, right: Layout) {
	if (left.size !== right.size) return false;
	for (const [key, point] of left) {
		const other = right.get(key);
		if (!other || other.x !== point.x || other.y !== point.y) return false;
	}
	return true;
}

export function interpolateLayout(
	from: Layout,
	to: Layout,
	progress: number,
): Layout {
	const frame: Layout = new Map();
	for (const [key, target] of to) {
		const start = from.get(key) ?? target;
		frame.set(key, {
			x: start.x + (target.x - start.x) * progress,
			y: start.y + (target.y - start.y) * progress,
		});
	}
	return frame;
}

export type Direction = "up" | "down" | "left" | "right";

export function neighbour(
	types: TreeType[],
	layout: Layout,
	key: string,
	direction: Direction,
): string | undefined {
	const type = types.find((candidate) => candidate.key === key);
	const here = layout.get(key);
	if (!type || !here) return undefined;
	const byX = (left: string, right: string) =>
		(layout.get(left)?.x ?? 0) - (layout.get(right)?.x ?? 0);
	const closest = (keys: string[]) =>
		[...keys].sort(
			(left, right) =>
				Math.abs((layout.get(left)?.x ?? 0) - here.x) -
				Math.abs((layout.get(right)?.x ?? 0) - here.x),
		)[0];
	if (direction === "up")
		return closest(type.parents.filter((parent) => parent !== key));
	if (direction === "down")
		return types
			.filter(
				(candidate) => candidate.key !== key && candidate.parents.includes(key),
			)
			.map((child) => child.key)
			.sort(byX)[0];
	const row = types
		.map((candidate) => candidate.key)
		.filter((candidate) => layout.get(candidate)?.y === here.y)
		.sort(byX);
	const index = row.indexOf(key);
	return row[direction === "left" ? index - 1 : index + 1];
}
