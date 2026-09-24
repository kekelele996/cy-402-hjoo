import { create } from 'zustand'
import {
  listTimeEntriesByCase,
  createTimeEntry,
  updateTimeEntry,
  deleteTimeEntry,
  getTimeSummary,
  listTimeEntriesByBilling,
  type TimeEntryPayload,
} from '@/api/timeEntry'
import type { TimeEntry, TimeSummary } from '@/types'

interface TimeEntryState {
  byCase: TimeEntry[]
  summary: TimeSummary
  sources: Record<number, TimeEntry[]>
  fetchByCase: (caseId: number) => Promise<void>
  fetchSummary: (caseId: number) => Promise<void>
  create: (caseId: number, data: TimeEntryPayload) => Promise<void>
  update: (id: number, data: Partial<TimeEntryPayload>) => Promise<void>
  remove: (id: number) => Promise<void>
  fetchSources: (billingId: number) => Promise<TimeEntry[]>
}

const emptySummary: TimeSummary = { case_id: 0, unbilled_minutes: 0, unbilled_hours: 0, estimated_amount: 0 }

export const useTimeEntryStore = create<TimeEntryState>((set, get) => ({
  byCase: [],
  summary: emptySummary,
  sources: {},
  async fetchByCase(caseId) {
    const res: any = await listTimeEntriesByCase(caseId)
    set({ byCase: res.data })
  },
  async fetchSummary(caseId) {
    const res: any = await getTimeSummary(caseId)
    set({ summary: res.data })
  },
  async create(caseId, data) {
    await createTimeEntry(caseId, data)
    await get().fetchByCase(caseId)
    await get().fetchSummary(caseId)
  },
  async update(id, data) {
    await updateTimeEntry(id, data)
    const caseId = get().summary.case_id || get().byCase[0]?.case_id
    if (caseId) {
      await get().fetchByCase(caseId)
      await get().fetchSummary(caseId)
    }
  },
  async remove(id) {
    await deleteTimeEntry(id)
    const caseId = get().summary.case_id || get().byCase[0]?.case_id
    if (caseId) {
      await get().fetchByCase(caseId)
      await get().fetchSummary(caseId)
    }
  },
  async fetchSources(billingId) {
    if (get().sources[billingId]) return get().sources[billingId]
    const res: any = await listTimeEntriesByBilling(billingId)
    set((state) => ({ sources: { ...state.sources, [billingId]: res.data } }))
    return res.data as TimeEntry[]
  },
}))
