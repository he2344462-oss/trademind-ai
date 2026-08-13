import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Button, Select, Space, Tag, message } from 'antd';
import { useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { bulkCandidateAction, fetchTopRecommendations, type RecommendationRow } from '@/services/productFlow';
import { imageCell, money, percent } from '../shared';

const labels: Record<string, string> = { strong_recommend: '强烈建议测试', recommend: '建议测试', watch: '观察', reject: '不建议测试' };
const sourceLabels: Record<string, string> = { official: '官方', authorized: '授权', public: '公开', manual: '人工', csv_import: '导入', fixture: '测试数据' };

const marketInfo = (row: RecommendationRow) => {
  const signals = row.analysis.marketSignalSnapshot || [];
  const participating = signals.filter((signal) => signal.participates);
  const origins = [...new Set(signals.map((signal) => signal.origin))];
  const coverage = row.analysis.confidenceBreakdown?.marketCoverageBps || 0;
  return { signals, participating, origins, coverage };
};

export default function RecommendationsPage() {
  const [batchId, setBatchId] = useState('');
  const [limit, setLimit] = useState(20);
  const [marketFilter, setMarketFilter] = useState<'all' | 'with_signal' | 'verified' | 'exclude_fixture'>('all');
  const [selected, setSelected] = useState<string[]>([]);
  const columns: ProColumns<RecommendationRow>[] = [
    { title: '批次 ID', dataIndex: 'batchId', hideInTable: true, fieldProps: { placeholder: '输入分析批次 ID' } },
    { title: '排名', dataIndex: 'rank', width: 70, search: false },
    { title: '商品', render: (_, row) => imageCell(row.candidate.sourceProduct?.originalImages?.[0], row.candidate.sourceProduct?.originalTitle), search: false },
    { title: '来源', render: (_, row) => row.candidate.sourceProduct?.sourcePlatform || '—', search: false },
    { title: '成本', render: (_, row) => money(row.candidate.estimatedCost), search: false },
    { title: '建议售价', render: (_, row) => money(row.candidate.estimatedSalePrice), search: false },
    { title: '预计利润', render: (_, row) => money(row.candidate.estimatedProfit), search: false },
    { title: '利润率', render: (_, row) => percent(row.candidate.estimatedMargin), search: false },
    { title: '商品评分', render: (_, row) => row.analysis.overallScore, search: false },
    { title: '排名分', dataIndex: 'rankingScore', search: false },
    { title: '分析可信度', render: (_, row) => `${row.analysis.confidenceScore}%`, search: false },
    { title: '市场覆盖', render: (_, row) => { const info = marketInfo(row); return <Space size={[0, 4]} wrap><Tag color={info.coverage >= 5000 ? 'green' : info.coverage > 0 ? 'gold' : 'default'}>{info.coverage >= 5000 ? '高' : info.coverage > 0 ? '中' : '低'}</Tag>{info.origins.map((origin) => <Tag key={origin} color={origin === 'fixture' ? 'red' : undefined}>{sourceLabels[origin] || origin}</Tag>)}</Space>; }, search: false },
    { title: '结论', render: (_, row) => <Tag>{labels[row.analysis.recommendation] || row.analysis.recommendation}</Tag>, search: false },
  ];
  return <TmPageContainer title="AI 推荐榜" subTitle="排名来自规则评分、利润、可信度和风险，不包含 LLM 自由判断" extra={[
    <Select key="market" value={marketFilter} onChange={setMarketFilter} options={[{ label: '全部市场数据', value: 'all' }, { label: '仅有市场信号', value: 'with_signal' }, { label: '仅官方/授权', value: 'verified' }, { label: '排除测试数据', value: 'exclude_fixture' }]} />,
    <Select key="limit" value={limit} onChange={setLimit} options={[10, 20, 50].map((value) => ({ label: `TOP ${value}`, value }))} />,
  ]}>
    <Alert style={{ marginBottom: 16 }} type="info" showIcon message="市场数据边界" description="需求或竞争信号缺失时会显示暂无可靠数据；分析可信度不代表爆款概率或赚钱概率。测试数据不会参与正式评分。" />
    <ProTable<RecommendationRow> rowKey="id" columns={columns} rowSelection={{ selectedRowKeys: selected, onChange: (keys) => setSelected(keys.map(String)) }} tableAlertOptionRender={() => <Space><Button type="primary" onClick={async () => { const result = await bulkCandidateAction('approve', selected, 'batch_recommendation'); message.success(`已批准 ${result.completed.length} 项，失败 ${Object.keys(result.failed).length} 项`); setSelected([]); }}>批准进入商品库</Button></Space>} search={{ labelWidth: 'auto' }} params={{ batchId, limit, marketFilter }} request={async (params) => { const id = String(params.batchId || batchId).trim(); if (!id) return { data: [], total: 0, success: true }; const result = await fetchTopRecommendations(id, limit); const list = result.list.filter((row) => { const info = marketInfo(row); if (marketFilter === 'with_signal') return info.participating.length > 0; if (marketFilter === 'verified') return info.participating.some((signal) => ['official', 'authorized'].includes(signal.origin)); if (marketFilter === 'exclude_fixture') return !info.origins.includes('fixture'); return true; }); return { data: list, total: list.length, success: true }; }} form={{ onValuesChange: (values) => setBatchId(String(values.batchId || '')) }} columnsState={{ value: {} }} />
  </TmPageContainer>;
}
