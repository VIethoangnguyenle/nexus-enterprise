import { Spinner } from './primitives'

interface LoadingStateProps {
  size?: number
}

/** Full-section centered spinner for query loading states. */
export function LoadingState({ size = 40 }: LoadingStateProps) {
  return (
    <div className="flex items-center justify-center py-16">
      <Spinner size={size <= 24 ? 'sm' : size <= 32 ? 'md' : 'lg'} />
    </div>
  )
}
