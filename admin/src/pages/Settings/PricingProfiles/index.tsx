import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormRadio, ProFormText } from '@ant-design/pro-components';
import { Alert, Button, Popconfirm, Tag, message } from 'antd';
import { useRef, useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { copyPricingProfile, createPricingProfile, fetchPricingProfiles, setDefaultPricingProfile, updatePricingProfile, type PricingProfile } from '@/services/productFlow';

const profileBody = (row: PricingProfile, enabled = row.enabled) => ({ ...row, platformFeeFixed: (row.platformFeeFixed / 100).toFixed(2), paymentFeeFixed: (row.paymentFeeFixed / 100).toFixed(2), otherFixed: (row.otherFixed / 100).toFixed(2), enabled });

export default function PricingProfilesPage() {
  const actionRef = useRef<ActionType>();
  const [open, setOpen] = useState(false);
  const columns: ProColumns<PricingProfile>[] = [
    { title: '名称', dataIndex: 'name' }, { title: '平台', dataIndex: 'platform' }, { title: '币种', dataIndex: 'currency', search: false },
    { title: '平台费率', render: (_, row) => `${(row.platformFeeBps / 100).toFixed(2)}%`, search: false },
    { title: '支付费率', render: (_, row) => `${(row.paymentFeeBps / 100).toFixed(2)}%`, search: false },
    { title: '售后预留', render: (_, row) => `${(row.returnReserveBps / 100).toFixed(2)}%`, search: false },
    { title: '版本', dataIndex: 'version', search: false },
    { title: '状态', search: false, render: (_, row) => <>{row.isDefault ? <Tag color="blue">默认</Tag> : null}<Tag color={row.enabled ? 'green' : undefined}>{row.enabled ? '启用' : '停用'}</Tag></> },
    { title: '操作', valueType: 'option', render: (_, row) => [!row.isDefault && row.enabled ? <a key="default" onClick={async () => { await setDefaultPricingProfile(row.id); message.success('已设为默认费用模型'); actionRef.current?.reload(); }}>设为默认</a> : null, <a key="copy" onClick={async () => { await copyPricingProfile(row.id); message.success('已复制费用模型'); actionRef.current?.reload(); }}>复制</a>, <Popconfirm key="enabled" title={row.enabled ? '停用后新分析不可再选择，历史快照不会变化。确定停用？' : '确定重新启用？'} onConfirm={async () => { await updatePricingProfile(row.id, profileBody(row, !row.enabled)); message.success(row.enabled ? '已停用' : '已启用'); actionRef.current?.reload(); }}><a>{row.enabled ? '停用' : '启用'}</a></Popconfirm>].filter(Boolean) },
  ];
  return <TmPageContainer title="平台费用模型" subTitle="费用均为用户配置的估算模型，不代表平台官方实时收费标准">
    <Alert style={{ marginBottom: 16 }} type="warning" showIcon message="历史可追溯" description="费率使用基点保存（100 = 1%），固定金额使用人民币分保存。每次修改都会生成新版本，历史分析和铺货草稿仍保留当时快照。" />
    <ProTable<PricingProfile> rowKey="id" actionRef={actionRef} columns={columns} search={false} request={async () => { const result = await fetchPricingProfiles(); return { data: result.list, total: result.list.length, success: true }; }} toolBarRender={() => [<Button key="create" type="primary" onClick={() => setOpen(true)}>新建费用模型</Button>]} />
    <ModalForm title="新建费用模型" open={open} onOpenChange={setOpen} initialValues={{ currency: 'CNY', platform: 'xianyu', enabled: true, isDefault: false, platformFeeBps: 0, paymentFeeBps: 0, returnReserveBps: 0, otherBps: 0, platformFeeFixed: 0, paymentFeeFixed: 0, otherFixed: 0 }} onFinish={async (values) => { const body = { ...values, platformFeeFixed: Number(values.platformFeeFixed || 0).toFixed(2), paymentFeeFixed: Number(values.paymentFeeFixed || 0).toFixed(2), otherFixed: Number(values.otherFixed || 0).toFixed(2) }; await createPricingProfile(body); message.success('已创建费用模型'); actionRef.current?.reload(); return true; }}>
      <ProFormText name="name" label="名称" rules={[{ required: true }]} /><ProFormRadio.Group name="platform" label="平台" options={[{ label: '闲鱼', value: 'xianyu' }, { label: '淘宝', value: 'taobao' }, { label: '自定义', value: 'custom' }]} /><ProFormText name="currency" label="币种" /><ProFormDigit name="platformFeeBps" label="平台费率（基点）" min={0} max={9999} /><ProFormDigit name="platformFeeFixed" label="平台固定费用（元）" min={0} fieldProps={{ precision: 2 }} /><ProFormDigit name="paymentFeeBps" label="支付费率（基点）" min={0} max={9999} /><ProFormDigit name="paymentFeeFixed" label="支付固定费用（元）" min={0} fieldProps={{ precision: 2 }} /><ProFormDigit name="returnReserveBps" label="售后预留率（基点）" min={0} max={9999} /><ProFormDigit name="otherBps" label="其他费率（基点）" min={0} max={9999} /><ProFormDigit name="otherFixed" label="其他固定费用（元）" min={0} fieldProps={{ precision: 2 }} /><ProFormRadio.Group name="isDefault" label="设为平台默认" options={[{ label: '否', value: false }, { label: '是', value: true }]} />
    </ModalForm>
  </TmPageContainer>;
}
