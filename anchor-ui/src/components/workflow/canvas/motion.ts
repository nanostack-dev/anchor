export const easeOut = [0.23, 1, 0.32, 1] as const;
/** `easeOut` for CSS transitions. */
export const EASE_OUT_CLASS = "ease-[cubic-bezier(0.23,1,0.32,1)]";

/** Stagger only the first paint of the canvas; later additions enter alone. */
export const ENTRANCE_STAGGER_SECONDS = 0.04;
export const MAX_STAGGERED_ORDER = 8;
