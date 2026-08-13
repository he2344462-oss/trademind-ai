import { Image, Tag, Typography } from 'antd';

export const money = (value?: number) => value == null ? '—' : `¥${value.toFixed(2)}`;
export const percent = (value?: number) => value == null ? '—' : `${(value * 100).toFixed(1)}%`;
export const imageCell = (url?: string, title?: string) => (
  <div style={{ display: 'flex', alignItems: 'center', gap: 12, minWidth: 220 }}>
    {url ? <Image src={url} width={48} height={48} style={{ objectFit: 'cover', borderRadius: 6 }} preview={false} /> : <div style={{ width: 48, height: 48, borderRadius: 6, background: '#f3f5f7' }} />}
    <Typography.Text ellipsis={{ tooltip: title }} style={{ maxWidth: 260 }}>{title || '未命名商品'}</Typography.Text>
  </div>
);
export const statusTag = (status?: string) => {
  const color: Record<string, string> = { approved: 'green', recommended: 'blue', watch: 'gold', rejected: 'red', active: 'green', ready: 'blue', failed: 'red' };
  return <Tag color={color[status || '']}>{status || '—'}</Tag>;
};
