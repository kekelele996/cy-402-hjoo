import { useEffect, useState } from 'react'
import { Button, Card, Form, Input, InputNumber, message, Avatar, Space, Alert } from 'antd'
import { UserOutlined } from '@ant-design/icons'
import AvatarUploader from '@/components/common/AvatarUploader'
import { useUserStore } from '@/stores/userStore'

export default function Profile() {
  const store = useUserStore()
  const [form] = Form.useForm()
  const [avatar, setAvatar] = useState('')

  useEffect(() => {
    store.fetchMe().then((me) => {
      form.setFieldsValue({
        real_name: me.real_name,
        email: me.email,
        phone: me.phone,
        license_no: me.license_no,
        hourly_rate: Number(me.hourly_rate) || 0,
      })
      setAvatar(me.avatar || '')
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  async function onSave() {
    const values = await form.validateFields()
    await store.updateMe({ ...values, avatar })
    message.success('保存成功')
  }

  return (
    <Card style={{ maxWidth: 520 }}>
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Avatar size={72} icon={<UserOutlined />} src={avatar} />
        <AvatarUploader onUploaded={(u) => setAvatar(u)} />
        <Form form={form} layout="vertical">
          <Form.Item name="real_name" label="姓名"><Input /></Form.Item>
          <Form.Item name="license_no" label="执业证号"><Input /></Form.Item>
          <Form.Item name="hourly_rate" label="当前费率（元/小时）">
            <InputNumber style={{ width: '100%' }} min={0} precision={2} />
          </Form.Item>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 12 }}
            message="费率调整仅对之后新登记的工时生效，已登记工时按其当时费率快照结算，不会被改变。"
          />
          <Form.Item name="email" label="邮箱"><Input /></Form.Item>
          <Form.Item name="phone" label="手机"><Input /></Form.Item>
          <Button type="primary" onClick={onSave}>保存</Button>
        </Form>
      </Space>
    </Card>
  )
}
