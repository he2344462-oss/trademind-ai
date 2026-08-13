import { Alert, Button, Card, Descriptions, Select, Space, Steps, Table, Upload, message } from 'antd';
import type { UploadProps } from 'antd';
import { useMemo, useState } from 'react';
import type { CSVImportBody, CSVImportPreview } from '@/services/productFlow';

type ImportResult = { imported: number; duplicates: number; failed: Record<string, string> };
type Field = { value: string; label: string; required?: boolean };

const parseHeader = (line: string): string[] => {
  const values: string[] = []; let value = ''; let quoted = false;
  for (let index = 0; index < line.length; index += 1) { const char = line[index]; if (char === '"') { if (quoted && line[index + 1] === '"') { value += '"'; index += 1; } else { quoted = !quoted; } } else if (char === ',' && !quoted) { values.push(value.trim()); value = ''; } else { value += char; } }
  values.push(value.trim()); return values;
};

export default function CsvImportPanel({ fields, preview, confirm, title }: { fields: Field[]; preview: (body: CSVImportBody) => Promise<CSVImportPreview>; confirm: (body: CSVImportBody) => Promise<ImportResult>; title: string }) {
  const [csv, setCsv] = useState(''); const [headers, setHeaders] = useState<string[]>([]); const [mapping, setMapping] = useState<Record<string, string>>({}); const [report, setReport] = useState<CSVImportPreview>(); const [result, setResult] = useState<ImportResult>(); const [loading, setLoading] = useState(false);
  const step = result ? 3 : report ? 2 : csv ? 1 : 0;
  const columns = useMemo(() => Object.keys(report?.rows?.[0] || {}).map((key) => ({ title: fields.find((f) => f.value === key)?.label || key, dataIndex: key, ellipsis: true })), [report, fields]);
  const upload: UploadProps['beforeUpload'] = async (file) => { if (file.size > 2 * 1024 * 1024) { message.error('CSV 不能超过 2 MB'); return Upload.LIST_IGNORE; } const text = await file.text(); if (text.includes('\uFFFD')) { message.error('CSV 必须使用 UTF-8 编码'); return Upload.LIST_IGNORE; } const firstLine = text.replace(/^\uFEFF/, '').split(/\r?\n/, 1)[0] || ''; setCsv(text); setHeaders(parseHeader(firstLine)); setMapping({}); setReport(undefined); setResult(undefined); return false; };
  const runPreview = async () => { setLoading(true); try { setReport(await preview({ csv, mapping })); setResult(undefined); } catch { message.error('预览失败，请检查字段映射和 CSV 内容'); } finally { setLoading(false); } };
  const runConfirm = async () => { setLoading(true); try { const next = await confirm({ csv, mapping }); setResult(next); message.success(`成功导入 ${next.imported} 行`); } catch { message.error('导入失败，请先修正错误行'); } finally { setLoading(false); } };
  return <Card title={title}>
    <Alert type="info" showIcon message="仅支持 UTF-8 CSV，最大 2 MB / 5000 行；测试数据与真实导入数据会分开统计。" style={{ marginBottom: 16 }} />
    <Steps current={step} items={[{ title: '上传' }, { title: '字段映射' }, { title: '验证预览' }, { title: '导入报告' }]} style={{ marginBottom: 20 }} />
    <Upload accept=".csv,text/csv" maxCount={1} beforeUpload={upload}><Button>选择 CSV 文件</Button></Upload>
    {csv ? <div style={{ marginTop: 20 }}><Space wrap>{headers.map((header) => <div key={header} style={{ width: 220 }}><div style={{ marginBottom: 4 }}>{header}</div><Select allowClear style={{ width: '100%' }} placeholder="映射到系统字段" options={fields} value={mapping[header]} onChange={(value) => setMapping((old) => ({ ...old, [header]: value }))} /></div>)}</Space><div style={{ marginTop: 16 }}><Button type="primary" loading={loading} onClick={runPreview}>验证并预览</Button></div></div> : null}
    {report ? <div style={{ marginTop: 20 }}><Descriptions size="small" items={[{ key: 'total', label: '总行数', children: report.totalRows }, { key: 'valid', label: '有效', children: report.validRows }, { key: 'invalid', label: '错误', children: report.invalidRows }]} /><Table rowKey={(_, index) => String(index)} size="small" pagination={false} scroll={{ x: 'max-content' }} columns={columns} dataSource={report.rows} /><Table rowKey={(row) => `${row.row}-${row.field}-${row.message}`} size="small" pagination={{ pageSize: 5 }} columns={[{ title: '行', dataIndex: 'row' }, { title: '字段', dataIndex: 'field' }, { title: '错误', dataIndex: 'message' }]} dataSource={report.errors} /><Button type="primary" disabled={report.validRows === 0} loading={loading} onClick={runConfirm}>确认导入有效行</Button></div> : null}
    {result ? <Alert style={{ marginTop: 16 }} type={Object.keys(result.failed).length ? 'warning' : 'success'} showIcon message={`导入完成：成功 ${result.imported}，重复 ${result.duplicates}，失败 ${Object.keys(result.failed).length}`} description={Object.entries(result.failed).slice(0, 20).map(([row, reason]) => <div key={row}>第 {row} 行：{reason}</div>)} /> : null}
  </Card>;
}
