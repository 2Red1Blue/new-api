/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { after, describe, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLButtonElement',
  'SVGElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { UpstreamTopupRatioConfirmDialog } =
  await import('../upstream-topup-ratio-confirm-dialog')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Upstream top-up ratio differs': 'Upstream top-up ratio differs',
        'The fetched upstream top-up ratio is {{upstream}}, while the current value is {{current}}. Use the upstream value?':
          'The fetched upstream top-up ratio is {{upstream}}, while the current value is {{current}}. Use the upstream value?',
        'Keep current': 'Keep current',
        'Use upstream': 'Use upstream',
      },
    },
  },
})

const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

describe('upstream top-up ratio confirmation dialog', () => {
  after(() => {
    domWindow.close()
  })

  test('shows both ratios and applies the upstream value only after confirmation', async () => {
    const container = document.createElement('div')
    document.body.append(container)
    const root = createRoot(container)
    let useUpstreamCount = 0

    await act(async () => {
      root.render(
        <I18nextProvider i18n={i18n}>
          <UpstreamTopupRatioConfirmDialog
            open
            currentRatio={1}
            upstreamRatio={1.25}
            onKeepCurrent={() => undefined}
            onUseUpstream={() => {
              useUpstreamCount++
            }}
          />
        </I18nextProvider>
      )
    })

    assert.match(document.body.textContent || '', /current value is 1/)
    assert.match(document.body.textContent || '', /ratio is 1\.25/)
    const useUpstreamButton = [...document.querySelectorAll('button')].find(
      (button) => button.textContent === 'Use upstream'
    )
    assert.ok(useUpstreamButton)

    await act(async () => useUpstreamButton.click())
    assert.equal(useUpstreamCount, 1)

    await act(async () => root.unmount())
    container.remove()
  })
})
