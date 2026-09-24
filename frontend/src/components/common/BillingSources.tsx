import { useEffect, useState } from 'react'
import { Empty, Spin, Table, Tag } from 'antd'
import { listTimeEntriesByBilling } from '@/api/timeEntry'
import { formatAmount } from '@/utils/amountFormatter'
import { formatDuration } from '@/utils/durationFormat'
import { formatDate } from '@/utils/dateFormat'
import type { TimeEntry } from '@/types'

// BillingSources 收费单展开行：列出构成该收费单金额的全部工时来源。
export default function BillingSources({ billingId }: { billingId: number }) {
  const [loading, setLoading] = useState(true)
  const [list, setList] = useState<TimeEntry[]>([])

  useEffect(() => {
    let alive = true
    setLoading(true)
    listTimeEntriesByBilling(billingId)
      .then((res: any) => {
        if (alive) setList(res.data)
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [billingId])

  if (loading) return <Spin size="small" />
  if (list.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="该账单为手工录入，无工时来源" />
  }

  return (
    <Table<TimeEntry>
      rowKey="id"
      size="small"
      dataSource={list}
      pagination={false}
      columns={[
        { title: '工作日期', dataIndex: 'work_date', width: 110, render: (v: string) => formatDate(v) },
        { title: '时长', dataIndex: 'duration_min', width: 120, render: (v: number) => formatDuration(v) },
        {
          title: '当时费率',
          dataIndex: 'hourly_rate',
          width: 120,
          render: (v: number | string) => `${formatAmount(v)}/时`,
        },
        {
          title: '小计',
          width: 120,
          render: (_: unknown, row: TimeEntry) =>
            formatAmount((Number(row.duration_min) / 60) * Number(row.hourly_rate)),
        },
        { title: '工作内容', dataIndex: 'description' },
        {
          title: '结算状态',
          dataIndex: 'billing_id',
          width: 100,
          render: (v: number | null) =>
            v === null ? <Tag color="orange">已退回待收费</Tag> : <Tag color="blue">收费单 #{v}</Tag>,
        },
      ]}
    />
  )
}
