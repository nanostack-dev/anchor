import { Composition } from "remotion";
import {
	LOGIN_ANIMATION_DURATION_IN_FRAMES,
	LOGIN_ANIMATION_FPS,
	LOGIN_ANIMATION_HEIGHT,
	LOGIN_ANIMATION_WIDTH,
	LoginAnimation,
} from "./LoginAnimation";

export function RemotionRoot() {
	return (
		<Composition
			id="LoginAnimation"
			component={LoginAnimation}
			durationInFrames={LOGIN_ANIMATION_DURATION_IN_FRAMES}
			fps={LOGIN_ANIMATION_FPS}
			width={LOGIN_ANIMATION_WIDTH}
			height={LOGIN_ANIMATION_HEIGHT}
		/>
	);
}
