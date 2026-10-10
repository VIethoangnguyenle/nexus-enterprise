import { motion } from 'motion/react'
import type { Burst } from '../../hooks/useArrivals'
import type { PeopleDirectory } from '../../lib/people'
import type { MotionPresets } from '../../lib/motion'
import { Avatar } from '../primitives'

export function nameOfAuthor(hueKey: string | undefined, people: PeopleDirectory): string {
  return (hueKey && people.byUserId.get(hueKey)?.name) || 'Một người'
}

/** One summary line when several people changed the folder at once. `row` is the motion preset for it. */
export function DriveBurstBanner({ burst, people, row }: {
  burst: Burst
  people: PeopleDirectory
  row: MotionPresets['row']
}) {
  return (
    <motion.div key="burst" role="status" {...row} className="mx-5 overflow-hidden">
      <div className="flex items-center gap-2.5 mb-2 px-3 py-2 rounded-surface bg-raised text-sm">
        <span className="flex items-center">
          {burst.authors.slice(0, 3).map((a, i) => (
            <Avatar key={a} name={nameOfAuthor(a, people)} hueKey={a} size={20} className={`ring-2 ring-raised ${i > 0 ? '-ml-1.5' : ''}`} />
          ))}
        </span>
        <span>
          <b className="font-semibold">{nameOfAuthor(burst.authors[0], people)}</b>
          {burst.authors.length > 1 ? ` và ${burst.authors.length - 1} người khác` : ''} vừa cập nhật thư mục này
        </span>
      </div>
    </motion.div>
  )
}
