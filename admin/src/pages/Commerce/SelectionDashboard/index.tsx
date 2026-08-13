import { ProCard, StatisticCard } from '@ant-design/pro-components';
import { Alert, Progress, Tag } from 'antd';
import { useEffect, useState } from 'react';
import { TmPageContainer } from '@/components/ui';
import { fetchPerformanceSummary, fetchSelectionDashboard, fetchSelectionPerformance, type PerformanceSummary, type SelectionDashboard, type SelectionPerformanceReport } from '@/services/productFlow';

export default function SelectionDashboardPage() {
  const [data, setData] = useState<SelectionDashboard>();
  const [performance, setPerformance] = useState<PerformanceSummary>();
  const [evaluation, setEvaluation] = useState<SelectionPerformanceReport>();
  useEffect(() => { void Promise.all([fetchSelectionDashboard(), fetchPerformanceSummary(), fetchSelectionPerformance()]).then(([dashboard, summary, report]) => { setData(dashboard); setPerformance(summary); setEvaluation(report); }); }, []);
  const cards: Array<[string, string | number | undefined]> = [['今日新增货源', data?.todaySources], ['待分析候选', data?.pendingCandidates], ['今日完成分析', data?.todayAnalyzed], ['强烈建议测试', data?.strongRecommend], ['建议测试', data?.recommend], ['观察', data?.watch], ['不建议测试', data?.reject], ['平均预计利润率', data ? `${(data.averageMarginBps / 100).toFixed(2)}%` : '—'], ['真实市场数据覆盖', data ? `${(data.realMarketCoverageBps / 100).toFixed(0)}%` : '—'], ['真实销售反馈覆盖', data ? `${(data.realPerformanceCoverageBps / 100).toFixed(0)}%` : '—'], ['Calibration 样本', data?.calibrationSampleCount], ['Selection Config', data?.selectionConfigVersion]];
  cards.push(['待生成内容', data?.pendingContent], ['待审核', data?.pendingReview], ['可发布', data?.readyToPublish], ['已发布', data?.publishedListings], ['今日生成商品', data?.todayGeneratedContent], ['预计上架利润', data ? `¥${data.estimatedListingProfit}` : '—']);
  return <TmPageContainer title="AI 选品工作台" subTitle="汇总货源、候选分析、市场信号覆盖与用户导入的销售表现">
    <Alert style={{ marginBottom: 16 }} type="info" showIcon message="运营摘要" description={data?.operationalSummary || '正在汇总运营数据…'} />
    <Alert style={{ marginBottom: 16 }} type="warning" showIcon message="分析结论边界" description="分析数据可信度不是赚钱概率。市场信号不足时，推荐主要来自利润、商品资料、供应与基础风险，建议小规模测试。" />
    <ProCard wrap>{cards.map(([title, value]) => <StatisticCard key={title} statistic={{ title, value }} colSpan={{ xs: 24, sm: 12, md: 8, lg: 6 }} />)}</ProCard>
    <ProCard title="近期分析任务" style={{ marginTop: 16 }}>{data?.runningBatches.length ? data.runningBatches.map((batch) => <div key={batch.id} style={{ marginBottom: 12 }}><div><Tag>{batch.status}</Tag>{batch.completed + batch.failed} / {batch.total}（失败 {batch.failed}）</div><Progress percent={batch.total ? Math.round((batch.completed + batch.failed) * 100 / batch.total) : 0} /></div>) : '当前没有分析中或暂停的任务'}</ProCard>
    <ProCard title="选品效果反馈" style={{ marginTop: 16 }}>{evaluation?.groups.length ? evaluation.groups.map((group) => <div key={group.recommendation} style={{ marginBottom: 8 }}>{group.recommendation}：已测试 {group.tested}，有成交 {group.withOrders}，正利润 {group.positiveProfit}</div>) : '尚未导入可用于评估的销售表现数据'}</ProCard>
  </TmPageContainer>;
}
