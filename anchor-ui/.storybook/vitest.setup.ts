/**
 * Runs only in the Vitest browser project. Durations go to zero so a play
 * function never waits on an animation or a transition.
 *
 * From design system 0.2.6, popups, dialogs and disclosures enter with a
 * transition from Base UI's starting style, not a keyframe: the element mounts
 * with [data-starting-style] at opacity 0 and loses the attribute a frame
 * later. No duration reaches that frame, so a toBeVisible() made in it failed
 * on a listbox that was merely one frame old. The frame is drawn at full
 * opacity here; the travel and the scale are left alone. Echopoint's
 * Storybook setup does the same.
 */
const style = document.createElement("style");
style.textContent = `
	*, *::before, *::after {
		animation-duration: 0s !important;
		animation-delay: 0s !important;
		transition-duration: 0s !important;
		transition-delay: 0s !important;
	}
	[data-starting-style] {
		opacity: 1 !important;
	}
`;
document.head.appendChild(style);
