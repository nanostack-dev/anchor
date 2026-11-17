import { Box } from "@nanostackorg/design-system/layout/box";

const ANCHOR_PATTERN = ["..X..", ".XXX.", "..X..", "X.X.X", ".XXX."];
const SQUARE = 10;
const PITCH = 11;
const MARK_SIZE = 54;

const ANCHOR_SQUARES = ANCHOR_PATTERN.flatMap((row, rowIndex) =>
	[...row].flatMap((cell, columnIndex) =>
		cell === "X" ? [{ x: columnIndex * PITCH, y: rowIndex * PITCH }] : [],
	),
);

const logoSizeClasses = {
	sm: "size-4",
	md: "size-5",
	lg: "size-6",
	xl: "size-24",
	hero: "size-40",
} as const;

type AnchorLogoProps = {
	size?: keyof typeof logoSizeClasses;
	label?: string;
};

export function AnchorLogo({ size = "md", label }: AnchorLogoProps) {
	return (
		<svg
			viewBox={`0 0 ${MARK_SIZE} ${MARK_SIZE}`}
			xmlns="http://www.w3.org/2000/svg"
			className={`${logoSizeClasses[size]} shrink-0`}
			role={label ? "img" : undefined}
			aria-label={label}
			aria-hidden={label ? undefined : true}
		>
			{ANCHOR_SQUARES.map((square) => (
				<rect
					key={`${square.x}-${square.y}`}
					x={square.x}
					y={square.y}
					width={SQUARE}
					height={SQUARE}
					rx={SQUARE * 0.2}
					fill="currentColor"
				/>
			))}
		</svg>
	);
}

const iconSizeClasses = {
	sm: "size-8 [&>svg]:size-5",
	md: "size-9 [&>svg]:size-5",
	lg: "size-10 [&>svg]:size-5",
} as const;

export function AnchorIcon({
	size = "md",
}: {
	size?: keyof typeof iconSizeClasses;
}) {
	return (
		<Box
			className={`grid shrink-0 place-items-center rounded-[23%] bg-primary text-primary-foreground ${iconSizeClasses[size]}`}
		>
			<AnchorLogo />
		</Box>
	);
}
