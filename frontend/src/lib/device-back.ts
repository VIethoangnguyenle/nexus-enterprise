/**
 * Whether the last step of the history was the device's own back (the system swipe or button) rather
 * than a step the app took. The device has already drawn that step; an app animation over it (a layer
 * fading in from nothing, one fading out over the page) is the flash a person sees when swiping back.
 *
 * The answer is kept per history and per entry index, because the screen that is drawn by a back is
 * usually a screen that mounts only now and so cannot have watched the index change. Something that
 * stays mounted through the move (the workspace shell) asks first, in the render the move causes; every
 * screen below it asks the same question and gets the same answer.
 */
interface Track {
  index: number
  byDevice: boolean
  /** The app asked for a back itself (its own button, Esc): the animation of that step is the app's to draw. */
  ownBack: boolean
}

const tracks = new WeakMap<object, Track>()

/** The index of the entry the history is on; a lower one than the last seen means the person went back. */
export function isDeviceBack(history: object, index: number): boolean {
  const t = tracks.get(history)
  if (!t) {
    tracks.set(history, { index, byDevice: false, ownBack: false })
    return false
  }
  if (t.index !== index) {
    t.byDevice = index < t.index && !t.ownBack
    t.ownBack = false
    t.index = index
  }
  return t.byDevice
}

/** Call just before the app takes a step back itself, so that step is not mistaken for the device's. */
export function markOwnBack(history: object): void {
  const t = tracks.get(history)
  if (t) t.ownBack = true
}

/** Call when the app moves within an entry (a replace): from here the screen is the app's to animate again. */
export function markOwnStep(history: object): void {
  const t = tracks.get(history)
  if (t) t.byDevice = false
}
