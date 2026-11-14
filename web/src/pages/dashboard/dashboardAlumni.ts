import { alumniApi } from '../../api/alumni';
import type { AlumniListItem, AlumniProfile } from '../../types/alumni';

const PAGE_SIZE = 100;
const CACHE_TTL_MS = 2 * 60 * 1000;

/**
 * 大屏列表把多段学籍展开成便于筛选的扁平视图：
 * 个人信息来自个人主档案，年级/班级/专业/导师等取第一段可访问学籍。
 * 完整多段信息在详情弹窗中展示。
 */
export interface DashboardAlumni extends AlumniListItem {
  grade?: string;
  class_name?: string;
  cohort?: string;
  counselor?: string;
  mentor?: string;
  major?: string;
  training_mode?: string;
  mailing_address?: string;
}

let alumniCache: {
  items: DashboardAlumni[];
  expiresAt: number;
  addressesEnriched: boolean;
} | null = null;
let alumniRequest: Promise<DashboardAlumni[]> | null = null;

function normalize(value?: string) {
  return value?.replace(/\s+/gu, '').trim() || '';
}

export function flattenAlumni(item: AlumniListItem): DashboardAlumni {
  const primary = item.enrollments?.[0];
  return {
    ...item,
    grade: primary?.grade,
    class_name: primary?.class_name,
    cohort: primary?.cohort,
    counselor: primary?.counselor,
    mentor: primary?.mentor,
    major: primary?.major,
    training_mode: primary?.training_mode,
  };
}

function parseCsv(text: string) {
  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let quoted = false;
  const source = text.replace(/^\uFEFF/u, '');

  for (let index = 0; index < source.length; index += 1) {
    const character = source[index];
    if (character === '"') {
      if (quoted && source[index + 1] === '"') {
        field += '"';
        index += 1;
      } else {
        quoted = !quoted;
      }
    } else if (character === ',' && !quoted) {
      row.push(field);
      field = '';
    } else if ((character === '\n' || character === '\r') && !quoted) {
      if (character === '\r' && source[index + 1] === '\n') {
        index += 1;
      }
      row.push(field);
      if (row.some(Boolean)) {
        rows.push(row);
      }
      row = [];
      field = '';
    } else {
      field += character;
    }
  }

  row.push(field);
  if (row.some(Boolean)) {
    rows.push(row);
  }
  return rows;
}

function alumniKey(item: Pick<DashboardAlumni, 'name' | 'work_unit' | 'mobile'>) {
  return [item.name, item.work_unit, item.mobile].map(normalize).join('|');
}

async function fetchAllAlumni() {
  const firstPage = await alumniApi.list({ page: 1, page_size: PAGE_SIZE });
  const allItems = [...firstPage.items];

  // 首页不满一页，说明数据已全部拉取
  if (firstPage.items.length < PAGE_SIZE) {
    return allItems.map(flattenAlumni);
  }

  // 用 total 估算页数以并行拉取，但不盲信（缓存计数可能偏小）
  const estimatedPages = Math.max(2, Math.ceil(firstPage.total / PAGE_SIZE));

  const batchPages = await Promise.all(
    Array.from({ length: estimatedPages - 1 }, (_, index) =>
      alumniApi.list({ page: index + 2, page_size: PAGE_SIZE }),
    ),
  );

  for (const page of batchPages) {
    allItems.push(...page.items);
  }

  // 兜底：最后一批并行页仍然满载，说明 total 偏小，继续顺序拉取直到空页或不完整页
  const lastBatch = batchPages[batchPages.length - 1];
  if (lastBatch && lastBatch.items.length === PAGE_SIZE) {
    let page = estimatedPages + 1;
    while (true) {
      const result = await alumniApi.list({ page, page_size: PAGE_SIZE });
      if (result.items.length === 0) break;
      allItems.push(...result.items);
      if (result.items.length < PAGE_SIZE) break;
      page++;
    }
  }

  return allItems.map(flattenAlumni);
}

function cacheAlumni(items: DashboardAlumni[], addressesEnriched: boolean) {
  alumniCache = {
    items,
    expiresAt: Date.now() + CACHE_TTL_MS,
    addressesEnriched,
  };
  return items;
}

export async function loadAllAlumni() {
  if (alumniCache && alumniCache.expiresAt > Date.now()) {
    return alumniCache.items;
  }
  if (alumniRequest) {
    return alumniRequest;
  }

  alumniRequest = fetchAllAlumni()
    .then((items) => cacheAlumni(items, false))
    .finally(() => {
      alumniRequest = null;
    });
  return alumniRequest;
}

export async function enrichAlumniMailingAddresses(items: DashboardAlumni[]) {
  if (alumniCache?.items === items && alumniCache.addressesEnriched) {
    return items;
  }

  const blob = await alumniApi.exportData({ format: 'csv' });
  const rows = parseCsv(await blob.text()).slice(1);
  const addressQueues = new Map<string, string[]>();

  // 导出平铺为一行一段学籍：姓名 0、工作单位 10、通讯地址 12、手机号 14。
  rows.forEach((columns) => {
    const key = alumniKey({
      name: columns[0] || '',
      work_unit: columns[10] || '',
      mobile: columns[14] || '',
    });
    const addresses = addressQueues.get(key) || [];
    addresses.push(columns[12] || '');
    addressQueues.set(key, addresses);
  });

  return cacheAlumni(
    items.map((item) => {
      const addresses = addressQueues.get(alumniKey(item));
      return addresses?.length ? { ...item, mailing_address: addresses.shift() } : item;
    }),
    true,
  );
}

/** 详情弹窗使用完整个人档案（含全部可访问学籍段）。 */
export type DashboardProfile = AlumniProfile;
