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
import { useTranslation } from 'react-i18next'

const YUNXI_AI_TITLE = 'Yunxi AI'

interface HeroProps {
  className?: string
}

export function Hero(_props: HeroProps) {
  const { t } = useTranslation()
  const title = t(YUNXI_AI_TITLE)
  const titleBase = title.replace(' AI', '')
  const titleSuffix = 'AI Relay'

  return (
    <main className='relative flex min-h-[calc(100svh-64px)] items-center justify-center overflow-hidden bg-[#070a13] px-6 pt-24 pb-16 text-white'>
      <iframe
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 z-0 size-full border-0'
        src='/yunxi-background.html?embed=1'
        title=''
        tabIndex={-1}
      />
      <div aria-hidden className='absolute inset-0 z-0 bg-black/15' />

      <section className='relative z-10 mx-auto flex w-full max-w-4xl flex-col items-center text-center'>
        <div
          className='landing-animate-fade-up text-[13px] leading-7 tracking-[0.18em] text-white/48 md:text-sm'
          style={{ animationDelay: '0ms' }}
        >
          <p>{t('Clouds pass without a sound,')}</p>
          <p>{t('and the stream carries light forward.')}</p>
        </div>

        <div
          className='landing-animate-fade-up mt-14 flex flex-col items-center opacity-0'
          style={{ animationDelay: '100ms' }}
        >
          <h1 className='text-[clamp(4.2rem,14vw,8.1rem)] leading-none text-white drop-shadow-[0_24px_80px_rgba(0,0,0,0.52)]'>
            <span
              className='relative inline-block animate-[title-float_7.2s_ease-in-out_infinite] font-medium tracking-[0.08em] text-white/98 after:absolute after:inset-x-[-0.05em] after:top-1/2 after:h-[0.52em] after:-translate-y-1/2 after:animate-[title-sheen_5.8s_ease-in-out_infinite] after:bg-[linear-gradient(90deg,transparent_0%,rgba(214,224,255,0.04)_18%,rgba(255,255,255,0.22)_50%,rgba(214,224,255,0.05)_82%,transparent_100%)] after:blur-[10px] after:content-[\"\"]'
              style={{
                fontFamily:
                  '"Noto Serif SC", "Songti SC", "STSong", "Source Han Serif SC", serif',
              }}
            >
              <span className='bg-[linear-gradient(180deg,rgba(255,255,255,1)_0%,rgba(245,247,255,0.96)_48%,rgba(214,223,255,0.86)_100%)] bg-clip-text text-transparent'>
                {titleBase}
              </span>
            </span>
          </h1>

          <p className='mt-5 animate-[title-float_7.2s_ease-in-out_infinite] text-[12px] font-light tracking-[0.42em] text-white/44 uppercase'>
            {titleSuffix}
          </p>
        </div>

        <div
          className='landing-animate-fade-up mt-24 flex flex-col items-center gap-3 text-[11px] text-white/42 opacity-0'
          style={{ animationDelay: '180ms' }}
        >
          <span className='tracking-[0.16em]'>{t('API base')}</span>
          <div className='rounded-full border border-white/10 bg-white/6 px-4 py-2 backdrop-blur'>
            <code className='font-mono text-[11px] text-white/88'>
              https://ai.laiber.cloud/v1
            </code>
          </div>
        </div>
      </section>

      <style>{`
        @keyframes title-float {
          0%, 100% {
            transform: translate3d(0, 0, 0);
          }
          50% {
            transform: translate3d(0, -6px, 0);
          }
        }

        @keyframes title-sheen {
          0%, 100% {
            opacity: 0.28;
            transform: translateX(-10%) translateY(-50%);
          }
          50% {
            opacity: 0.9;
            transform: translateX(10%) translateY(-50%);
          }
        }
      `}</style>
    </main>
  )
}
