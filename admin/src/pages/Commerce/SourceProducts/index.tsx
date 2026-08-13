import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { ModalForm, ProFormDigit, ProFormText, ProFormTextArea } from '@ant-design/pro-components';
import { Button, Drawer, Descriptions, Image, message } from 'antd';
import { useRef, useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { formatDateTime } from '@/utils/formatTime';
import { addSourceToCandidates, createSourceProduct, fetchSourceProducts, type SourceProduct } from '@/services/productFlow';
import { imageCell, money, statusTag } from '../shared';

export default function SourceProductsPage() {
  const actionRef = useRef<ActionType>(); const [open, setOpen] = useState(false); const [detail, setDetail] = useState<SourceProduct>();
  const columns: ProColumns<SourceProduct>[] = [
    { title: '商品', dataIndex: 'keyword', render: (_, r) => imageCell(r.originalImages?.[0], r.originalTitle) },
    { title: '来源', dataIndex: 'sourcePlatform', width: 90 }, { title: '供应商', dataIndex: 'supplierName', search: false, width: 140 },
    { title: '采购价', dataIndex: 'sourcePrice', search: false, render: (_, r) => money(r.sourcePrice), width: 100 },
    { title: '运费', dataIndex: 'freight', search: false, render: (_, r) => money(r.freight), width: 90 },
    { title: '规格', search: false, render: (_, r) => r.skuData?.length ?? 0, width: 70 },
    { title: '采集时间', dataIndex: 'collectedAt', search: false, renderText: (value) => formatDateTime(value), width: 170 },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: { collected: { text: '已采集' }, candidate: { text: '已入候选' }, approved: { text: '已批准' } }, render: (_, r) => statusTag(r.status), width: 100 },
    { title: '操作', valueType: 'option', render: (_, r) => [<a key="detail" onClick={() => setDetail(r)}>查看详情</a>, <a key="candidate" onClick={async () => { const out = await addSourceToCandidates(r.id); message.success(out.created ? '已加入候选池' : '候选记录已存在'); actionRef.current?.reload(); }}>加入候选池</a>] },
  ];
  return <TmPageContainer title="货源池" subTitle="保存未经运营加工的原始货源数据">
    <ProTable<SourceProduct> key="source-products-table" rowKey="id" actionRef={actionRef} columns={columns} request={async (params) => { const r = await fetchSourceProducts(params); return { data: r.list, total: r.pagination.total, success: true }; }} toolBarRender={() => [<Button key="create" type="primary" onClick={() => setOpen(true)}>导入测试货源</Button>]} />
    <ModalForm title="导入测试货源" open={open} onOpenChange={setOpen} onFinish={async (v) => { await createSourceProduct({ ...v, minOrderQuantity: 1, originalImages: v.imageUrl ? [v.imageUrl] : [], skuData: [] }); message.success('货源创建成功'); actionRef.current?.reload(); return true; }}>
      <ProFormText name="sourcePlatform" label="来源平台" initialValue="1688" rules={[{ required: true }]} /><ProFormText name="sourceProductId" label="来源商品 ID" rules={[{ required: true }]} /><ProFormText name="sourceUrl" label="来源链接" rules={[{ required: true }]} /><ProFormText name="supplierName" label="供应商" /><ProFormText name="originalTitle" label="原始标题" rules={[{ required: true }]} /><ProFormTextArea name="originalDescription" label="原始描述" /><ProFormText name="imageUrl" label="商品图片 URL" /><ProFormDigit name="sourcePrice" label="采购价" min={0} fieldProps={{ precision: 2 }} /><ProFormDigit name="freight" label="运费" min={0} fieldProps={{ precision: 2 }} />
    </ModalForm>
    <Drawer title="货源详情" width={560} open={Boolean(detail)} onClose={() => setDetail(undefined)}>
      {detail && <><Image src={detail.originalImages?.[0]} width={160} fallback="data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=" /><Descriptions column={1} bordered size="small" style={{ marginTop: 16 }} items={[{ key: 'title', label: '原始标题', children: detail.originalTitle }, { key: 'platform', label: '来源平台', children: detail.sourcePlatform }, { key: 'supplier', label: '供应商', children: detail.supplierName || '—' }, { key: 'price', label: '采购价', children: money(detail.sourcePrice) }, { key: 'freight', label: '运费', children: money(detail.freight) }, { key: 'url', label: '来源链接', children: <a href={detail.sourceUrl} target="_blank" rel="noreferrer">打开原商品</a> }, { key: 'description', label: '原始描述', children: detail.originalDescription || '—' }]} /></>}
    </Drawer>
  </TmPageContainer>;
}
