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
import { useCallback, useMemo, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { cleanupModelAccess } from '../api'
import type { ModelAccessCleanupMode } from '../types'

function parseModelNames(value: string) {
  const seen = new Set<string>()
  return value
    .split(/[,\n\r\t]+/)
    .map((item) => item.trim())
    .filter((item) => {
      if (!item || seen.has(item)) return false
      seen.add(item)
      return true
    })
}

export function ModelAccessCleanup() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [rawModels, setRawModels] = useState('')
  const [mode, setMode] = useState<ModelAccessCleanupMode>('remove')
  const [confirmOpen, setConfirmOpen] = useState(false)

  const models = useMemo(() => parseModelNames(rawModels), [rawModels])

  const cleanupMutation = useMutation({
    mutationFn: cleanupModelAccess,
    onSuccess: (data) => {
      if (!data.success || !data.data) {
        toast.error(data.message || t('Failed to clean up model access'))
        return
      }

      const changedAbilities =
        data.data.mode === 'remove'
          ? (data.data.deleted_abilities ?? 0)
          : (data.data.disabled_abilities ?? 0)
      toast.success(
        t(
          'Cleaned up {{models}} model(s), updated {{channels}} channel(s), changed {{abilities}} ability record(s).',
          {
            models: data.data.models.length,
            channels: data.data.updated_channels,
            abilities: changedAbilities,
          }
        )
      )
      setRawModels('')
      setConfirmOpen(false)
      queryClient.invalidateQueries({ queryKey: ['system-options'] })
      queryClient.invalidateQueries({ queryKey: ['pricing'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to clean up model access'))
    },
  })

  const handleSubmit = useCallback(() => {
    if (models.length === 0) {
      toast.error(t('Enter at least one model name'))
      return
    }
    setConfirmOpen(true)
  }, [models.length, t])

  const handleConfirm = useCallback(() => {
    cleanupMutation.mutate({ models, mode })
  }, [cleanupMutation, mode, models])

  const isPending = cleanupMutation.isPending

  return (
    <Alert variant='destructive'>
      <Trash2 className='h-4 w-4' />
      <AlertTitle>{t('Bulk clean up model access')}</AlertTitle>
      <AlertDescription className='space-y-4'>
        <p>
          {t(
            'Remove unavailable models from every channel model list and update matching ability records.'
          )}
        </p>
        <div className='grid gap-3 lg:grid-cols-[1fr_18rem]'>
          <div className='space-y-2'>
            <Label htmlFor='model-access-cleanup-models'>
              {t('Model names')}
            </Label>
            <Textarea
              id='model-access-cleanup-models'
              rows={3}
              value={rawModels}
              onChange={(event) => setRawModels(event.target.value)}
              placeholder='gpt-5.2-chat-latest, claude-haiku-4-5'
              disabled={isPending}
              className='bg-background/70'
            />
          </div>

          <div className='space-y-2'>
            <Label>{t('Cleanup mode')}</Label>
            <RadioGroup
              value={mode}
              onValueChange={(value) =>
                setMode(value as ModelAccessCleanupMode)
              }
              className='gap-2'
            >
              <label className='border-border bg-background/70 flex cursor-pointer gap-2 rounded-lg border p-2'>
                <RadioGroupItem value='remove' className='mt-0.5' />
                <span className='space-y-0.5'>
                  <span className='block font-medium'>{t('Remove')}</span>
                  <span className='text-muted-foreground block text-xs'>
                    {t('Delete matching ability records.')}
                  </span>
                </span>
              </label>
              <label className='border-border bg-background/70 flex cursor-pointer gap-2 rounded-lg border p-2'>
                <RadioGroupItem value='disable' className='mt-0.5' />
                <span className='space-y-0.5'>
                  <span className='block font-medium'>{t('Disable')}</span>
                  <span className='text-muted-foreground block text-xs'>
                    {t('Keep matching ability records but mark them disabled.')}
                  </span>
                </span>
              </label>
            </RadioGroup>
          </div>
        </div>

        <div className='flex justify-end'>
          <Button
            type='button'
            variant='destructive'
            onClick={handleSubmit}
            disabled={isPending || models.length === 0}
          >
            {isPending ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <Trash2 className='mr-2 h-4 w-4' />
            )}
            {t('Clean up model access')}
          </Button>
        </div>
      </AlertDescription>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Clean up model access?')}
        desc={
          <div className='space-y-3'>
            <p>
              {mode === 'remove'
                ? t(
                    'This will remove the selected models from every channel and delete matching ability records.'
                  )
                : t(
                    'This will remove the selected models from every channel and disable matching ability records.'
                  )}
            </p>
            <div className='bg-muted max-h-36 overflow-auto rounded-lg p-2 text-xs'>
              {models.map((model) => (
                <div key={model}>{model}</div>
              ))}
            </div>
          </div>
        }
        destructive
        isLoading={isPending}
        handleConfirm={handleConfirm}
        confirmText={t('Clean up')}
      />
    </Alert>
  )
}
