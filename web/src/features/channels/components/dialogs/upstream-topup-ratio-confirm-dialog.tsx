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

import { ConfirmDialog } from '@/components/confirm-dialog'

type UpstreamTopupRatioConfirmDialogProps = {
  open: boolean
  currentRatio: number
  upstreamRatio: number
  onKeepCurrent: () => void
  onUseUpstream: () => void
}

export function UpstreamTopupRatioConfirmDialog(
  props: UpstreamTopupRatioConfirmDialogProps
) {
  const { t } = useTranslation()

  return (
    <ConfirmDialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) {
          props.onKeepCurrent()
        }
      }}
      title={t('Upstream top-up ratio differs')}
      desc={t(
        'The fetched upstream top-up ratio is {{upstream}}, while the current value is {{current}}. Use the upstream value?',
        {
          upstream: props.upstreamRatio,
          current: props.currentRatio,
        }
      )}
      cancelBtnText={t('Keep current')}
      confirmText={t('Use upstream')}
      handleConfirm={props.onUseUpstream}
    />
  )
}
