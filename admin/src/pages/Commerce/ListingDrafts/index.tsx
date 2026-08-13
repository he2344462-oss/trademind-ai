import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Space, Tag, message } from 'antd';
import { useRef } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { fetchListingDrafts, generateListingContent, type ListingDraft } from '@/services/productFlow';
import { money, statusTag } from '../shared';

const stateText: Record<string, string> = { draft: '待生成', content_generated: '已生成', needs_review: '待审核', approved: '已审核', ready_to_publish: '可发布', published_manual: '已人工发布' };

export default function ListingDraftsPage() {
  const actionRef = useRef<ActionType>();
  const generate = async (row: ListingDraft, mode: 'template_only' | 'ai_generate') => {
    await generateListingContent(row.id, mode); message.success(mode === 'ai_generate' ? 'AI 内容已生成；不可用时已自动使用模板' : '模板内容已生成'); actionRef.current?.reload();
  };
  const columns: ProColumns<ListingDraft>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => r.title, ellipsis: true },
    { title: '平台', dataIndex: 'platform', valueType: 'select', valueEnum: { xianyu: { text: '闲鱼' }, taobao: { text: '淘宝' } }, width: 90 },
    { title: '内容状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(stateText).map(([k, text]) => [k, { text }])), render: (_, r) => <Tag>{stateText[r.publishStatus] || r.publishStatus}</Tag>, width: 110 },
    { title: '售价', search: false, render: (_, r) => money(r.salePrice), width: 100 },
    { title: '预计利润', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '图片', search: false, render: (_, r) => `${r.images?.length || 0} 张`, width: 80 },
    { title: 'SKU', search: false, render: (_, r) => `${r.platformSkuData?.length || 0} 个`, width: 80 },
    { title: '发布状态', search: false, render: (_, r) => statusTag(r.publishStatus), width: 110 },
    { title: '操作', valueType: 'option', width: 270, render: (_, r) => <Space wrap>
      <a onClick={() => generate(r, 'template_only')}>模板生成</a><a onClick={() => generate(r, 'ai_generate')}>AI 生成</a><a onClick={() => history.push(`/listing-drafts/${r.id}/review`)}>审核与发布包</a>
    </Space> },
  ];
  return <TmPageContainer title="铺货工作台" subTitle="生成平台差异化内容、人工审核并下载发布资料包；系统不会自动操作闲鱼或淘宝">
    <Space style={{ marginBottom: 16 }}><Button onClick={() => actionRef.current?.reload()}>刷新</Button><Tag color="blue">预计利润不等于实际利润</Tag></Space>
    <ProTable<ListingDraft> rowKey="id" actionRef={actionRef} columns={columns} scroll={{ x: 1080 }} request={async (p) => { const r = await fetchListingDrafts(p); return { data: r.list, total: r.pagination.total, success: true }; }} />
  </TmPageContainer>;
}
