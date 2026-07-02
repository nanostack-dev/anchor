import {
	AbsoluteFill,
	interpolate,
	spring,
	useCurrentFrame,
	useVideoConfig,
} from "remotion";
import { AnchorLogoMark } from "./AnchorLogoMark";

export const LOGIN_ANIMATION_FPS = 30;
export const LOGIN_ANIMATION_DURATION_IN_FRAMES = 300;
export const LOGIN_ANIMATION_WIDTH = 960;
export const LOGIN_ANIMATION_HEIGHT = 1080;

const BRAND_BLUE = "#0054e7";
const DEEP_NAVY = "#050d1f";

const FONT_STACK =
	"'Plus Jakarta Sans', 'Outfit', system-ui, -apple-system, 'Segoe UI', sans-serif";

/** Deterministic pseudo-random in [0, 1) so renders are reproducible. */
function seeded(seed: number): number {
	const x = Math.sin(seed * 127.1 + 311.7) * 43758.5453;
	return x - Math.floor(x);
}

/** Periodic sine wave in [-1, 1] whose period divides the loop length. */
function loopSine(frame: number, period: number, phase = 0): number {
	return Math.sin((frame / period) * Math.PI * 2 + phase);
}

function AuroraBlobs({ frame }: { frame: number }) {
	const blobs = [
		{
			size: 720,
			x: 120 + loopSine(frame, 300) * 70,
			y: 140 + loopSine(frame, 150, 1.3) * 50,
			color: "rgba(0, 84, 231, 0.45)",
		},
		{
			size: 640,
			x: 520 + loopSine(frame, 300, 2.4) * 90,
			y: 520 + loopSine(frame, 150, 0.6) * 60,
			color: "rgba(0, 180, 255, 0.22)",
		},
		{
			size: 560,
			x: 240 + loopSine(frame, 300, 4.1) * 60,
			y: 760 + loopSine(frame, 150, 3.1) * 40,
			color: "rgba(76, 29, 149, 0.28)",
		},
	];

	return (
		<AbsoluteFill>
			{blobs.map((blob, i) => (
				<div
					key={`blob-${blob.color}`}
					style={{
						position: "absolute",
						left: blob.x - blob.size / 2,
						top: blob.y - blob.size / 2,
						width: blob.size,
						height: blob.size,
						borderRadius: "50%",
						background: `radial-gradient(circle, ${blob.color} 0%, transparent 70%)`,
						filter: "blur(40px)",
						transform: `scale(${1 + loopSine(frame, 150, i * 2) * 0.06})`,
					}}
				/>
			))}
		</AbsoluteFill>
	);
}

function Bubbles({ frame }: { frame: number }) {
	const bubbles = Array.from({ length: 22 }, (_, i) => {
		const laps = 1 + Math.floor(seeded(i) * 2); // 1 or 2 full climbs per loop
		const progress =
			((frame * laps) / LOGIN_ANIMATION_DURATION_IN_FRAMES + seeded(i + 40)) %
			1;
		const x =
			seeded(i + 80) * LOGIN_ANIMATION_WIDTH +
			loopSine(frame, 100, i) * (8 + seeded(i + 7) * 14);
		const y = LOGIN_ANIMATION_HEIGHT * (1.05 - progress * 1.15);
		const size = 3 + seeded(i + 120) * 8;
		const opacity =
			(0.12 + seeded(i + 160) * 0.25) *
			// fade near top and bottom edges
			Math.min(1, progress * 6, (1 - progress) * 6);
		return { x, y, size, opacity, key: `bubble-${i}` };
	});

	return (
		<AbsoluteFill>
			{bubbles.map((b) => (
				<div
					key={b.key}
					style={{
						position: "absolute",
						left: b.x,
						top: b.y,
						width: b.size,
						height: b.size,
						borderRadius: "50%",
						background: "rgba(160, 205, 255, 0.9)",
						boxShadow: "0 0 12px rgba(120, 180, 255, 0.6)",
						opacity: b.opacity,
					}}
				/>
			))}
		</AbsoluteFill>
	);
}

function Ripples({
	frame,
	centerY,
}: {
	frame: number;
	centerY: number;
}) {
	const RING_COUNT = 4;
	const PERIOD = 100;
	return (
		<AbsoluteFill>
			{Array.from({ length: RING_COUNT }, (_, i) => {
				const progress =
					((frame + (i * PERIOD) / RING_COUNT) % PERIOD) / PERIOD;
				const size = interpolate(progress, [0, 1], [260, 860]);
				const opacity = interpolate(progress, [0, 0.15, 1], [0, 0.32, 0]);
				return (
					<div
						key={`ripple-${i * PERIOD}`}
						style={{
							position: "absolute",
							left: LOGIN_ANIMATION_WIDTH / 2 - size / 2,
							top: centerY - size / 2,
							width: size,
							height: size,
							borderRadius: "50%",
							border: "1.5px solid rgba(140, 190, 255, 0.9)",
							opacity,
						}}
					/>
				);
			})}
		</AbsoluteFill>
	);
}

function Waves({ frame }: { frame: number }) {
	// Each wave path is 2x the canvas width and scrolls one full canvas
	// width over a period that divides the loop, so the loop is seamless.
	const waves = [
		{ period: 300, height: 190, opacity: 0.5, color: "rgba(0, 84, 231, 0.35)" },
		{
			period: 150,
			height: 150,
			opacity: 0.5,
			color: "rgba(0, 140, 255, 0.28)",
		},
		{
			period: 100,
			height: 110,
			opacity: 0.6,
			color: "rgba(140, 200, 255, 0.18)",
		},
	];

	return (
		<AbsoluteFill style={{ justifyContent: "flex-end" }}>
			{waves.map((wave, i) => {
				const shift =
					((frame % wave.period) / wave.period) * LOGIN_ANIMATION_WIDTH;
				const h = wave.height;
				const w = LOGIN_ANIMATION_WIDTH;
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
						key={`wave-${wave.period}`}
						width={w * 2}
						height={h}
						viewBox={`0 0 ${w * 2} ${h}`}
						style={{
							position: "absolute",
							bottom: -8 + i * -4,
							left: -shift,
							opacity: wave.opacity,
						}}
						role="presentation"
						aria-hidden
					>
						<path d={path} fill={wave.color} />
					</svg>
				);
			})}
		</AbsoluteFill>
	);
}

function StaggeredTitle({ frame, fps }: { frame: number; fps: number }) {
	const letters = "Anchor".split("");
	return (
		<div style={{ display: "flex", justifyContent: "center" }}>
			{letters.map((letter, i) => {
				const drive = spring({
					frame: frame - 42 - i * 4,
					fps,
					config: { damping: 14, mass: 0.6 },
				});
				return (
					<span
						key={`letter-${letter}-${i * 4}`}
						style={{
							display: "inline-block",
							fontFamily: FONT_STACK,
							fontSize: 92,
							fontWeight: 700,
							letterSpacing: "-0.02em",
							color: "#ffffff",
							opacity: drive,
							transform: `translateY(${(1 - drive) * 46}px)`,
							textShadow: "0 8px 40px rgba(0, 84, 231, 0.55)",
						}}
					>
						{letter}
					</span>
				);
			})}
		</div>
	);
}

function FeatureChips({ frame, fps }: { frame: number; fps: number }) {
	const chips = ["Identity", "RBAC", "Multi-tenancy"];
	return (
		<div
			style={{
				display: "flex",
				gap: 14,
				justifyContent: "center",
				marginTop: 34,
			}}
		>
			{chips.map((chip, i) => {
				const drive = spring({
					frame: frame - 78 - i * 7,
					fps,
					config: { damping: 13, mass: 0.7 },
				});
				return (
					<div
						key={chip}
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
							opacity: drive,
							transform: `scale(${0.7 + drive * 0.3}) translateY(${(1 - drive) * 18}px)`,
						}}
					>
						{chip}
					</div>
				);
			})}
		</div>
	);
}

/**
 * Brand animation for the Anchor login page.
 *
 * 10s seamless-ish loop: aurora background, rising bubbles, ripple rings,
 * the Anchor mark floating on a spring entrance, staggered wordmark,
 * tagline, feature chips and layered scrolling waves.
 */
export function LoginAnimation() {
	const frame = useCurrentFrame();
	const { fps } = useVideoConfig();

	const LOGO_CENTER_Y = 400;

	const logoIn = spring({
		frame: frame - 8,
		fps,
		config: { damping: 12, mass: 0.9 },
	});
	const bobY = loopSine(frame, 150) * 12;
	const tilt = loopSine(frame, 300, 1) * 3;

	const taglineOpacity = interpolate(frame, [64, 90], [0, 1], {
		extrapolateLeft: "clamp",
		extrapolateRight: "clamp",
	});
	const taglineY = interpolate(frame, [64, 90], [26, 0], {
		extrapolateLeft: "clamp",
		extrapolateRight: "clamp",
	});

	return (
		<AbsoluteFill
			style={{
				background: `linear-gradient(165deg, ${DEEP_NAVY} 0%, #0a1b3d 45%, #0d2a6e 100%)`,
				overflow: "hidden",
			}}
		>
			<AuroraBlobs frame={frame} />
			<Bubbles frame={frame} />
			<Ripples frame={frame} centerY={LOGO_CENTER_Y} />

			{/* Logo */}
			<div
				style={{
					position: "absolute",
					left: 0,
					right: 0,
					top: LOGO_CENTER_Y - 130,
					display: "flex",
					justifyContent: "center",
				}}
			>
				<div
					style={{
						width: 230,
						height: 262,
						opacity: logoIn,
						transform: `translateY(${(1 - logoIn) * 90 + bobY}px) scale(${
							0.6 + logoIn * 0.4
						}) rotate(${tilt}deg)`,
						filter:
							"drop-shadow(0 0 44px rgba(0, 120, 255, 0.55)) drop-shadow(0 12px 24px rgba(0, 0, 30, 0.45))",
					}}
				>
					<AnchorLogoMark color="#ffffff" />
				</div>
			</div>

			{/* Wordmark + tagline + chips */}
			<div
				style={{
					position: "absolute",
					left: 0,
					right: 0,
					top: LOGO_CENTER_Y + 190,
				}}
			>
				<StaggeredTitle frame={frame} fps={fps} />
				<div
					style={{
						fontFamily: FONT_STACK,
						fontSize: 25,
						fontWeight: 400,
						textAlign: "center",
						color: "rgba(195, 215, 245, 0.92)",
						marginTop: 18,
						opacity: taglineOpacity,
						transform: `translateY(${taglineY}px)`,
					}}
				>
					Organizations, identity &amp; access — anchored.
				</div>
				<FeatureChips frame={frame} fps={fps} />
			</div>

			<Waves frame={frame} />

			{/* Vignette for depth */}
			<AbsoluteFill
				style={{
					background:
						"radial-gradient(ellipse at 50% 42%, transparent 55%, rgba(2, 6, 18, 0.55) 100%)",
					pointerEvents: "none",
				}}
			/>
		</AbsoluteFill>
	);
}

export const loginAnimationBrandColor: string = BRAND_BLUE;
