import { useState } from 'react'
import { Globe, Palette, Sparkles } from 'lucide-react'
import { useAccountLanguage } from '../../hooks/useAccountLanguage'
import { usePreferences } from '../../hooks/usePreferences'
import { AppearanceTab, LANGUAGE_CHOICES, THEME_CHOICES } from './AppearanceTab'
import { ChoicePage, SettingsGroup, SettingsPage, SettingsRow, SettingsSwitch, choiceLabel } from './SettingsGroup'
import { ACCOUNT_LEAD } from './settings-model'
import { useSettingsWide } from './settings-layout'
import { useSettingsNav } from './settings-nav'

const ICON = { size: 18, strokeWidth: 1.75 } as const

/**
 * Giao diện, ngôn ngữ: the theme and reduced motion live on this device (`usePreferences`), the language on the
 * account (auth). On a phone (Tôi → Giao diện, ngôn ngữ) each is a row; "Thu gọn sidebar" means nothing there and is
 * not shown. Inside the desktop shell the theme cards, the choices side by side and the sidebar switch.
 */
export function AppearanceScreen() {
  return useSettingsWide() ? <AppearanceOptions /> : <AppearanceRows />
}

function AppearanceOptions() {
  const { back } = useSettingsNav()
  return (
    <SettingsPage title="Giao diện, ngôn ngữ" lead={ACCOUNT_LEAD} onBack={back}>
      <AppearanceTab />
    </SettingsPage>
  )
}

function AppearanceRows() {
  const { back } = useSettingsNav()
  const { prefs, update } = usePreferences()
  const language = useAccountLanguage()
  const [choosing, setChoosing] = useState<'theme' | 'language' | null>(null)

  if (choosing === 'theme') {
    return (
      <ChoicePage
        title="Giao diện"
        value={prefs.theme}
        choices={THEME_CHOICES}
        note="Lưu trên máy này."
        onPick={(theme) => update({ theme })}
        onBack={() => setChoosing(null)}
      />
    )
  }
  if (choosing === 'language') {
    return (
      <ChoicePage
        title="Ngôn ngữ"
        value={language.value}
        choices={LANGUAGE_CHOICES}
        note="Dùng chung mọi workspace và mọi thiết bị của bạn."
        onPick={(v) => language.set(v)}
        onBack={() => setChoosing(null)}
      />
    )
  }

  return (
    <SettingsPage title="Giao diện, ngôn ngữ" onBack={back}>
      <SettingsGroup title="Trên máy này">
        <SettingsRow
          icon={<Palette {...ICON} />}
          label="Giao diện"
          value={choiceLabel(THEME_CHOICES, prefs.theme)}
          onPress={() => setChoosing('theme')}
        />
        <SettingsSwitch
          icon={<Sparkles {...ICON} />}
          label="Giảm chuyển động"
          hint="Tắt trượt và phóng to; giữ fade ngắn"
          checked={prefs.motion === 'reduce'}
          onChange={(on) => update({ motion: on ? 'reduce' : 'system' })}
        />
      </SettingsGroup>
      <SettingsGroup title="Tài khoản">
        <SettingsRow
          icon={<Globe {...ICON} />}
          label="Ngôn ngữ"
          value={choiceLabel(LANGUAGE_CHOICES, language.value)}
          onPress={language.ready ? () => setChoosing('language') : undefined}
        />
      </SettingsGroup>
    </SettingsPage>
  )
}
