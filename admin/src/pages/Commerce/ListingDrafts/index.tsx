import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { Popconfirm, message } from 'antd';
import { useRef, useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { deleteListingDraft, fetchListingDrafts, updateListingDraft, type ListingDraft } from '@/services/productFlow';
import { money, percent, statusTag } from '../shared';

export default function ListingDraftsPage() {
  const actionRef = useRef<ActionType>(); const [editing, setEditing] = useState<ListingDraft>();
  const columns: ProColumns<ListingDraft>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => r.title, ellipsis: true },
    { title: '平台', dataIndex: 'platform', valueType: 'select', valueEnum: { xianyu: { text: '闲鱼' }, taobao: { text: '淘宝' } }, width: 100 },
    { title: '售价', dataIndex: 'salePrice', search: false, render: (_, r) => money(r.salePrice), width: 110 },
    { title: '预计利润', dataIndex: 'estimatedProfit', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '利润率', dataIndex: 'estimatedMargin', search: false, render: (_, r) => percent(r.estimatedMargin), width: 90 },
    { title: '费用配置', search: false, render: (_, r) => r.pricingSnapshot?.profile.configured ? r.pricingSnapshot.profile.code : '未配置（按 0 计）', width: 150 },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { draft: { text: '草稿' }, ready: { text: '就绪' } }, render: (_, r) => statusTag(r.publishStatus), width: 100 },
    { title: '操作', valueType: 'option', render: (_, r) => [<a key="edit" onClick={() => setEditing(r)}>编辑</a>, <Popconfirm key="delete" title="仅删除这个未发布草稿，确定继续？" onConfirm={async () => { await deleteListingDraft(r.id); message.success('草稿已删除'); actionRef.current?.reload(); }}><a>删除</a></Popconfirm>] },
  ];
  return <TmPageContainer title="铺货中心" subTitle="各平台可采用不同费用配置；本阶段只编辑草稿，不调用真实发布接口">
    <ProTable<ListingDraft> rowKey="id" actionRef={actionRef} columns={columns} request={async (p) => { const r = await fetchListingDrafts(p); return { data: r.list, total: r.pagination.total, success: true }; }} />
    <ModalForm title="编辑铺货草稿" open={Boolean(editing)} initialValues={editing ? { ...editing, imageUrls: editing.images?.join('\n') } : undefined} onOpenChange={(v) => !v && setEditing(undefined)} onFinish={async (v) => { if (!editing) return false; const images = String(v.imageUrls || '').split(/\r?\n/).map((item) => item.trim()).filter(Boolean); const { imageUrls: _, ...payload } = v; await updateListingDraft(editing.id, { ...payload, images }); message.success('草稿已更新'); actionRef.current?.reload(); return true; }}>
      <ProFormText name="title" label="标题" rules={[{ required: true }]} /><ProFormTextArea name="description" label="描述" /><ProFormTextArea name="imageUrls" label="图片 URL" tooltip="每行一个 URL" /><ProFormDigit name="salePrice" label="售价" min={0} fieldProps={{ precision: 2 }} /><ProFormText name="platformCategory" label="平台类目" />
    </ModalForm>
  </TmPageContainer>;
}
