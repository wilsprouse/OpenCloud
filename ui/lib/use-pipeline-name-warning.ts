'use client'

import { ClipboardEvent, FormEvent, useState } from "react"
import { toast } from "sonner"
import { getPipelineNameWarnings, sanitizePipelineName } from "@/lib/pipeline-name"

export const usePipelineNameWarning = (setValue: (value: string) => void) => {
  const [lastWarning, setLastWarning] = useState<string | null>(null)

  const showWarning = (value: string) => {
    const warnings = getPipelineNameWarnings(value)
    const nextWarning = warnings.length > 0 ? warnings.join(" ") : null

    if (nextWarning && nextWarning !== lastWarning) {
      toast.warning(nextWarning)
      setLastWarning(nextWarning)
    } else if (!nextWarning && lastWarning) {
      setLastWarning(null)
    }

    return nextWarning
  }

  const handleChange = (value: string) => {
    showWarning(value)
    setValue(sanitizePipelineName(value))
  }

  const handleBeforeInput = (event: FormEvent<HTMLInputElement>) => {
    const inputEvent = event.nativeEvent as InputEvent
    const attemptedValue = inputEvent.data

    if (!attemptedValue) return

    const target = event.currentTarget
    const selectionStart = target.selectionStart ?? 0
    const selectionEnd = target.selectionEnd ?? selectionStart
    const nextValue = `${target.value.slice(0, selectionStart)}${attemptedValue}${target.value.slice(selectionEnd)}`

    showWarning(nextValue)
  }

  const handlePaste = (event: ClipboardEvent<HTMLInputElement>) => {
    const attemptedValue = event.clipboardData.getData("text")
    const target = event.currentTarget
    const selectionStart = target.selectionStart ?? 0
    const selectionEnd = target.selectionEnd ?? selectionStart
    const nextValue = `${target.value.slice(0, selectionStart)}${attemptedValue}${target.value.slice(selectionEnd)}`

    showWarning(nextValue)
  }

  const resetWarning = () => {
    setLastWarning(null)
  }

  return {
    handleBeforeInput,
    handleChange,
    handlePaste,
    resetWarning,
  }
}
