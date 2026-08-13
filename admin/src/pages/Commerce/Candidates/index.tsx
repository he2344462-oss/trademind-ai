import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Popconfirm, Space, message } from 'antd';
import { useRef } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { approveCandidate, fetchCandidates, rejectCandidate, watchCandidate, type Candidate } from '@/services/productFlow';
import { imageCell, money, percent, statusTag } from '../shared';

export default function CandidatesPage() {
  const actionRef = useRef<ActionType>();
  const run = async (op: () => Promise<unknown>, text: string) => { await op(); message.success(text); actionRef.current?.reload(); };
  const columns: ProColumns<Candidate>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => imageCell(r.sourceProduct?.originalImages?.[0], r.sourceProduct?.originalTitle) },
    { title: '来源', search: false, render: (_, r) => r.sourceProduct?.sourcePlatform || '—', width: 90 },
    { title: '采购成本', dataIndex: 'estimatedCost', search: false, render: (_, r) => money(r.estimatedCost), width: 110 },
    { title: '预计售价', dataIndex: 'estimatedSalePrice', search: false, render: (_, r) => money(r.estimatedSalePrice), width: 110 },
    { title: '预计利润', dataIndex: 'estimatedProfit', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '利润率', dataIndex: 'estimatedMargin', search: false, render: (_, r) => percent(r.estimatedMargin), width: 90 },
    { title: '潜力评分', dataIndex: 'potentialScore', search: false, renderText: (v) => v ?? '待评分', width: 100 },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { pending: { text: '待分析' }, watch: { text: '观察' }, rejected: { text: '淘汰' }, approved: { text: '已批准' } }, render: (_, r) => statusTag(r.status), width: 100 },
    { title: '操作', valueType: 'option', render: (_, r) => r.status === 'approved' || r.status === 'rejected' ? [] : [<Popconfirm key="approve" title="批准后将生成正式商品，确定继续？" onConfirm={() => run(() => approveCandidate(r.id), '已批准并生成商品')}><a>批准</a></Popconfirm>, <a key="watch" onClick={() => run(() => watchCandidate(r.id), '已加入观察')}>观察</a>, <Popconfirm key="reject" title="确定淘汰这个候选商品？" onConfirm={() => run(() => rejectCandidate(r.id, '人工淘汰'), '已淘汰')}><a>淘汰</a></Popconfirm>] },
  ];
  return <TmPageContainer title="AI 候选池" subTitle="Sprint 1 使用人工状态流转，评分字段暂时允许为空"><ProTable<Candidate> key="candidates-table" rowKey="id" actionRef={actionRef} columns={columns} request={async (p) => { const r = await fetchCandidates(p); return { data: r.list, total: r.pagination.total, success: true }; }} /></TmPageContainer>;
}
