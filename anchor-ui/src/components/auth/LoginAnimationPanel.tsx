import { motion, useReducedMotion } from "motion/react";
import type * as React from "react";
import { AnchorLogoMark } from "./AnchorLogoMark";

const FONT_STACK =
	"'Plus Jakarta Sans', 'Outfit', system-ui, -apple-system, 'Segoe UI', sans-serif";

const BACKGROUND_GRADIENT =
	"linear-gradient(165deg, #050d1f 0%, #0a1b3d 45%, #0d2a6e 100%)";

/** Deterministic pseudo-random in [0, 1) so the particle field is stable. */
function seeded(seed: number): number {
	const x = Math.sin(seed * 127.1 + 311.7) * 43758.5453;
	return x - Math.floor(x);
}

const KEYFRAMES = `
@keyframes anchor-bob {
	0%, 100% { transform: translateY(-12px); }
	50% { transform: translateY(12px); }
}
@keyframes anchor-tilt {
	0%, 100% { transform: rotate(-3deg); }
	50% { transform: rotate(3deg); }
}
@keyframes anchor-rise {
	from { transform: translateY(0); }
	to { transform: translateY(-115vh); }
}
@keyframes anchor-sway {
	0%, 100% { transform: translateX(-10px); }
	50% { transform: translateX(10px); }
}
@keyframes anchor-ripple {
	0% { transform: scale(0.3); opacity: 0; }
	15% { opacity: 0.32; }
	100% { transform: scale(1); opacity: 0; }
}
@keyframes anchor-wave-scroll {
	from { transform: translateX(0); }
	to { transform: translateX(-50%); }
}
@keyframes anchor-blob-drift {
	0%, 100% { transform: translate(-40px, -25px) scale(0.96); }
	50% { transform: translate(40px, 25px) scale(1.06); }
}
@media (prefers-reduced-motion: reduce) {
	.anchor-login-animation * { animation: none !important; }
}
`;

function AuroraBlobs() {
	const blobs = [
		{
			size: "70%",
			left: "-25%",
			top: "-15%",
			color: "rgba(0, 84, 231, 0.45)",
			duration: 20,
			delay: 0,
		},
		{
			size: "62%",
			left: "35%",
			top: "30%",
			color: "rgba(0, 180, 255, 0.22)",
			duration: 14,
			delay: -5,
		},
		{
			size: "55%",
			left: "-10%",
			top: "55%",
			color: "rgba(76, 29, 149, 0.28)",
			duration: 17,
			delay: -11,
		},
	];
	return (
		<>
			{blobs.map((blob) => (
				<div
					key={`blob-${blob.color}`}
					style={{
						position: "absolute",
						left: blob.left,
						top: blob.top,
						width: blob.size,
						aspectRatio: "1",
						borderRadius: "50%",
						background: `radial-gradient(circle, ${blob.color} 0%, transparent 70%)`,
						filter: "blur(40px)",
						animation: `anchor-blob-drift ${blob.duration}s ease-in-out ${blob.delay}s infinite`,
					}}
				/>
			))}
		</>
	);
}

function Bubbles() {
	const bubbles = Array.from({ length: 22 }, (_, i) => ({
		key: `bubble-${i}`,
		left: `${seeded(i + 80) * 100}%`,
		size: 3 + seeded(i + 120) * 8,
		opacity: 0.12 + seeded(i + 160) * 0.25,
		riseDuration: 9 + seeded(i) * 9,
		riseDelay: -seeded(i + 40) * 18,
		swayDuration: 3 + seeded(i + 7) * 2.5,
	}));
	return (
		<>
			{bubbles.map((b) => (
				<div
					key={b.key}
					style={{
						position: "absolute",
						left: b.left,
						top: "108%",
						animation: `anchor-rise ${b.riseDuration}s linear ${b.riseDelay}s infinite`,
					}}
				>
					<div
						style={{
							width: b.size,
							height: b.size,
							borderRadius: "50%",
							background: "rgba(160, 205, 255, 0.9)",
							boxShadow: "0 0 12px rgba(120, 180, 255, 0.6)",
							opacity: b.opacity,
							animation: `anchor-sway ${b.swayDuration}s ease-in-out infinite`,
						}}
					/>
				</div>
			))}
		</>
	);
}

function Ripples() {
	const RING_COUNT = 4;
	const PERIOD = 3.4;
	return (
		<>
			{Array.from({ length: RING_COUNT }, (_, i) => (
				<div
					key={`ripple-${i * 100}`}
					style={{
						position: "absolute",
						left: "50%",
						top: "37%",
						width: "min(90%, 860px)",
						aspectRatio: "1",
						marginLeft: "max(-45%, -430px)",
						translate: "0 -50%",
						borderRadius: "50%",
						border: "1.5px solid rgba(140, 190, 255, 0.9)",
						opacity: 0,
						animation: `anchor-ripple ${PERIOD}s linear ${(i * PERIOD) / RING_COUNT}s infinite`,
					}}
				/>
			))}
		</>
	);
}

function Waves() {
	// Each SVG is 2x the panel width and scrolls half its own width per
	// cycle, so the loop is seamless.
	const waves = [
		{
			duration: 10,
			height: 190,
			opacity: 0.5,
			color: "rgba(0, 84, 231, 0.35)",
		},
		{
			duration: 5,
			height: 150,
			opacity: 0.5,
			color: "rgba(0, 140, 255, 0.28)",
		},
		{
			duration: 3.4,
			height: 110,
			opacity: 0.6,
			color: "rgba(140, 200, 255, 0.18)",
		},
	];
	const w = 960;
	return (
		<>
			{waves.map((wave, i) => {
				const h = wave.height;
				const path = [
					`M0 ${h * 0.45}`,
					`C ${w * 0.25} ${h * 0.1}, ${w * 0.25} ${h * 0.8}, ${w * 0.5} ${h * 0.45}`,
					`C ${w * 0.75} ${h * 0.1}, ${w * 0.75} ${h * 0.8}, ${w} ${h * 0.45}`,
					`C ${w * 1.25} ${h * 0.1}, ${w * 1.25} ${h * 0.8}, ${w * 1.5} ${h * 0.45}`,
					`C ${w * 1.75} ${h * 0.1}, ${w * 1.75} ${h * 0.8}, ${w * 2} ${h * 0.45}`,
					`L ${w * 2} ${h} L 0 ${h} Z`,
				].join(" ");
				return (
					<svg
						key={`wave-${wave.duration}`}
						viewBox={`0 0 ${w * 2} ${h}`}
						preserveAspectRatio="none"
						style={{
							position: "absolute",
							bottom: -8 + i * -4,
							left: 0,
							width: "200%",
							height: h,
							opacity: wave.opacity,
							animation: `anchor-wave-scroll ${wave.duration}s linear infinite`,
						}}
						role="presentation"
						aria-hidden
					>
						<path d={path} fill={wave.color} />
					</svg>
				);
			})}
		</>
	);
}

function StaggeredTitle() {
	const letters = "Anchor".split("");
	return (
		<div style={{ display: "flex", justifyContent: "center" }}>
			{letters.map((letter, i) => (
				<motion.span
					key={`letter-${letter}-${i * 4}`}
					initial={{ opacity: 0, y: 46 }}
					animate={{ opacity: 1, y: 0 }}
					transition={{
						delay: 1.4 + i * 0.13,
						type: "spring",
						stiffness: 170,
						damping: 14,
						mass: 0.6,
					}}
					style={{
						display: "inline-block",
						fontFamily: FONT_STACK,
						fontSize: "clamp(56px, 9vw, 92px)",
						fontWeight: 700,
						letterSpacing: "-0.02em",
						color: "#ffffff",
						textShadow: "0 8px 40px rgba(0, 84, 231, 0.55)",
					}}
				>
					{letter}
				</motion.span>
			))}
		</div>
	);
}

function FeatureChips() {
	const chips = ["Identity", "RBAC", "Multi-tenancy"];
	return (
		<div
			style={{
				display: "flex",
				gap: 14,
				justifyContent: "center",
				marginTop: 34,
				flexWrap: "wrap",
			}}
		>
			{chips.map((chip, i) => (
				<motion.div
					key={chip}
					initial={{ opacity: 0, y: 18, scale: 0.7 }}
					animate={{ opacity: 1, y: 0, scale: 1 }}
					transition={{
						delay: 2.6 + i * 0.23,
						type: "spring",
						stiffness: 170,
						damping: 13,
						mass: 0.7,
					}}
					style={{
						fontFamily: FONT_STACK,
						fontSize: 19,
						fontWeight: 600,
						color: "rgba(220, 235, 255, 0.95)",
						padding: "10px 22px",
						borderRadius: 999,
						border: "1px solid rgba(150, 195, 255, 0.35)",
						background: "rgba(30, 70, 160, 0.25)",
						backdropFilter: "blur(6px)",
					}}
				>
					{chip}
				</motion.div>
			))}
		</div>
	);
}

/**
 * Reusable animated Anchor backdrop: aurora gradient, rising bubbles,
 * layered scrolling waves and a vignette, all Motion (MIT) + CSS
 * keyframes. Children render above the background layers but under the
 * waves/vignette, matching the login panel's depth stacking. Honors
 * prefers-reduced-motion. Fills the nearest positioned ancestor.
 */
export function AnchorAnimatedBackdrop({
	children,
}: {
	children?: React.ReactNode;
}) {
	return (
		<div
			className="anchor-login-animation"
			style={{
				position: "absolute",
				inset: 0,
				overflow: "hidden",
				background: BACKGROUND_GRADIENT,
			}}
		>
			<style>{KEYFRAMES}</style>
			<AuroraBlobs />
			<Bubbles />
			{children}
			<Waves />
			{/* Vignette for depth */}
			<div
				style={{
					position: "absolute",
					inset: 0,
					background:
						"radial-gradient(ellipse at 50% 42%, transparent 55%, rgba(2, 6, 18, 0.55) 100%)",
					pointerEvents: "none",
				}}
			/>
		</div>
	);
}

/**
 * The floating Anchor mark: spring entrance, then a continuous bob and
 * tilt. Requires the backdrop's keyframes to be present.
 */
export function FloatingAnchorMark({
	width = "clamp(150px, 24vw, 230px)",
}: {
	width?: string;
}) {
	const reducedMotion = useReducedMotion();
	return (
		<motion.div
			initial={reducedMotion ? false : { opacity: 0, y: 90, scale: 0.6 }}
			animate={{ opacity: 1, y: 0, scale: 1 }}
			transition={{
				delay: 0.27,
				type: "spring",
				stiffness: 120,
				damping: 12,
				mass: 0.9,
			}}
			style={{
				filter:
					"drop-shadow(0 0 44px rgba(0, 120, 255, 0.55)) drop-shadow(0 12px 24px rgba(0, 0, 30, 0.45))",
			}}
		>
			<div style={{ animation: "anchor-bob 5s ease-in-out infinite" }}>
				<div
					style={{
						width,
						animation: "anchor-tilt 10s ease-in-out infinite",
					}}
				>
					<AnchorLogoMark color="#ffffff" />
				</div>
			</div>
		</motion.div>
	);
}

/**
 * Right-hand branding panel of the auth pages: the animated backdrop
 * with ripple rings, the floating Anchor mark, staggered wordmark,
 * tagline and feature chips.
 */
export function LoginAnimationPanel() {
	const reducedMotion = useReducedMotion();

	return (
		<AnchorAnimatedBackdrop>
			<Ripples />

			{/* Logo */}
			<div
				style={{
					position: "absolute",
					left: 0,
					right: 0,
					top: "37%",
					display: "flex",
					justifyContent: "center",
				}}
			>
				<div style={{ translate: "0 -60%" }}>
					<FloatingAnchorMark />
				</div>
			</div>

			{/* Wordmark + tagline + chips */}
			<div
				style={{
					position: "absolute",
					left: 0,
					right: 0,
					top: "54%",
					padding: "0 24px",
				}}
			>
				<StaggeredTitle />
				<motion.div
					initial={reducedMotion ? false : { opacity: 0, y: 26 }}
					animate={{ opacity: 1, y: 0 }}
					transition={{ delay: 2.13, duration: 0.85, ease: "easeOut" }}
					style={{
						fontFamily: FONT_STACK,
						fontSize: "clamp(19px, 2.2vw, 25px)",
						fontWeight: 400,
						textAlign: "center",
						color: "rgba(195, 215, 245, 0.92)",
						marginTop: 18,
					}}
				>
					Organizations, identity &amp; access — anchored.
				</motion.div>
				<FeatureChips />
			</div>
		</AnchorAnimatedBackdrop>
	);
}
