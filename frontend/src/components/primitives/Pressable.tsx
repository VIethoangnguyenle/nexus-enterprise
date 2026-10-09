import { type ButtonHTMLAttributes, forwardRef } from 'react'

/**
 * A real `<button>` with no chrome of its own: focus ring, cursor and
 * `type="button"` by default. For composite rows
 * whose layout is the caller's (a topic's reply summary, a section toggle, the
 * new-chat button) where none of Button's fixed geometries fit.
 *
 * Interactive rows that navigate should be a `<Link>` instead.
 */
export const Pressable = forwardRef<HTMLButtonElement, ButtonHTMLAttributes<HTMLButtonElement>>(
  ({ className = '', type = 'button', ...props }, ref) => (
    <button
      ref={ref}
      type={type}
      // No background or border here: preflight already clears both on <button>,
      // and declaring them would tie on specificity with the caller's own
      // bg-* and win or lose by stylesheet order.
      className={`text-left cursor-pointer focus-ring
        disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
      {...props}
    />
  ),
)
Pressable.displayName = 'Pressable'
