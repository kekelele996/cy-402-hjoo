import { Button, Popconfirm, Table, Tag } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { formatAmount } from '@/utils/amountFormatter'
import { formatDate } from '@/utils/dateFormat'
import type { TimeEntry } from '@/types'

interface Props {
  entries: TimeEntry[]
  loading?: boolean
  showStatus?: boolean
  onDelete?: (id: number) => void
}

// 工时表格：案件详情工时页与账单收费来源展开共用。
export default function TimeEntryTable({ entries, loading, showStatus = true, onDelete }: Props) {
  const columns: ColumnsType<TimeEntry> = [
    { title: '日期', dataIndex: 'work_date', width: 110, render: (v) => formatDate(v) },
    { title: '工作内容', dataIndex: 'description', ellipsis: true },
    { title: '时长(小时)', dataIndex: 'hours', width: 100, render: (v) => Number(v) },
    { title: '当时费率', dataIndex: 'hourly_rate', width: 110, render: (v) => `${formatAmount(v)}/时` },
    { title: '金额', key: 'amount', width: 120, render: (_, r) => formatAmount(Number(r.hours) * Number(r.hourly_rate)) },
  ]
  if (showStatus) {
    columns.push({
      title: '状态', dataIndex: 'billing_id', width: 110,
      render: (v: number | null) => (v ? <Tag color="blue">已结算 #{v}</Tag> : <Tag color="orange">待结算</Tag>),
    })
  }
  if (onDelete) {
    columns.push({
      title: '操作', key: 'action', width: 80,
      render: (_, r) =>
        r.billing_id ? null : (
          <Popconfirm title="删除该工时记录？" onConfirm={() => onDelete(r.id)}>
            <Button size="small" danger>删除</Button>
          </Popconfirm>
        ),
    })
  }
  return <Table<TimeEntry> rowKey="id" size="small" loading={loading} dataSource={entries} columns={columns} pagination={false} />
}
