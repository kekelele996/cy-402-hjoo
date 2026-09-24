import request from '@/utils/request'

export function listTimeEntries(caseId: number, unbilledOnly = false) {
  return request.get(`/cases/${caseId}/time-entries`, { params: unbilledOnly ? { unbilled: 'true' } : {} })
}

export function getUnbilledSummary(caseId: number) {
  return request.get(`/cases/${caseId}/time-entries/unbilled-summary`)
}

export function createTimeEntry(caseId: number, data: { work_date: string; hours: number; description: string; hourly_rate: number }) {
  return request.post(`/cases/${caseId}/time-entries`, data)
}

export function deleteTimeEntry(id: number) {
  return request.delete(`/time-entries/${id}`)
}

export function generateBillingFromTimeEntries(data: { case_id: number; entry_ids?: number[]; invoice_info?: string }) {
  return request.post('/billings/from-time-entries', data)
}

export function listBillingTimeEntries(billingId: number) {
  return request.get(`/billings/${billingId}/time-entries`)
}
