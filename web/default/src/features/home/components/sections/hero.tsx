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
import { CopyButton } from '@/components/copy-button'
import { useTranslation } from 'react-i18next'

const YUNXI_AI_TITLE = 'Yunxi AI'
const API_BASE = 'https://ai.laiber.cloud'
const API_PATHS = [
  '/v1/chat/completions',
  '/v1/images/generations',
  '/v1/audio/transcriptions',
  '/v1/embeddings',
]

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
          className='landing-animate-fade-up text-[13px] leading-7 tracking-[0.18em] text-white/60 md:text-sm'
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

          <p className='mt-5 animate-[title-float_7.2s_ease-in-out_infinite] text-[12px] font-light tracking-[0.42em] text-white/58 uppercase'>
            {titleSuffix}
          </p>
        </div>

        <div
          className='landing-animate-fade-up mt-24 flex flex-col items-center gap-3 text-[11px] text-white/58 opacity-0'
          style={{ animationDelay: '180ms' }}
        >
          <span className='tracking-[0.16em]'>{t('API base')}</span>
          <div className='flex items-center gap-2 rounded-full border border-white/10 bg-white/8 px-3 py-2 shadow-[0_16px_50px_rgba(5,10,18,0.32)] backdrop-blur-xl'>
            <div className='flex min-w-0 items-center rounded-full bg-white/10 px-4 py-2'>
              <code className='min-w-0 font-mono text-[11px] text-white/88 sm:text-[12px]'>
                <span className='whitespace-nowrap text-white/82'>
                  {API_BASE}
                </span>
                <span className='mx-2 text-white/26'>|</span>
                <span className='relative inline-flex h-[1.35em] min-w-[17.5ch] overflow-hidden align-bottom text-left text-[#8ebdff] sm:min-w-[20ch]'>
                  <span className='animate-api-path-scroll flex flex-col'>
                    {[...API_PATHS, API_PATHS[0]].map((path, index) => (
                      <span
                        key={`${path}-${index}`}
                        className='flex h-[1.35em] items-center whitespace-nowrap'
                      >
                        {path}
                      </span>
                    ))}
                  </span>
                </span>
              </code>
            </div>

            <CopyButton
              value={`${API_BASE}${API_PATHS[0]}`}
              className='size-9 rounded-full border border-white/10 bg-white/10 text-white/72 hover:bg-white/14 hover:text-white'
              iconClassName='size-4'
              tooltip={t('Copy API endpoint')}
              successTooltip={t('API endpoint copied!')}
              aria-label={t('Copy API endpoint')}
            />
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

        @keyframes api-path-scroll {
          0%, 18% {
            transform: translateY(0);
          }
          25%, 43% {
            transform: translateY(-1.35em);
          }
          50%, 68% {
            transform: translateY(-2.7em);
          }
          75%, 93% {
            transform: translateY(-4.05em);
          }
          100% {
            transform: translateY(-5.4em);
          }
        }

        .animate-api-path-scroll {
          animation: api-path-scroll 10s cubic-bezier(0.22, 1, 0.36, 1) infinite;
        }
      `}</style>
    </main>
  )
}
