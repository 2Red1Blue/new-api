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
import { motion, useReducedMotion } from 'motion/react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

interface BackgroundBeamsWithCollisionProps {
  children: ReactNode
  className?: string
}

interface BeamConfig {
  color: string
  delay: number
  duration: number
  left: number
  repeatDelay: number
  rotate: number
  width: number
}

const beams: BeamConfig[] = [
  {
    color: 'oklch(0.7 0.19 255)',
    delay: 0.2,
    duration: 7.4,
    left: 8,
    repeatDelay: 2.8,
    rotate: -5,
    width: 1,
  },
  {
    color: 'oklch(0.72 0.18 285)',
    delay: 1.6,
    duration: 8.8,
    left: 23,
    repeatDelay: 3.4,
    rotate: 4,
    width: 1.5,
  },
  {
    color: 'oklch(0.76 0.15 210)',
    delay: 0.8,
    duration: 6.8,
    left: 39,
    repeatDelay: 2.4,
    rotate: -3,
    width: 1,
  },
  {
    color: 'oklch(0.7 0.2 265)',
    delay: 2.4,
    duration: 9.2,
    left: 55,
    repeatDelay: 3,
    rotate: 5,
    width: 2,
  },
  {
    color: 'oklch(0.74 0.17 300)',
    delay: 1.1,
    duration: 7.8,
    left: 70,
    repeatDelay: 3.6,
    rotate: -4,
    width: 1.5,
  },
  {
    color: 'oklch(0.78 0.14 205)',
    delay: 3,
    duration: 8.4,
    left: 84,
    repeatDelay: 2.7,
    rotate: 3,
    width: 1,
  },
  {
    color: 'oklch(0.69 0.19 250)',
    delay: 4.2,
    duration: 9.6,
    left: 94,
    repeatDelay: 3.2,
    rotate: -6,
    width: 1.5,
  },
]

const particleVectors = [
  { x: -42, y: -32 },
  { x: -24, y: -48 },
  { x: -8, y: -38 },
  { x: 10, y: -52 },
  { x: 28, y: -34 },
  { x: 44, y: -20 },
  { x: -36, y: -12 },
  { x: 34, y: -6 },
]

function AnimatedBeam({ beam }: { beam: BeamConfig }) {
  const transition = {
    delay: beam.delay,
    duration: beam.duration,
    ease: 'linear' as const,
    repeat: Number.POSITIVE_INFINITY,
    repeatDelay: beam.repeatDelay,
  }

  return (
    <>
      <motion.div
        className='absolute top-0 h-36 rounded-full opacity-0 blur-[0.3px] will-change-[top,opacity]'
        style={{
          left: `${beam.left}%`,
          width: beam.width,
          rotate: beam.rotate,
          background: `linear-gradient(to bottom, transparent, ${beam.color} 45%, white)`,
          boxShadow: `0 0 14px ${beam.color}`,
        }}
        initial={{ opacity: 0, top: '-9rem' }}
        animate={{
          opacity: [0, 0.8, 0.8, 0],
          top: ['-9rem', '-6rem', 'calc(100% - 2rem)', 'calc(100% + 4rem)'],
        }}
        transition={{ ...transition, times: [0, 0.08, 0.86, 1] }}
      />

      <motion.div
        className='absolute bottom-0 size-2 -translate-x-1/2 rounded-full'
        style={{
          left: `${beam.left}%`,
          backgroundColor: beam.color,
          boxShadow: `0 0 22px 8px ${beam.color}`,
        }}
        initial={{ opacity: 0, scale: 0.3 }}
        animate={{
          opacity: [0, 0, 0.95, 0],
          scale: [0.3, 0.3, 1.8, 3.2],
        }}
        transition={{ ...transition, times: [0, 0.84, 0.87, 1] }}
      >
        <motion.span
          className='absolute top-1/2 left-1/2 size-10 -translate-x-1/2 -translate-y-1/2 rounded-full border'
          style={{ borderColor: beam.color }}
          initial={{ opacity: 0, scale: 0 }}
          animate={{ opacity: [0, 0, 0.7, 0], scale: [0, 0, 1, 2.6] }}
          transition={{ ...transition, times: [0, 0.84, 0.9, 1] }}
        />

        {particleVectors.map((vector, index) => (
          <motion.span
            key={index}
            className='absolute top-1/2 left-1/2 size-1 rounded-full'
            style={{ backgroundColor: beam.color }}
            initial={{ opacity: 0, x: 0, y: 0 }}
            animate={{
              opacity: [0, 0, 1, 0],
              x: [0, 0, vector.x, vector.x * 1.25],
              y: [0, 0, vector.y, vector.y * 1.2],
            }}
            transition={{ ...transition, times: [0, 0.85, 0.9, 1] }}
          />
        ))}
      </motion.div>
    </>
  )
}

export function BackgroundBeamsWithCollision({
  children,
  className,
}: BackgroundBeamsWithCollisionProps) {
  const shouldReduceMotion = useReducedMotion()

  return (
    <div
      className={cn(
        'bg-background relative isolate w-full overflow-hidden',
        className
      )}
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 z-0 overflow-hidden [contain:layout_paint]'
      >
        <div className='absolute inset-0 bg-[radial-gradient(ellipse_70%_55%_at_50%_10%,oklch(0.7_0.18_255_/_0.12),transparent_72%)] dark:bg-[radial-gradient(ellipse_70%_55%_at_50%_10%,oklch(0.7_0.18_255_/_0.08),transparent_72%)]' />

        {!shouldReduceMotion &&
          beams.map((beam) => <AnimatedBeam key={beam.left} beam={beam} />)}

        <div className='bg-border/50 absolute inset-x-0 bottom-0 h-px shadow-[0_-10px_42px_oklch(0.7_0.18_255_/_0.18)]' />
        <div className='from-background/90 absolute inset-x-0 bottom-0 h-24 bg-gradient-to-t to-transparent' />
      </div>

      <div className='relative z-10'>{children}</div>
    </div>
  )
}
