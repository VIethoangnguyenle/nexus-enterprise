/**
 * A short, soft two-note chime: "Thử loa" in the pre-join check and the sound
 * another person's raised hand makes. Built from an oscillator so no audio file
 * ships. Resolves when it has played; a browser without WebAudio, or one that
 * has not allowed sound yet, just stays quiet.
 */
export async function playChime(sinkId?: string): Promise<void> {
  try {
    const Ctx: typeof AudioContext | undefined =
      window.AudioContext ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctx) return
    const ctx = new Ctx()
    const sink = ctx as AudioContext & { setSinkId?: (id: string) => Promise<void> }
    if (sinkId && sink.setSinkId) await sink.setSinkId(sinkId).catch(() => {})
    const now = ctx.currentTime
    const gain = ctx.createGain()
    gain.connect(ctx.destination)
    gain.gain.setValueAtTime(0.0001, now)
    gain.gain.exponentialRampToValueAtTime(0.12, now + 0.03)
    gain.gain.exponentialRampToValueAtTime(0.0001, now + 0.5)
    for (const [i, hz] of [660, 880].entries()) {
      const osc = ctx.createOscillator()
      osc.type = 'sine'
      osc.frequency.value = hz
      osc.connect(gain)
      osc.start(now + i * 0.18)
      osc.stop(now + i * 0.18 + 0.28)
    }
    await new Promise((r) => setTimeout(r, 700))
    await ctx.close()
  } catch {
    /* no sound is not an error */
  }
}
