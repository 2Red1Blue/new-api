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
import * as React from 'react'
import { createPortal } from 'react-dom'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Input } from '@/components/ui/input'

export type ComboboxInputOption = {
  value: string
  label: string
  icon?: React.ReactNode
}

interface ComboboxInputProps {
  options: ComboboxInputOption[]
  value?: string
  onValueChange: (value: string) => void
  placeholder?: string
  emptyText?: string
  className?: string
  id?: string
  allowCustomValue?: boolean
}

export function ComboboxInput({
  options,
  value = '',
  onValueChange,
  placeholder = 'Select or type...',
  emptyText = 'No option found.',
  className,
  id,
  allowCustomValue = false,
}: ComboboxInputProps) {
  const { t } = useTranslation()
  const [open, setOpen] = React.useState(false)
  const [searchValue, setSearchValue] = React.useState('')
  const [highlightedIndex, setHighlightedIndex] = React.useState(-1)
  const containerRef = React.useRef<HTMLDivElement>(null)
  const inputRef = React.useRef<HTMLInputElement>(null)
  const dropdownRef = React.useRef<HTMLDivElement>(null)
  const listRef = React.useRef<HTMLUListElement>(null)
  const [dropdownStyle, setDropdownStyle] =
    React.useState<React.CSSProperties | null>(null)
  const selectedOption = React.useMemo(
    () => options.find((option) => option.value === value),
    [options, value]
  )
  const displayValue = open ? searchValue : (selectedOption?.label ?? value)

  const filteredOptions = React.useMemo(() => {
    if (!searchValue.trim()) return options
    const search = searchValue.toLowerCase().trim()
    return options.filter(
      (option) =>
        option.label.toLowerCase().includes(search) ||
        option.value.toLowerCase().includes(search)
    )
  }, [options, searchValue])

  // Reset highlight when filtered options change
  React.useEffect(() => {
    setHighlightedIndex(-1)
  }, [filteredOptions])

  // Handle click outside to close
  React.useEffect(() => {
    if (!open) return

    const handleClickOutside = (e: MouseEvent) => {
      const target = e.target as Node
      if (
        containerRef.current &&
        !containerRef.current.contains(target) &&
        !dropdownRef.current?.contains(target)
      ) {
        setOpen(false)
        setSearchValue('')
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [open])

  const handleSelect = (selectedValue: string) => {
    onValueChange(selectedValue)
    setOpen(false)
    setSearchValue('')
    inputRef.current?.focus()
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (!open && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      setOpen(true)
      return
    }

    if (!open) return

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setHighlightedIndex((prev) =>
          prev < filteredOptions.length - 1 ? prev + 1 : 0
        )
        break
      case 'ArrowUp':
        e.preventDefault()
        setHighlightedIndex((prev) =>
          prev > 0 ? prev - 1 : filteredOptions.length - 1
        )
        break
      case 'Enter':
        e.preventDefault()
        if (highlightedIndex >= 0 && filteredOptions[highlightedIndex]) {
          handleSelect(filteredOptions[highlightedIndex].value)
        } else if (allowCustomValue && searchValue.trim()) {
          handleSelect(searchValue.trim())
        } else {
          // No highlighted option, just close the dropdown and keep current value
          setOpen(false)
          setSearchValue('')
        }
        break
      case 'Escape':
        e.preventDefault()
        setOpen(false)
        setSearchValue('')
        break
    }
  }

  // Scroll highlighted item into view
  React.useEffect(() => {
    if (highlightedIndex < 0 || !listRef.current) return
    const item = listRef.current.children[highlightedIndex] as HTMLElement
    item?.scrollIntoView({ block: 'nearest' })
  }, [highlightedIndex])

  const showDropdown =
    open &&
    (filteredOptions.length > 0 || (allowCustomValue && searchValue.trim()))

  const updateDropdownPosition = React.useCallback(() => {
    if (!inputRef.current) return

    const rect = inputRef.current.getBoundingClientRect()
    const gap = 4
    const viewportPadding = 8
    const preferredMaxHeight = 200
    const availableBelow = window.innerHeight - rect.bottom - gap - viewportPadding
    const availableAbove = rect.top - gap - viewportPadding
    const shouldOpenAbove = availableBelow < 120 && availableAbove > availableBelow
    const availableHeight = shouldOpenAbove ? availableAbove : availableBelow
    const maxHeight = Math.min(
      preferredMaxHeight,
      Math.max(120, availableHeight)
    )

    setDropdownStyle({
      position: 'fixed',
      left: rect.left,
      top: shouldOpenAbove
        ? Math.max(viewportPadding, rect.top - gap - maxHeight)
        : rect.bottom + gap,
      width: rect.width,
      maxHeight,
      zIndex: 100,
    })
  }, [])

  React.useLayoutEffect(() => {
    if (!showDropdown) {
      setDropdownStyle(null)
      return
    }

    updateDropdownPosition()
  }, [showDropdown, updateDropdownPosition, filteredOptions.length])

  React.useEffect(() => {
    if (!showDropdown) return

    window.addEventListener('resize', updateDropdownPosition)
    window.addEventListener('scroll', updateDropdownPosition, true)

    return () => {
      window.removeEventListener('resize', updateDropdownPosition)
      window.removeEventListener('scroll', updateDropdownPosition, true)
    }
  }, [showDropdown, updateDropdownPosition])

  const dropdown = showDropdown && dropdownStyle && (
    <div
      ref={dropdownRef}
      className='bg-popover text-popover-foreground rounded-md border shadow-md'
      style={dropdownStyle}
    >
      {filteredOptions.length > 0 ? (
        <ul
          ref={listRef}
          role='listbox'
          className='overflow-y-auto p-1'
          style={{ maxHeight: dropdownStyle.maxHeight }}
        >
          {filteredOptions.map((option, index) => (
            <li
              key={option.value}
              role='option'
              aria-selected={value === option.value}
              data-highlighted={index === highlightedIndex}
              className={cn(
                'relative flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm select-none',
                index === highlightedIndex &&
                  'bg-accent text-accent-foreground',
                value === option.value && 'font-medium'
              )}
              onMouseEnter={() => setHighlightedIndex(index)}
              onMouseDown={(e) => {
                e.preventDefault()
                handleSelect(option.value)
              }}
            >
              <Check
                className={cn(
                  'size-4 shrink-0',
                  value === option.value ? 'opacity-100' : 'opacity-0'
                )}
              />
              {option.icon && <span>{option.icon}</span>}
              <span className='truncate'>{option.label}</span>
            </li>
          ))}
        </ul>
      ) : (
        <div className='px-2 py-6 text-center text-sm'>
          {t(emptyText)}
          {allowCustomValue && searchValue.trim() && (
            <div className='text-muted-foreground mt-1 text-xs'>
              {t('Press Enter to use "{{value}}"', {
                value: searchValue.trim(),
              })}
            </div>
          )}
        </div>
      )}
    </div>
  )

  return (
    <div ref={containerRef} className='relative'>
      <Input
        ref={inputRef}
        id={id}
        type='text'
        role='combobox'
        aria-expanded={open}
        aria-haspopup='listbox'
        aria-autocomplete='list'
        autoComplete='off'
        placeholder={placeholder}
        value={displayValue}
        onChange={(e) => {
          const nextValue = e.target.value
          setSearchValue(nextValue)
          if (allowCustomValue) {
            onValueChange(nextValue)
          }
          if (!open) setOpen(true)
        }}
        onFocus={() => {
          setSearchValue(allowCustomValue && !selectedOption ? value : '')
          setOpen(true)
        }}
        onKeyDown={handleKeyDown}
        className={cn('pr-9', className)}
      />
      <ChevronsUpDown className='pointer-events-none absolute top-1/2 right-3 size-4 shrink-0 -translate-y-1/2 opacity-50' />

      {dropdown ? createPortal(dropdown, document.body) : null}
    </div>
  )
}
