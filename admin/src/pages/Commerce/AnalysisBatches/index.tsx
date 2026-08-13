import type { ActionType, ProColumns } from '@ant-design/pro-components';
import { Button, Drawer, Popconfirm, Space, Tag, message } from 'antd';
import { useRef, useState } from 'react';
import { TmPageContainer, TmProTable as ProTable } from '@/components/ui';
import { controlCandidateAnalysisBatch, fetchCandidateAnalysisBatchItems, fetchCandidateAnalysisBatches, type BatchItem, type CandidateAnalysisBatch } from '@/services/productFlow';

const batchStatus: Record<string, { text: string; color?: string }> = {
  pending: { text: '等待中' }, running: { text: '分析中', color: 'processing' }, pausing: { text: '正在暂停', color: 'warning' }, paused: { text: '已暂停', color: 'warning' }, cancelling: { text: '正在取消', color: 'warning' }, cancelled: { text: '已取消' }, completed: { text: '已完成', color: 'success' }, partial_failed: { text: '部分失败', color: 'error' }, failed: { text: '失败', color: 'error' },
};

export default function AnalysisBatchesPage() {
  const actionRef = useRef<ActionType>();
  const itemRef = useRef<ActionType>();
  const [selected, setSelected] = useState<CandidateAnalysisBatch>();
  const operate = async (row: CandidateAnalysisBatch, action: 'pause' | 'resume' | 'cancel' | 'retry-failed') => {
    const result = await controlCandidateAnalysisBatch(row.id, action);
    message.success(`任务状态已更新：${batchStatus[result.batch.status]?.text || result.batch.status}`);
    actionRef.current?.reload(); itemRef.current?.reload();
  };
  const columns: ProColumns<CandidateAnalysisBatch>[] = [
    { title: '任务编号', dataIndex: 'id', ellipsis: true, copyable: true },
    { title: '创建时间', dataIndex: 'createdAt', valueType: 'dateTime', search: false },
    { title: '状态', dataIndex: 'status', valueType: 'select', valueEnum: Object.fromEntries(Object.entries(batchStatus).map(([key, item]) => [key, { text: item.text }])), render: (_, row) => <Tag color={batchStatus[row.status]?.color}>{batchStatus[row.status]?.text || row.status}</Tag> },
    { title: '进度', search: false, render: (_, row) => `${row.completed + row.failed} / ${row.total}` },
    { title: '成功', dataIndex: 'completed', search: false }, { title: '失败', dataIndex: 'failed', search: false }, { title: '等待', dataIndex: 'pending', search: false },
    { title: '模式', dataIndex: 'analysisMode', search: false, renderText: (value) => value === 'rules_only' ? '规则分析' : 'AI 解释' },
    { title: '操作', valueType: 'option', render: (_, row) => [<a key="detail" onClick={() => setSelected(row)}>查看详情</a>, ['pending', 'running'].includes(row.status) ? <a key="pause" onClick={() => operate(row, 'pause')}>暂停</a> : null, ['paused', 'pausing'].includes(row.status) ? <a key="resume" onClick={() => operate(row, 'resume')}>恢复</a> : null, ['failed', 'partial_failed'].includes(row.status) ? <a key="retry" onClick={() => operate(row, 'retry-failed')}>重试失败项</a> : null, ['pending', 'running', 'pausing', 'paused'].includes(row.status) ? <Popconfirm key="cancel" title="取消后未开始的商品不会再分析，确定取消？" onConfirm={() => operate(row, 'cancel')}><a>取消</a></Popconfirm> : null].filter(Boolean) },
  ];
  const itemColumns: ProColumns<BatchItem>[] = [
    { title: '商品', render: (_, row) => row.candidate?.sourceProduct?.originalTitle || row.candidateId },
    { title: '状态', dataIndex: 'status' }, { title: '尝试次数', render: (_, row) => `${row.attempts} / ${row.maxAttempts}` },
    { title: '排名分', dataIndex: 'rankingScore' }, { title: '排名', dataIndex: 'rank' },
    { title: '错误类型', dataIndex: 'errorType', renderText: (value) => value === 'retryable' ? '可重试' : value === 'non_retryable' ? '不可重试' : '—' },
    { title: '错误原因', dataIndex: 'errorMessage', ellipsis: true },
  ];
  return <TmPageContainer title="分析任务" subTitle="查看批量选品任务的进度、失败原因和协作式暂停、恢复、取消状态">
    <ProTable<CandidateAnalysisBatch> rowKey="id" actionRef={actionRef} columns={columns} request={async (params) => { const result = await fetchCandidateAnalysisBatches(params); return { data: result.list, total: result.pagination.total, success: true }; }} />
    <Drawer title="分析任务详情" width={900} open={Boolean(selected)} onClose={() => setSelected(undefined)} extra={selected ? <Space><Tag>{batchStatus[selected.status]?.text || selected.status}</Tag></Space> : null}>
      {selected ? <ProTable<BatchItem> rowKey="id" actionRef={itemRef} columns={itemColumns} search={false} request={async (params) => { const result = await fetchCandidateAnalysisBatchItems(selected.id, params); return { data: result.list, total: result.pagination.total, success: true }; }} /> : null}
    </Drawer>
  </TmPageContainer>;
}
