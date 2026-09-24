import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Card, Descriptions, Tabs, Button, Select, Space, message, Tag, Statistic, Row, Col, Popconfirm } from 'antd'
import { FileDoneOutlined } from '@ant-design/icons'
import { getCase, changeCaseStatus, assignLawyer } from '@/api/case'
import { getClient } from '@/api/client'
import { generateInvoiceFromTime } from '@/api/billing'
import DocumentList from '@/components/common/DocumentList'
import BillingCard from '@/components/common/BillingCard'
import StatusBadge from '@/components/common/StatusBadge'
import PermissionGuard from '@/components/common/PermissionGuard'
import TimelineItem from '@/components/common/TimelineItem'
import TimeEntryList from '@/components/common/TimeEntryList'
import { useDocumentStore } from '@/stores/documentStore'
import { useBillingStore } from '@/stores/billingStore'
import { useUserStore } from '@/stores/userStore'
import { useTimeEntryStore } from '@/stores/timeEntryStore'
import { CaseStatusOptions, CaseTypeOptions } from '@/constants/case'
import { formatAmount, formatAmountPlain } from '@/utils/amountFormatter'
import { formatDuration } from '@/utils/durationFormat'
import type { CaseItem, Client } from '@/types'

export default function CaseDetail() {
  const { id } = useParams()
  const caseId = Number(id)
  const [item, setItem] = useState<CaseItem | null>(null)
  const [client, setClient] = useState<Client | null>(null)
  const [status, setStatus] = useState('')
  const [lawyer, setLawyer] = useState<number>()
  const docStore = useDocumentStore()
  const billingStore = useBillingStore()
  const userStore = useUserStore()
  const timeStore = useTimeEntryStore()

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
    timeStore.fetchByCase(caseId)
    timeStore.fetchSummary(caseId)
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

  async function onGenerateInvoice() {
    const res: any = await generateInvoiceFromTime(caseId)
    message.success(`收费单 ${res.data.bill_no} 已生成，金额 ${formatAmount(res.data.amount)}`)
    load()
  }

  if (!item) return null

  return (
    <Card>
      <Space style={{ marginBottom: 16 }}>
        <h2 style={{ margin: 0 }}>{item.case_no} {item.title}</h2>
        <StatusBadge status={item.status} />
        <Tag>{CaseTypeOptions.find((o) => o.value === item.case_type)?.label || item.case_type}</Tag>
      </Space>

      {/* 待收费工时汇总：待收费时长 + 预计金额，可一键汇总生成收费单 */}
      <Card size="small" style={{ marginBottom: 16 }}>
        <Row gutter={16} align="middle">
          <Col span={6}>
            <Statistic title="待收费时长" value={formatDuration(timeStore.summary.unbilled_minutes)} />
          </Col>
          <Col span={6}>
            <Statistic
              title="预计金额"
              value={formatAmountPlain(timeStore.summary.estimated_amount)}
              prefix="¥"
              valueStyle={{ color: '#cf1322' }}
            />
          </Col>
          <Col span={12} style={{ textAlign: 'right' }}>
            <PermissionGuard roles={['admin', 'lawyer']}>
              <Popconfirm
                title="将该案件全部待收费工时汇总生成一张收费单？"
                disabled={timeStore.summary.unbilled_minutes === 0}
                onConfirm={onGenerateInvoice}
              >
                <Button
                  type="primary"
                  icon={<FileDoneOutlined />}
                  disabled={timeStore.summary.unbilled_minutes === 0}
                >
                  汇总待收费工时生成收费单
                </Button>
              </Popconfirm>
            </PermissionGuard>
          </Col>
        </Row>
      </Card>

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
            key: 'time',
            label: `工时（${timeStore.byCase.length}）`,
            children: <TimeEntryList caseId={caseId} />,
          },
          {
            key: 'docs',
            label: '文档',
            children: <DocumentList documents={docStore.byCase} />,
          },
          {
            key: 'billings',
            label: `账单（${billingStore.byCase.length}）`,
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
    </Card>
  )
}
