import { messageText } from '../../lib/message-model'
import { toast } from '../primitives'

/**
 * Copies a message as plain text, line breaks kept: text selection is off on
 * touch screens (the long press opens the sheet), so this is how a phone copies.
 */
export function copyText(content: string) {
  navigator.clipboard?.writeText(messageText(content)).then(
    () => toast('Đã sao chép'),
    () => toast('Không sao chép được'),
  )
}
