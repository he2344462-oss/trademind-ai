import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormSelect, ProFormText } from '@ant-design/pro-components';
import { message } from 'antd';
import { useRef, useState } from 'react';
import { history } from '@umijs/max';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { createListingDraft, fetchCatalogProducts, type AnalyzeCandidateInput, type CatalogProduct } from '@/services/productFlow';
import { imageCell, money, percent, statusTag } from '../shared';

type ListingForm = { platform: 'xianyu' | 'taobao'; code?: string; platformFeeBps?: number; platformFeeFixed?: number; paymentFeeBps?: number; paymentFeeFixed?: number; returnReserveBps?: number; otherBps?: number; otherFixed?: number };
export default function CatalogProductsPage() {
  const actionRef = useRef<ActionType>(); const [drafting, setDrafting] = useState<CatalogProduct>();
  const columns: ProColumns<CatalogProduct>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => imageCell(r.images?.[0]?.publicUrl || r.images?.[0]?.originUrl, r.title) },
    { title: '供应商', dataIndex: 'supplier', search: false, width: 140 }, { title: '采购成本', dataIndex: 'purchaseCost', search: false, render: (_, r) => money(r.purchaseCost), width: 110 },
    { title: '建议售价', dataIndex: 'suggestedSalePrice', search: false, render: (_, r) => money(r.suggestedSalePrice), width: 110 }, { title: '预计利润', dataIndex: 'estimatedProfit', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '利润率', dataIndex: 'estimatedMargin', search: false, render: (_, r) => percent(r.estimatedMargin), width: 90 }, { title: 'SKU', search: false, render: (_, r) => r.skus?.length ?? 0, width: 70 },
    { title: '状态', dataIndex: 'status', render: (_, r) => statusTag(r.catalogStatus), width: 100 },
    { title: '操作', valueType: 'option', render: (_, r) => [<a key="view" onClick={() => history.push(`/product/drafts/${r.id}`)}>查看 / 编辑</a>, <a key="draft" onClick={() => setDrafting(r)}>创建铺货草稿</a>] },
  ];
  return <TmPageContainer title="商品库" subTitle="已通过人工批准、准备经营的正式商品">
    <ProTable<CatalogProduct> rowKey="id" actionRef={actionRef} columns={columns} request={async (p) => { const r = await fetchCatalogProducts(p); return { data: r.list, total: r.pagination.total, success: true }; }} />
    <ModalForm<ListingForm> title="创建平台铺货草稿" open={Boolean(drafting)} onOpenChange={(open) => !open && setDrafting(undefined)} initialValues={{ platform: 'xianyu', platformFeeBps: 0, paymentFeeBps: 0, returnReserveBps: 0, otherBps: 0 }} onFinish={async (values) => {
      if (!drafting) return false; const moneyString = (v?: number) => v === undefined ? undefined : v.toFixed(2);
      const profile: AnalyzeCandidateInput['pricingProfile'] = { code: values.code, platformFeeBps: values.platformFeeBps, platformFeeFixed: moneyString(values.platformFeeFixed), paymentFeeBps: values.paymentFeeBps, paymentFeeFixed: moneyString(values.paymentFeeFixed), returnReserveBps: values.returnReserveBps, otherBps: values.otherBps, otherFixed: moneyString(values.otherFixed) };
      const out = await createListingDraft(drafting.id, values.platform, profile); message.success(out.created ? '铺货草稿已创建' : '该平台草稿已存在'); setDrafting(undefined); return true;
    }}>
      <ProFormSelect name="platform" label="平台" rules={[{ required: true }]} options={[{ label: '闲鱼', value: 'xianyu' }, { label: '淘宝', value: 'taobao' }]} /><ProFormText name="code" label="费用配置名称" placeholder="例如 taobao-custom；留空表示未配置" />
      <ProFormDigit name="platformFeeBps" label="平台费率（基点，500=5%）" min={0} max={9999} /><ProFormDigit name="platformFeeFixed" label="平台固定费用（元）" min={0} fieldProps={{ precision: 2 }} /><ProFormDigit name="paymentFeeBps" label="支付费率（基点）" min={0} max={9999} /><ProFormDigit name="paymentFeeFixed" label="支付固定费用（元）" min={0} fieldProps={{ precision: 2 }} /><ProFormDigit name="returnReserveBps" label="退货预留率（基点）" min={0} max={9999} /><ProFormDigit name="otherBps" label="其他费率（基点）" min={0} max={9999} /><ProFormDigit name="otherFixed" label="其他固定费用（元）" min={0} fieldProps={{ precision: 2 }} />
    </ModalForm>
  </TmPageContainer>;
}
