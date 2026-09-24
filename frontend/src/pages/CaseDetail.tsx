import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, Card, DatePicker, Descriptions, Form, Input, InputNumber, Modal, Popconfirm, Tabs, Button, Select, Space, message, Tag } from 'antd'
import { getCase, changeCaseStatus, assignLawyer } from '@/api/case'
import { getClient } from '@/api/client'
import { createTimeEntry, deleteTimeEntry, generateBillingFromTimeEntries } from '@/api/timeEntry'
import DocumentList from '@/components/common/DocumentList'
import BillingCard from '@/components/common/BillingCard'
import StatusBadge from '@/components/common/StatusBadge'
import PermissionGuard from '@/components/common/PermissionGuard'
import TimelineItem from '@/components/common/TimelineItem'
import TimeEntryTable from '@/components/common/TimeEntryTable'
import { useDocumentStore } from '@/stores/documentStore'
import { useBillingStore } from '@/stores/billingStore'
import { useTimeEntryStore } from '@/stores/timeEntryStore'
import { useUserStore } from '@/stores/userStore'
import { CaseStatusOptions, CaseTypeOptions } from '@/constants/case'
import { formatAmount } from '@/utils/amountFormatter'
import type { CaseItem, Client } from '@/types'

export default function CaseDetail() {
  const { id } = useParams()
  const caseId = Number(id)
  const [item, setItem] = useState<CaseItem | null>(null)
  const [client, setClient] = useState<Client | null>(null)
  const [status, setStatus] = useState('')
  const [lawyer, setLawyer] = useState<number>()
  const [entryOpen, setEntryOpen] = useState(false)
  const [entryForm] = Form.useForm()
  const docStore = useDocumentStore()
  const billingStore = useBillingStore()
  const timeEntryStore = useTimeEntryStore()
  const userStore = useUserStore()

  useEffect(() => {
    userStore.fetchLawyers()
    load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caseId])

  async function load() {
    const res: any = await getCase(caseId)
    setItem(res.data)
    setStatus(res.data.status)
    if (res.data.client_id) {
      const cr: any = await getClient(res.data.client_id)
      setClient(cr.data.client)
    }
    docStore.fetchByCase(caseId)
    billingStore.fetchByCase(caseId)
    refreshTimeEntries()
  }

  function refreshTimeEntries() {
    timeEntryStore.fetchByCase(caseId)
    timeEntryStore.fetchUnbilledSummary(caseId)
  }

  async function onStatusChange() {
    await changeCaseStatus(caseId, status)
    message.success('状态已更新')
    load()
  }

  async function onAssign() {
    if (!lawyer) return
    await assignLawyer(caseId, { lead_lawyer_id: lawyer })
    message.success('律师已分配')
    load()
  }

  async function onCreateEntry() {
    const values = await entryForm.validateFields()
    await createTimeEntry(caseId, {
      work_date: values.work_date.format('YYYY-MM-DD'),
      hours: values.hours,
      hourly_rate: values.hourly_rate ?? 0,
      description: values.description,
    })
    message.success('工时记录成功')
    setEntryOpen(false)
    entryForm.resetFields()
    refreshTimeEntries()
  }

  async function onDeleteEntry(entryId: number) {
    await deleteTimeEntry(entryId)
    message.success('工时已删除')
    refreshTimeEntries()
  }

  async function onGenerateBilling() {
    await generateBillingFromTimeEntries({ case_id: caseId })
    message.success('收费单已生成')
    refreshTimeEntries()
    billingStore.fetchByCase(caseId)
  }

  if (!item) return null

  const unbilled = timeEntryStore.unbilled

  return (
    <Card>
      <Space style={{ marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>{item.case_no} {item.title}</h2>
        <StatusBadge status={item.status} />
        <Tag>{CaseTypeOptions.find((o) => o.value === item.case_type)?.label || item.case_type}</Tag>
      </Space>
      <Tabs
        items={[
          {
            key: 'info',
            label: '案件信息',
            children: (
              <>
                <Descriptions bordered column={2} size="small">
                  <Descriptions.Item label="案号">{item.case_no}</Descriptions.Item>
                  <Descriptions.Item label="状态">{item.status}</Descriptions.Item>
                  <Descriptions.Item label="类型">{item.case_type}</Descriptions.Item>
                  <Descriptions.Item label="主办律师">#{item.lead_lawyer_id}</Descriptions.Item>
                  <Descriptions.Item label="受理日期">{item.accept_date || '-'}</Descriptions.Item>
                  <Descriptions.Item label="结案日期">{item.close_date || '-'}</Descriptions.Item>
                  <Descriptions.Item label="摘要" span={2}>{item.summary || '-'}</Descriptions.Item>
                </Descriptions>
                <PermissionGuard roles={['admin', 'lawyer']}>
                  <Space style={{ marginTop: 16 }}>
                    <Select value={status} style={{ width: 150 }} options={CaseStatusOptions} onChange={setStatus} />
                    <Button type="primary" onClick={onStatusChange}>更新状态</Button>
                  </Space>
                  <Space style={{ marginTop: 8 }}>
                    <Select
                      placeholder="分配主办律师"
                      style={{ width: 180 }}
                      value={lawyer}
                      onChange={setLawyer}
                      options={userStore.lawyers.map((l) => ({ label: l.real_name || l.username, value: l.id }))}
                    />
                    <Button onClick={onAssign}>分配</Button>
                  </Space>
                </PermissionGuard>
              </>
            ),
          },
          {
            key: 'client',
            label: '关联客户',
            children: client ? (
              <Descriptions bordered column={1} size="small">
                <Descriptions.Item label="姓名">{client.name}</Descriptions.Item>
                <Descriptions.Item label="证件号">{client.id_number}</Descriptions.Item>
                <Descriptions.Item label="联系方式">{client.contact}</Descriptions.Item>
                <Descriptions.Item label="地址">{client.address}</Descriptions.Item>
              </Descriptions>
            ) : null,
          },
          {
            key: 'docs',
            label: '文档',
            children: <DocumentList documents={docStore.byCase} />,
          },
          {
            key: 'time_entries',
            label: '工时',
            children: (
              <>
                <Alert
                  style={{ marginBottom: 16 }}
                  type={unbilled.count > 0 ? 'warning' : 'info'}
                  message={`待收费时长 ${Number(unbilled.hours)} 小时，预计金额 ${formatAmount(unbilled.amount)}（共 ${unbilled.count} 项未结算工时）`}
                />
                <PermissionGuard roles={['admin', 'lawyer']}>
                  <Space style={{ marginBottom: 16 }}>
                    <Button type="primary" onClick={() => setEntryOpen(true)}>登记工时</Button>
                    <Popconfirm
                      title={`将 ${unbilled.count} 项待结算工时（${formatAmount(unbilled.amount)}）生成收费单？`}
                      onConfirm={onGenerateBilling}
                      disabled={unbilled.count === 0}
                    >
                      <Button disabled={unbilled.count === 0}>生成收费单</Button>
                    </Popconfirm>
                  </Space>
                </PermissionGuard>
                <TimeEntryTable entries={timeEntryStore.byCase} onDelete={onDeleteEntry} />
              </>
            ),
          },
          {
            key: 'billings',
            label: '账单',
            children: billingStore.byCase.map((b) => <BillingCard key={b.id} item={b} />),
          },
          {
            key: 'timeline',
            label: '时间线',
            children: (
              <TimelineItem
                items={[
                  { id: 1, time: item.created_at, text: `案件创建（${item.case_no}）` },
                  { id: 2, time: item.accept_date || item.created_at, text: '案件受理' },
                  { id: 3, time: item.close_date || item.created_at, text: item.close_date ? '案件结案' : '案件进行中' },
                ]}
              />
            ),
          },
        ]}
      />
      <Modal title="登记工时" open={entryOpen} onOk={onCreateEntry} onCancel={() => setEntryOpen(false)} destroyOnClose>
        <Form form={entryForm} layout="vertical">
          <Form.Item name="work_date" label="工作日期" rules={[{ required: true, message: '请选择工作日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="hours" label="时长（小时）" rules={[{ required: true, message: '请输入时长' }]}>
            <InputNumber style={{ width: '100%' }} min={0.5} max={24} step={0.5} precision={2} />
          </Form.Item>
          <Form.Item name="hourly_rate" label="当时费率（元/小时）" rules={[{ required: true, message: '请输入费率' }]}>
            <InputNumber style={{ width: '100%' }} min={0} precision={2} />
          </Form.Item>
          <Form.Item name="description" label="工作内容" rules={[{ required: true, message: '请输入工作内容' }]}>
            <Input.TextArea rows={3} maxLength={500} showCount />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
