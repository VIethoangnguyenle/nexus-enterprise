import { Component, type ErrorInfo, type ReactNode } from 'react'
import { ErrorState } from './ErrorState'

interface Props {
  children: ReactNode
  /** Fallback UI shown when an error is caught. Defaults to ErrorState. */
  fallback?: ReactNode
  /** Name of the module boundary for structured logging. */
  moduleName?: string
}

interface State {
  hasError: boolean
  error: Error | null
}

/**
 * ErrorBoundary catches unhandled rendering errors in a subtree and
 * renders a recovery UI instead of crashing the entire app.
 *
 * Usage:
 *   <ErrorBoundary moduleName="chat">
 *     <ChatView />
 *   </ErrorBoundary>
 */
export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    const module = this.props.moduleName || 'unknown'
    console.error(`[ErrorBoundary:${module}]`, error, info.componentStack)
  }

  private handleRetry = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback
      return (
        <div className="flex-1 flex items-center justify-center p-8">
          <ErrorState
            title="Something went wrong"
            message={this.state.error?.message || 'An unexpected error occurred'}
            onRetry={this.handleRetry}
          />
        </div>
      )
    }
    return this.props.children
  }
}
