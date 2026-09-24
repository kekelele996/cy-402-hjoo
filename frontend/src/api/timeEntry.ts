import request from '@/utils/request'

export interface TimeEntryPayload {
  work_date: string
  duration_min: number
  description?: string
  hourly_rate?: number
}

export function listTimeEntriesByCase(caseId: number, status?: string) {
  return request.get(`/cases/${caseId}/time-entries`, { params: status ? { status } : {} })
}

export function createTimeEntry(caseId: number, data: TimeEntryPayload) {
  return request.post(`/cases/${caseId}/time-entries`, data)
}

export function updateTimeEntry(id: number, data: Partial<TimeEntryPayload>) {
  return request.put(`/time-entries/${id}`, data)
}

export function deleteTimeEntry(id: number) {
  return request.delete(`/time-entries/${id}`)
}

export function getTimeSummary(caseId: number) {
  return request.get(`/cases/${caseId}/time-summary`)
}

export function listTimeEntriesByBilling(billingId: number) {
  return request.get(`/time-entries/by-billing/${billingId}`)
}
