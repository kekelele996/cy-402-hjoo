export interface User {
  id: number
  username: string
  real_name: string
  role: string
  license_no: string
  email: string
  phone: string
  avatar: string
  hourly_rate: number | string
  created_at: string
}

export interface Client {
  id: number
  name: string
  id_number: string
  contact: string
  address: string
  remark: string
  created_at: string
}

export interface CaseItem {
  id: number
  case_no: string
  title: string
  case_type: string
  status: string
  accept_date: string | null
  close_date: string | null
  summary: string
  client_id: number
  lead_lawyer_id: number
  co_lawyer_ids: number[]
  created_at: string
}

export interface DocumentItem {
  id: number
  title: string
  file_type: string
  file_url: string
  upload_time: string
  case_id: number
  uploader_id: number
  created_at: string
}

export interface Billing {
  id: number
  bill_no: string
  billing_type: string
  amount: number | string
  status: string
  case_id: number
  client_id: number
  invoice_info: string
  source: string
  created_at: string
}

// TimeEntry 工时记录，hourly_rate 为登记时的费率快照。
export interface TimeEntry {
  id: number
  case_id: number
  lawyer_id: number
  work_date: string
  duration_min: number
  description: string
  hourly_rate: number | string
  billing_id: number | null
  created_at: string
}

// TimeSummary 案件待收费工时汇总。
export interface TimeSummary {
  case_id: number
  unbilled_minutes: number
  unbilled_hours: number
  estimated_amount: number
}

export interface AuditLog {
  id: number
  operator_id: number
  operator_name: string
  action: string
  entity_type: string
  entity_id: string
  detail: string
  ip: string
  created_at: string
}

export interface PageResult<T> {
  list: T[]
  total: number
  page: number
  page_size: number
}
