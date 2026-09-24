import { Card, Descriptions, Tag } from 'antd'
import StatusBadge from './StatusBadge'
import { BillingTypeText, BillingSourceText, BillingSource } from '@/constants/billing'
import { formatAmount } from '@/utils/amountFormatter'
import type { Billing } from '@/types'

export default function BillingCard({ item }: { item: Billing }) {
  const isTimeInvoice = item.source === BillingSource.TIME_ENTRIES
  return (
    <Card
      size="small"
      style={{ marginBottom: 8 }}
      title={
        <span>
          {item.bill_no} {BillingTypeText[item.billing_type] || item.billing_type}{' '}
          <Tag color={isTimeInvoice ? 'geekblue' : 'default'} style={{ marginLeft: 4 }}>
            {isTimeInvoice ? BillingSourceText[BillingSource.TIME_ENTRIES] : BillingSourceText[BillingSource.MANUAL]}
          </Tag>
        </span>
      }
      extra={<StatusBadge status={item.status} kind="billing" />}
    >
      <Descriptions column={1} size="small">
        <Descriptions.Item label="金额">{formatAmount(item.amount)}</Descriptions.Item>
        <Descriptions.Item label="案件ID">{item.case_id}</Descriptions.Item>
        <Descriptions.Item label="客户ID">{item.client_id}</Descriptions.Item>
        <Descriptions.Item label="发票信息">{item.invoice_info || '-'}</Descriptions.Item>
      </Descriptions>
    </Card>
  )
}
