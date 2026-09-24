import { create } from 'zustand'
import { listTimeEntries, getUnbilledSummary } from '@/api/timeEntry'
import type { TimeEntry, UnbilledSummary } from '@/types'

interface TimeEntryState {
  byCase: TimeEntry[]
  unbilled: UnbilledSummary
  fetchByCase: (caseId: number) => Promise<void>
  fetchUnbilledSummary: (caseId: number) => Promise<void>
}

export const useTimeEntryStore = create<TimeEntryState>((set) => ({
  byCase: [],
  unbilled: { hours: 0, amount: 0, count: 0 },
  async fetchByCase(caseId: number) {
    const res: any = await listTimeEntries(caseId)
    set({ byCase: res.data || [] })
  },
  async fetchUnbilledSummary(caseId: number) {
    const res: any = await getUnbilledSummary(caseId)
    set({ unbilled: res.data })
  },
}))
