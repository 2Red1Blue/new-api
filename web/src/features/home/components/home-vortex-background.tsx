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
import { lazy, Suspense } from 'react'

import { useTheme } from '@/context/theme-provider'

const Vortex = lazy(() => import('@/components/ui/vortex'))

export function HomeVortexBackground() {
  const { resolvedTheme } = useTheme()
  const dark = resolvedTheme === 'dark'

  return (
    <div
      aria-hidden='true'
      className='pointer-events-none absolute inset-0 z-0 overflow-hidden [mask-image:linear-gradient(to_bottom,black_0%,black_82%,transparent_100%)] opacity-35 dark:opacity-55'
    >
      <Suspense fallback={null}>
        <Vortex
          background='transparent'
          topRadius={340}
          waistRadius={52}
          waistPosition={48}
          bottomRadius={980}
          twist={3}
          zoom={72}
          speed={7}
          direction='right'
          lineOptions={{
            count: 126,
            color: dark ? '#93c5fd' : '#2563eb',
            glow: 6,
          }}
          dots
          dotOptions={{
            count: 3700,
            size: 16,
            color: dark ? '#c4b5fd' : '#7c3aed',
            glow: 5,
            flicker: 6,
          }}
          comets
          cometOptions={{
            count: 7,
            speed: 5,
            color: '#f97316',
            glow: 5,
            tail: 14,
            delay: 10,
            collide: 4,
          }}
          repel={false}
          style={{ transform: 'scale(1.08)' }}
        />
      </Suspense>
    </div>
  )
}
