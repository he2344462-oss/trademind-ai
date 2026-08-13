import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Select, Space, Tag, message } from 'antd';
import { useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { bulkCandidateAction, fetchTopRecommendations, type RecommendationRow } from '@/services/productFlow';
import { imageCell, money, percent } from '../shared';

const labels: Record<string, string> = { strong_recommend: '强烈建议测试', recommend: '建议测试', watch: '观察', reject: '不建议测试' };

export default function RecommendationsPage() {
  const [batchId, setBatchId] = useState(''); const [limit, setLimit] = useState(20);
  const [selected, setSelected] = useState<string[]>([]);
  const columns: ProColumns<RecommendationRow>[] = [
    { title: '批次 ID', dataIndex: 'batchId', hideInTable: true, fieldProps: { placeholder: '输入分析批次 ID' } },
    { title: '排名', dataIndex: 'rank', width: 70, search: false },
    { title: '商品', render: (_, r) => imageCell(r.candidate.sourceProduct?.originalImages?.[0], r.candidate.sourceProduct?.originalTitle), search: false },
    { title: '来源', render: (_, r) => r.candidate.sourceProduct?.sourcePlatform || '—', search: false },
    { title: '成本', render: (_, r) => money(r.candidate.estimatedCost), search: false },
    { title: '建议售价', render: (_, r) => money(r.candidate.estimatedSalePrice), search: false },
    { title: '预计利润', render: (_, r) => money(r.candidate.estimatedProfit), search: false },
    { title: '利润率', render: (_, r) => percent(r.candidate.estimatedMargin), search: false },
    { title: '商品评分', render: (_, r) => r.analysis.overallScore, search: false },
    { title: '排名分', dataIndex: 'rankingScore', search: false },
    { title: '分析可信度', render: (_, r) => `${r.analysis.confidenceScore}%`, search: false },
    { title: '结论', render: (_, r) => <Tag>{labels[r.analysis.recommendation] || r.analysis.recommendation}</Tag>, search: false },
  ];
  return <TmPageContainer title="AI 推荐榜" subTitle="排名来自规则评分、利润、可信度和风险，不包含 LLM 自由判断" extra={[<Select key="limit" value={limit} onChange={setLimit} options={[10, 20, 50].map((value) => ({ label: `TOP ${value}`, value }))} />]}>
    <Alert style={{ marginBottom: 16 }} type="info" showIcon message="市场数据边界" description="需求或竞争信号缺失时会显示暂无可靠数据；当前排名不代表爆款概率或赚钱概率。" />
    <ProTable<RecommendationRow> rowKey="id" columns={columns} rowSelection={{ selectedRowKeys: selected, onChange: (keys) => setSelected(keys.map(String)) }} tableAlertOptionRender={() => <Space><Button type="primary" onClick={async () => { const result = await bulkCandidateAction('approve', selected, 'batch_recommendation'); message.success(`已批准 ${result.completed.length} 项，失败 ${Object.keys(result.failed).length} 项`); setSelected([]); }}>批准进入商品库</Button></Space>} search={{ labelWidth: 'auto' }} params={{ batchId, limit }} request={async (params) => { const id = String(params.batchId || batchId).trim(); if (!id) return { data: [], total: 0, success: true }; const result = await fetchTopRecommendations(id, limit); return { data: result.list, total: result.list.length, success: true }; }} form={{ onValuesChange: (values) => setBatchId(String(values.batchId || '')) }} columnsState={{ value: {} }} />
  </TmPageContainer>;
}
