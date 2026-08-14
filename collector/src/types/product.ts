/**
 * 与 Go / 主业务约定的统一商品结构（任何采集源最终归一为此格式）。
 */
export type NormalizedProduct = {
  source: string;
  sourceUrl: string;
  title: string;
  currency: string;
  /** 页面真实文本描述（非 AI 生成） */
  mainDescription?: string;
  mainImages: string[];
  descriptionImages: string[];
  attributes: Record<string, string | number | boolean>;
  skus: ProductSku[];
  /** 采购物流证据。金额使用十进制字符串，避免跨语言浮点误差。 */
  freight?: ProductFreight;
  /** 平台页原始快照，必填以便复盘与二次解析 */
  raw: Record<string, unknown>;
};

export type ProductFreight = {
  status: 'verified' | 'estimated' | 'free_shipping' | 'unknown';
  orderAmount?: string;
  unitAmount?: string;
  currency: string;
  destination?: string;
  quantity: number;
  source: string;
  confidenceBps: number;
  observedAt: string;
  rawSnapshot: Record<string, unknown>;
  calculationMethod: string;
};

export type ProductSku = {
  id?: string;
  /** 如颜色、尺码等键值 */
  properties?: Record<string, string>;
  price?: number;
  stock?: number;
  skuCode?: string;
  image?: string;
  /** SKU 粒度原始快照（Go 入库时保留在 product_skus.raw_data） */
  raw?: Record<string, unknown>;
};
