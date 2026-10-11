import { useEffect, useState } from 'react'

/**
 * `flag`, but only once it has stayed true for `ms`: a save that answers at
 * once never flashes "Đang lưu…". It goes false the moment `flag` does.
 */
export function useDelayedFlag(flag: boolean, ms: number): boolean {
  const [late, setLate] = useState(false)
  useEffect(() => {
    if (!flag) {
      setLate(false)
      return
    }
    const timer = setTimeout(() => setLate(true), ms)
    return () => clearTimeout(timer)
  }, [flag, ms])
  return flag && late
}
