/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { CHANNEL_FORM_DEFAULT_VALUES, channelFormSchema } from '../channel-form'

function validChannelForm(overrides: { upstream_rpm_limit: number }) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'Test channel',
    type: 1,
    models: 'gpt-4',
    group: ['default'],
    upstream_rpm_limit: overrides.upstream_rpm_limit,
  }
}

describe('channel upstream RPM setting', () => {
  test('accepts zero as the unlimited value', () => {
    const result = channelFormSchema.safeParse(
      validChannelForm({ upstream_rpm_limit: 0 })
    )

    assert.equal(result.success, true)
  })

  test('rejects negative and fractional limits', () => {
    const negativeResult = channelFormSchema.safeParse(
      validChannelForm({ upstream_rpm_limit: -1 })
    )
    const fractionalResult = channelFormSchema.safeParse(
      validChannelForm({ upstream_rpm_limit: 1.5 })
    )

    assert.equal(negativeResult.success, false)
    assert.equal(fractionalResult.success, false)
  })
})
