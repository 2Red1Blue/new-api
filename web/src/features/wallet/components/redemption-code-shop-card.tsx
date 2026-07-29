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
import { ExternalLink, ShoppingBag } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import { cn } from '@/lib/utils'

const REDEMPTION_CODE_SHOP_URL = 'https://pay.ldxp.cn/shop/6IKDTWVQ'

export function RedemptionCodeShopCard() {
  const { t } = useTranslation()
  const [loaded, setLoaded] = useState(false)

  return (
    <TitledCard
      title={t('Redemption Code Shop')}
      description={t('Buy a redemption code here, then redeem it above.')}
      icon={<ShoppingBag />}
      iconTone='info'
      disableHoverEffect
      contentClassName='p-0'
      action={
        <Button
          variant='outline'
          size='sm'
          className='w-full sm:w-auto'
          render={
            <a
              href={REDEMPTION_CODE_SHOP_URL}
              target='_blank'
              rel='noopener noreferrer'
            />
          }
        >
          {t('Open in new window')}
          <ExternalLink className='size-3.5' />
        </Button>
      }
    >
      <div className='bg-muted/10 relative h-[72vh] max-h-[900px] min-h-[640px] overflow-hidden'>
        {!loaded && (
          <div className='absolute inset-0 z-10 space-y-4 p-4 sm:p-6'>
            <Skeleton className='h-10 w-2/3' />
            <Skeleton className='h-24 w-full' />
            <Skeleton className='h-48 w-full' />
            <Skeleton className='h-12 w-full' />
          </div>
        )}
        {/* eslint-disable-next-line react/iframe-missing-sandbox -- The fixed cross-origin checkout needs scripts, storage, and payment redirects. */}
        <iframe
          src={REDEMPTION_CODE_SHOP_URL}
          title={t('Redemption Code Shop')}
          loading='lazy'
          referrerPolicy='strict-origin-when-cross-origin'
          allow='clipboard-read; clipboard-write; payment'
          onLoad={() => setLoaded(true)}
          className={cn(
            'bg-background h-full w-full border-0 transition-opacity duration-300',
            loaded ? 'opacity-100' : 'opacity-0'
          )}
        />
      </div>
    </TitledCard>
  )
}
