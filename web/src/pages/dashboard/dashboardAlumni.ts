import { alumniApi } from '../../api/alumni';
import type { AlumniProfile } from '../../types/alumni';

const PAGE_SIZE = 100;
const CACHE_TTL_MS = 2 * 60 * 1000;

let alumniCache: {
  items: AlumniProfile[];
  expiresAt: number;
  addressesEnriched: boolean;
} | null = null;
let alumniRequest: Promise<AlumniProfile[]> | null = null;

function normalize(value?: string) {
  return value?.replace(/\s+/gu, '').trim() || '';
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

function alumniKey(item: Pick<AlumniProfile, 'name' | 'grade' | 'work_unit' | 'mobile'>) {
  return [item.name, item.grade, item.work_unit, item.mobile].map(normalize).join('|');
}

async function fetchAllAlumni() {
  const firstPage = await alumniApi.list({ page: 1, page_size: PAGE_SIZE });
  const allItems = [...firstPage.items];

  // 首页不满一页，说明数据已全部拉取
  if (firstPage.items.length < PAGE_SIZE) {
    return allItems;
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

  return allItems;
}

function cacheAlumni(items: AlumniProfile[], addressesEnriched: boolean) {
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

export async function enrichAlumniMailingAddresses(items: AlumniProfile[]) {
  if (alumniCache?.items === items && alumniCache.addressesEnriched) {
    return items;
  }

  const blob = await alumniApi.exportData({ format: 'csv' });
  const rows = parseCsv(await blob.text()).slice(1);
  const addressQueues = new Map<string, string[]>();

  rows.forEach((columns) => {
    const key = alumniKey({
      name: columns[0] || '',
      grade: columns[1] || '',
      work_unit: columns[9] || '',
      mobile: columns[13] || '',
    });
    const addresses = addressQueues.get(key) || [];
    addresses.push(columns[11] || '');
    addressQueues.set(key, addresses);
  });

  return cacheAlumni(
    items.map((item) => {
      const addresses = addressQueues.get(alumniKey(item));
      return addresses?.length
        ? { ...item, mailing_address: addresses.shift() }
        : item;
    }),
    true,
  );
}
