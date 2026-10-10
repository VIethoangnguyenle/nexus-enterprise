/** "Bước 2 trên 3": a thin bar per step, the ones reached filled with the accent. */
export function Steps({ current, total = 3 }: { current: number; total?: number }) {
  return (
    <div role="img" aria-label={`Bước ${current} trên ${total}`} className="flex gap-1.5">
      {Array.from({ length: total }, (_, i) => (
        <span key={i} className={`h-1 flex-1 rounded-full ${i < current ? 'bg-accent' : 'bg-line'}`} />
      ))}
    </div>
  )
}
