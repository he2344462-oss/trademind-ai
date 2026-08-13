import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Dropdown, message } from 'antd';
import { useRef } from 'react';
import { history } from '@umijs/max';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { createListingDraft, fetchCatalogProducts, type CatalogProduct } from '@/services/productFlow';
import { imageCell, money, percent, statusTag } from '../shared';

export default function CatalogProductsPage() {
  const actionRef = useRef<ActionType>();
  const columns: ProColumns<CatalogProduct>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => imageCell(r.images?.[0]?.publicUrl || r.images?.[0]?.originUrl, r.title) },
    { title: '供应商', dataIndex: 'supplier', search: false, width: 140 }, { title: '采购成本', dataIndex: 'purchaseCost', search: false, render: (_, r) => money(r.purchaseCost), width: 110 },
    { title: '建议售价', dataIndex: 'suggestedSalePrice', search: false, render: (_, r) => money(r.suggestedSalePrice), width: 110 }, { title: '预计利润', dataIndex: 'estimatedProfit', search: false, render: (_, r) => money(r.estimatedProfit), width: 110 },
    { title: '利润率', dataIndex: 'estimatedMargin', search: false, render: (_, r) => percent(r.estimatedMargin), width: 90 }, { title: 'SKU', search: false, render: (_, r) => r.skus?.length ?? 0, width: 70 },
    { title: '状态', dataIndex: 'status', render: (_, r) => statusTag(r.catalogStatus), width: 100 },
    { title: '操作', valueType: 'option', render: (_, r) => [<a key="view" onClick={() => history.push(`/product/drafts/${r.id}`)}>查看 / 编辑</a>, <Dropdown key="draft" menu={{ items: [{ key: 'xianyu', label: '闲鱼草稿' }, { key: 'taobao', label: '淘宝草稿' }], onClick: async ({ key }) => { const out = await createListingDraft(r.id, key as 'xianyu' | 'taobao'); message.success(out.created ? '铺货草稿已创建' : '该平台草稿已存在'); } }}><a>创建铺货草稿</a></Dropdown>] },
  ];
  return <TmPageContainer title="商品库" subTitle="已通过人工批准、准备经营的正式商品"><ProTable<CatalogProduct> key="catalog-products-table" rowKey="id" actionRef={actionRef} columns={columns} request={async (p) => { const r = await fetchCatalogProducts(p); return { data: r.list, total: r.pagination.total, success: true }; }} /></TmPageContainer>;
}
