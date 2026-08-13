import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { history } from '@umijs/max';
import { Button, Card, Progress, Space, Table, Tag, message } from 'antd';
import { useEffect, useRef, useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { controlListingOperationBatch, createListingOperationBatch, fetchListingDrafts, fetchListingOperationBatches, type ListingDraft, type ListingOperationBatch } from '@/services/productFlow';
import { money, statusTag } from '../shared';

const stateText: Record<string, string> = { draft: '待生成', content_generated: '已生成', needs_review: '待审核', approved: '已审核', ready_to_publish: '可发布', published_manual: '已人工发布' };

export default function ListingDraftsPage() {
  const actionRef = useRef<ActionType>(); const [selected, setSelected] = useState<string[]>([]); const [batches, setBatches] = useState<ListingOperationBatch[]>([]);
  const loadBatches = async () => setBatches((await fetchListingOperationBatches()).list);
  useEffect(() => { void loadBatches(); const timer = window.setInterval(() => void loadBatches(), 3000); return () => window.clearInterval(timer); }, []);
  const start = async (operation: ListingOperationBatch['operation'], mode = 'template_only') => { if (!selected.length) { message.warning('请先选择 Listing'); return; } await createListingOperationBatch(operation, selected, mode); message.success('后台任务已创建'); setSelected([]); await loadBatches(); };
  const columns: ProColumns<ListingDraft>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => r.title, ellipsis: true },
    { title: '平台', dataIndex: 'platform', valueType: 'select', valueEnum: { xianyu: { text: '闲鱼' }, taobao: { text: '淘宝' } }, width: 90 },
    { title: '内容状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(stateText).map(([k, text]) => [k, { text }])), render: (_, r) => <Tag>{stateText[r.publishStatus] || r.publishStatus}</Tag>, width: 110 },
    { title: '售价', search: false, render: (_, r) => money(r.salePrice), width: 100 }, { title: '预计利润', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '发布状态', search: false, render: (_, r) => statusTag(r.publishStatus), width: 110 },
    { title: '操作', valueType: 'option', width: 150, render: (_, r) => <a onClick={() => history.push(`/listing-drafts/${r.id}/review`)}>审核与发布包</a> },
  ];
  return <TmPageContainer title="铺货工作台" subTitle="内容和发布包由受限并发的后台任务生成；系统不会自动操作闲鱼或淘宝。">
    <Space wrap style={{ marginBottom: 16 }}><Button onClick={() => actionRef.current?.reload()}>刷新</Button><Button disabled={!selected.length} onClick={() => start('content_generation')}>批量模板内容</Button><Button disabled={!selected.length} onClick={() => start('content_generation', 'ai_generate')}>批量 AI 内容</Button><Button disabled={!selected.length} onClick={() => start('publish_package')}>批量发布包</Button><Tag color="blue">预计利润不等于实际利润</Tag></Space>
    <ProTable<ListingDraft> rowKey="id" actionRef={actionRef} columns={columns} scroll={{ x: 980 }} rowSelection={{ selectedRowKeys: selected, onChange: (keys) => setSelected(keys.map(String)) }} request={async (p) => { const r = await fetchListingDrafts(p); return { data: r.list, total: r.pagination.total, success: true }; }} />
    <Card title="最近后台任务" style={{ marginTop: 16 }}><Table rowKey="id" pagination={false} dataSource={batches.slice(0, 10)} columns={[
      { title: '任务', render: (_, r: ListingOperationBatch) => r.operation === 'content_generation' ? '生成平台内容' : '生成发布包' },
      { title: '进度', render: (_, r: ListingOperationBatch) => <Progress percent={r.total ? Math.round((r.completed + r.failed) * 100 / r.total) : 0} size="small" format={() => `${r.completed}/${r.total}`} /> },
      { title: '处理中', dataIndex: 'processing' }, { title: '失败', dataIndex: 'failed' }, { title: '状态', render: (_, r: ListingOperationBatch) => <Tag>{r.status}</Tag> },
      { title: '操作', render: (_, r: ListingOperationBatch) => <Space>{r.failed > 0 && <a onClick={() => controlListingOperationBatch(r.id, 'retry-failed').then(loadBatches)}>重试失败项</a>}{['pending','running'].includes(r.status) && <a onClick={() => controlListingOperationBatch(r.id, 'cancel').then(loadBatches)}>取消</a>}</Space> },
    ]} /></Card>
  </TmPageContainer>;
}
