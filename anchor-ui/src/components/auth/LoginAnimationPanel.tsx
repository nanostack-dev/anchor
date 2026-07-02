import {
	LOGIN_ANIMATION_DURATION_IN_FRAMES,
	LOGIN_ANIMATION_FPS,
	LOGIN_ANIMATION_HEIGHT,
	LOGIN_ANIMATION_WIDTH,
	LoginAnimation,
} from "@/remotion/LoginAnimation";
import { Player } from "@remotion/player";

/**
 * Right-hand branding panel of the auth pages: a looping Remotion
 * animation. The wrapper background matches the composition's gradient so
 * any letterboxing from the Player's aspect-ratio fit is invisible.
 */
export function LoginAnimationPanel() {
	return (
		<div
			className="absolute inset-0 flex items-center justify-center"
			style={{
				background:
					"linear-gradient(165deg, #050d1f 0%, #0a1b3d 45%, #0d2a6e 100%)",
			}}
		>
			<Player
				component={LoginAnimation}
				durationInFrames={LOGIN_ANIMATION_DURATION_IN_FRAMES}
				fps={LOGIN_ANIMATION_FPS}
				compositionWidth={LOGIN_ANIMATION_WIDTH}
				compositionHeight={LOGIN_ANIMATION_HEIGHT}
				style={{ width: "100%", height: "100%" }}
				autoPlay
				loop
				controls={false}
				clickToPlay={false}
				acknowledgeRemotionLicense
			/>
		</div>
	);
}
