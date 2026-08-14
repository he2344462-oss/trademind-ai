import type { ProColumns } from '@ant-design/pro-components';
import { Alert, Card, Col, Descriptions, Drawer, Row, Statistic, Tag } from 'antd';
import { useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { fetchCostCenter, type CatalogProduct } from '@/services/productFlow';
import { imageCell, money, percent, statusTag } from '../shared';

export default function CostCenterPage() {
  const [selected, setSelected] = useState<CatalogProduct>();
  const [summary, setSummary] = useState({ productCount: 0, averageMarginBps: 0, lowProfitCount: 0, highProfitCount: 0, costAnomalyCount: 0 });
  const columns: ProColumns<CatalogProduct>[] = [
    { title: '商品', render: (_, r) => imageCell(r.images?.[0]?.publicUrl || r.images?.[0]?.originUrl, r.title) },
    { title: '采购成本', render: (_, r) => money(r.purchaseCost), width: 110 }, { title: '建议售价', render: (_, r) => money(r.suggestedSalePrice), width: 110 },
    { title: '采购运费', render: (_, r) => { const status = r.freightStatus || 'unknown'; const labels: Record<string, string> = { verified: '已验证', estimated: '估算', free_shipping: '包邮', unknown: '未知' }; return <><div>{status === 'unknown' ? '待确认' : money(r.freightCost)}</div><Tag color={status === 'verified' || status === 'free_shipping' ? 'green' : status === 'estimated' ? 'gold' : 'red'}>{labels[status]}</Tag></>; }, width: 110 },
    { title: '预计利润', render: (_, r) => money(r.estimatedProfit), width: 110 }, { title: '预计利润率', render: (_, r) => percent(r.estimatedMargin), width: 110 },
    { title: '状态', render: (_, r) => statusTag(r.catalogStatus), width: 90 }, { title: '操作', valueType: 'option', render: (_, r) => <a onClick={() => setSelected(r)}>查看成本</a> },
  ];
  return <TmPageContainer title="成本利润中心" subTitle="基于已批准商品的精确成本与预计利润汇总">
    <Row gutter={[16, 16]} style={{ marginBottom: 16 }}>{[['商品数', summary.productCount], ['平均预计利润率', `${(summary.averageMarginBps / 100).toFixed(2)}%`], ['低利润商品', summary.lowProfitCount], ['高利润商品', summary.highProfitCount], ['成本异常', summary.costAnomalyCount]].map(([title, value]) => <Col xs={24} sm={12} lg={4} key={String(title)}><Card><Statistic title={title} value={value} /></Card></Col>)}</Row>
    <ProTable<CatalogProduct> search={false} rowKey="id" columns={columns} request={async () => { const r = await fetchCostCenter(); setSummary(r); return { data: r.products, total: r.productCount, success: true }; }} />
    <Drawer title="商品成本与利润" width={520} open={Boolean(selected)} onClose={() => setSelected(undefined)}>{selected && <>{selected.freightStatus === 'unknown' || !selected.freightStatus ? <Alert style={{ marginBottom: 16 }} type="warning" showIcon message="采购运费待确认" description="当前预计利润未包含可靠采购运费，利润仅供初筛。" /> : null}<Descriptions column={1} items={[{ key: 'purchase', label: '采购成本', children: money(selected.purchaseCost) }, { key: 'freight', label: '采购运费', children: selected.freightStatus === 'unknown' ? '待确认' : money(selected.freightCost) }, { key: 'freightStatus', label: '运费状态', children: ({ verified: '已验证', estimated: '估算', free_shipping: '包邮', unknown: '未知' } as Record<string, string>)[selected.freightStatus || 'unknown'] }, { key: 'scenario', label: '采购场景', children: `${selected.freightDestination || '未配置地区'} / ${selected.freightQuantity || 1} 件` }, { key: 'packaging', label: '包装成本', children: money(selected.packagingCost) }, { key: 'other', label: '其他成本', children: money(selected.otherCost) }, { key: 'suggested', label: '建议售价', children: money(selected.suggestedSalePrice) }, { key: 'sale', label: '销售价', children: money(selected.salePrice) }, { key: 'profit', label: selected.freightStatus === 'unknown' ? '不含运费利润' : '预计利润', children: money(selected.estimatedProfit) }, { key: 'margin', label: '预计利润率', children: percent(selected.estimatedMargin) }]} /></>}</Drawer>
  </TmPageContainer>;
}
