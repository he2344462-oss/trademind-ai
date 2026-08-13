import { ProCard, StatisticCard } from '@ant-design/pro-components';
import { Alert, Progress } from 'antd';
import { useEffect, useState } from 'react';
import { TmPageContainer } from '@/components/ui';
import { fetchSelectionDashboard, type SelectionDashboard } from '@/services/productFlow';

export default function SelectionDashboardPage() {
  const [data, setData] = useState<SelectionDashboard>();
  useEffect(() => { void fetchSelectionDashboard().then(setData); }, []);
  const cards = [['今日新增货源', data?.todaySources], ['待分析候选', data?.pendingCandidates], ['今日已分析', data?.todayAnalyzed], ['强烈建议测试', data?.strongRecommend], ['建议测试', data?.recommend], ['观察', data?.watch], ['不建议测试', data?.reject], ['平均预计利润率', data ? `${(data.averageMarginBps / 100).toFixed(2)}%` : '—']];
  return <TmPageContainer title="AI 选品工作台" subTitle="真实展示候选分析、推荐状态和当前后台任务">
    <Alert style={{ marginBottom: 16 }} type="info" showIcon message="分析结论边界" description="分析可信度衡量输入数据覆盖与质量，不是赚钱概率；市场信号不足时不代表爆款预测。" />
    <ProCard wrap>{cards.map(([title, value]) => <StatisticCard key={String(title)} statistic={{ title: String(title), value: value as string | number | undefined }} colSpan={{ xs: 24, sm: 12, md: 8, lg: 6 }} />)}</ProCard>
    <ProCard title="当前分析任务" style={{ marginTop: 16 }}>{data?.runningBatches.length ? data.runningBatches.map((batch) => <div key={batch.id} style={{ marginBottom: 12 }}><div>{batch.completed + batch.failed} / {batch.total}（失败 {batch.failed}）</div><Progress percent={batch.total ? Math.round((batch.completed + batch.failed) * 100 / batch.total) : 0} /></div>) : '当前没有运行中的分析任务'}</ProCard>
  </TmPageContainer>;
}
