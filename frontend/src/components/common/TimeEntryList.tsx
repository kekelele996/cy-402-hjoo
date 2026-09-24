import { useState } from 'react'
import { Button, DatePicker, Form, InputNumber, Input, Modal, Popconfirm, Space, Table, Tag, message } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import { useTimeEntryStore } from '@/stores/timeEntryStore'
import { useUserStore } from '@/stores/userStore'
import { formatAmount } from '@/utils/amountFormatter'
import { formatDuration } from '@/utils/durationFormat'
import { formatDate } from '@/utils/dateFormat'
import PermissionGuard from '@/components/common/PermissionGuard'
import type { TimeEntry } from '@/types'

interface Props {
  caseId: number
  // unbilled：仅待结算工时；billed：仅已入单工时；不传则全部
  filter?: 'unbilled' | 'billed'
  compact?: boolean
}

export default function TimeEntryList({ caseId, filter, compact }: Props) {
  const store = useTimeEntryStore()
  const userStore = useUserStore()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<TimeEntry | null>(null)
  const [form] = Form.useForm()

  const lawyerName = (id: number) =>
    userStore.lawyers.find((l) => l.id === id)?.real_name || `律师#${id}`

  const dataSource = store.byCase.filter((t) =>
    filter === 'unbilled' ? t.billing_id === null : filter === 'billed' ? t.billing_id !== null : true,
  )

  function openCreate() {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      work_date: dayjs(),
      duration_min: 60,
      hourly_rate: Number(userStore.me?.hourly_rate) || undefined,
    })
    setOpen(true)
  }

  function openEdit(row: TimeEntry) {
    setEditing(row)
    form.setFieldsValue({
      work_date: dayjs(row.work_date),
      duration_min: row.duration_min,
      description: row.description,
      hourly_rate: Number(row.hourly_rate),
    })
    setOpen(true)
  }

  async function onSubmit() {
    const values = await form.validateFields()
    const payload = {
      work_date: (values.work_date as Dayjs).format('YYYY-MM-DD'),
      duration_min: values.duration_min,
      description: values.description || '',
      hourly_rate: values.hourly_rate,
    }
    if (editing) {
      await store.update(editing.id, payload)
      message.success('工时已更新')
    } else {
      await store.create(caseId, payload)
      message.success('工时登记成功')
    }
    setOpen(false)
  }

  const columns = [
    { title: '日期', dataIndex: 'work_date', width: 110, render: (v: string) => formatDate(v) },
    { title: '承办律师', dataIndex: 'lawyer_id', width: 100, render: (v: number) => lawyerName(v) },
    { title: '时长', dataIndex: 'duration_min', width: 110, render: (v: number) => formatDuration(v) },
    {
      title: '当时费率',
      dataIndex: 'hourly_rate',
      width: 110,
      render: (v: number | string) => `${formatAmount(v)}/时`,
    },
    {
      title: '金额',
      width: 110,
      render: (_: unknown, row: TimeEntry) => formatAmount((Number(row.duration_min) / 60) * Number(row.hourly_rate)),
    },
    { title: '工作内容', dataIndex: 'description', ellipsis: true },
    {
      title: '状态',
      dataIndex: 'billing_id',
      width: 100,
      render: (v: number | null) =>
        v === null ? <Tag color="orange">待收费</Tag> : <Tag color="default">已入单 #{v}</Tag>,
    },
    {
      title: '操作',
      width: 130,
      render: (_: unknown, row: TimeEntry) =>
        row.billing_id === null ? (
          <Space size={4}>
            <Button size="small" type="link" onClick={() => openEdit(row)}>编辑</Button>
            <Popconfirm
              title="确定删除该工时？"
              onConfirm={async () => {
                await store.remove(row.id)
                message.success('已删除')
              }}
            >
              <Button size="small" type="link" danger>删除</Button>
            </Popconfirm>
          </Space>
        ) : null,
    },
  ]

  return (
    <div>
      <Space style={{ marginBottom: 12 }}>
        <PermissionGuard roles={['admin', 'lawyer']}>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>登记工时</Button>
        </PermissionGuard>
        <span style={{ color: '#999' }}>
          共 {dataSource.length} 条，合计 {formatDuration(dataSource.reduce((s, t) => s + t.duration_min, 0))}
        </span>
      </Space>
      <Table<TimeEntry>
        rowKey="id"
        size={compact ? 'small' : 'middle'}
        dataSource={dataSource}
        pagination={false}
        columns={columns}
      />
      <Modal
        title={editing ? '编辑工时' : '登记工时'}
        open={open}
        onOk={onSubmit}
        onCancel={() => setOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="work_date" label="工作日期" rules={[{ required: true }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="duration_min" label="时长（分钟）" rules={[{ required: true }]}>
            <InputNumber style={{ width: '100%' }} min={1} max={1440} step={15} />
          </Form.Item>
          <Form.Item name="hourly_rate" label="当时费率（元/小时，留空取本人当前费率）">
            <InputNumber style={{ width: '100%' }} min={0} precision={2} />
          </Form.Item>
          <Form.Item name="description" label="工作内容">
            <Input.TextArea rows={3} maxLength={2000} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
