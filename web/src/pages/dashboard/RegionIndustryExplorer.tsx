import { useEffect, useMemo, useState } from 'react';
import { EnvironmentOutlined, SearchOutlined } from '@ant-design/icons';
import { Input, Segmented, Spin, message } from 'antd';
import { alumniApi } from '../../api/alumni';
import type { AlumniProfile } from '../../types/alumni';

interface RegionIndustryExplorerProps {
  expanded: boolean;
  onSelectAlumni: (profile: AlumniProfile) => void;
}

type RegionScope = 'shandong' | 'outside';

interface AlumniRegion {
  scope: RegionScope;
  region: string;
  district?: string;
}

const shandongRegions: Record<string, string[]> = {
  济南市: ['历下区', '市中区', '槐荫区', '天桥区', '历城区', '长清区', '章丘区', '济阳区', '莱芜区', '钢城区', '平阴县', '商河县'],
  青岛市: ['市南区', '市北区', '黄岛区', '崂山区', '李沧区', '城阳区', '即墨区', '胶州市', '平度市', '莱西市'],
  淄博市: ['淄川区', '张店区', '博山区', '临淄区', '周村区', '桓台县', '高青县', '沂源县'],
  枣庄市: ['市中区', '薛城区', '峄城区', '台儿庄区', '山亭区', '滕州市'],
  东营市: ['东营区', '河口区', '垦利区', '利津县', '广饶县'],
  烟台市: ['芝罘区', '福山区', '牟平区', '莱山区', '蓬莱区', '龙口市', '莱阳市', '莱州市', '招远市', '栖霞市', '海阳市'],
  潍坊市: ['潍城区', '寒亭区', '坊子区', '奎文区', '临朐县', '昌乐县', '青州市', '诸城市', '寿光市', '安丘市', '高密市', '昌邑市'],
  济宁市: ['任城区', '兖州区', '微山县', '鱼台县', '金乡县', '嘉祥县', '汶上县', '泗水县', '梁山县', '曲阜市', '邹城市'],
  泰安市: ['泰山区', '岱岳区', '宁阳县', '东平县', '新泰市', '肥城市'],
  威海市: ['环翠区', '文登区', '荣成市', '乳山市'],
  日照市: ['东港区', '岚山区', '五莲县', '莒县'],
  临沂市: ['兰山区', '罗庄区', '河东区', '沂南县', '郯城县', '沂水县', '兰陵县', '费县', '平邑县', '莒南县', '蒙阴县', '临沭县'],
  德州市: ['德城区', '陵城区', '宁津县', '庆云县', '临邑县', '齐河县', '平原县', '夏津县', '武城县', '乐陵市', '禹城市'],
  聊城市: ['东昌府区', '茌平区', '阳谷县', '莘县', '东阿县', '冠县', '高唐县', '临清市'],
  滨州市: ['滨城区', '沾化区', '惠民县', '阳信县', '无棣县', '博兴县', '邹平市'],
  菏泽市: ['牡丹区', '定陶区', '曹县', '单县', '成武县', '巨野县', '郓城县', '鄄城县', '东明县'],
};

const outsideRegions = [
  '北京市', '天津市', '河北省', '山西省', '内蒙古自治区', '辽宁省', '吉林省', '黑龙江省',
  '上海市', '江苏省', '浙江省', '安徽省', '福建省', '江西省', '河南省', '湖北省',
  '湖南省', '广东省', '广西壮族自治区', '海南省', '重庆市', '四川省', '贵州省',
  '云南省', '西藏自治区', '陕西省', '甘肃省', '青海省', '宁夏回族自治区',
  '新疆维吾尔自治区', '香港特别行政区', '澳门特别行政区', '台湾省',
];

const regionAliases = new Map<string, string>([
  ...outsideRegions.flatMap((region) => {
    const shortName = region.replace(/特别行政区|维吾尔自治区|壮族自治区|回族自治区|自治区|省|市$/u, '');
    return [[region, region], [shortName, region]] as Array<[string, string]>;
  }),
  ['内蒙', '内蒙古自治区'],
  ['广西', '广西壮族自治区'],
  ['宁夏', '宁夏回族自治区'],
  ['新疆', '新疆维吾尔自治区'],
  ['西藏', '西藏自治区'],
  ['香港', '香港特别行政区'],
  ['澳门', '澳门特别行政区'],
]);

const cityAliases = new Map<string, string>();
Object.entries(shandongRegions).forEach(([city, districts]) => {
  cityAliases.set(city, city);
  cityAliases.set(city.replace(/市$/u, ''), city);
  districts.forEach((district) => {
    cityAliases.set(district, city);
    cityAliases.set(district.replace(/[市区县]$/u, ''), city);
  });
});

const locationAliases = [...new Set([
  '山东省',
  '山东',
  ...cityAliases.keys(),
  ...regionAliases.keys(),
])].sort((left, right) => right.length - left.length);

const branchSuffixes = ['分公司', '分行', '支行', '办事处', '联络处', '营业部', '项目部'];

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
  const pageSize = 100;
  const firstPage = await alumniApi.list({ page: 1, page_size: pageSize });
  const pageCount = Math.ceil(firstPage.total / pageSize);
  const remainingPages = pageCount > 1
    ? await Promise.all(
      Array.from({ length: pageCount - 1 }, (_, index) =>
        alumniApi.list({ page: index + 2, page_size: pageSize }),
      ),
    )
    : [];
  const items = [firstPage.items, ...remainingPages.map((page) => page.items)]
    .flat()
    .map((item) => ({ ...item }));

  try {
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

    items.forEach((item) => {
      const addresses = addressQueues.get(alumniKey(item));
      if (addresses?.length) {
        item.mailing_address = addresses.shift();
      }
    });
  } catch {
    message.warning('通讯地址加载失败，地域将仅按工作单位判定');
  }

  return items;
}

function findBranchLocation(workUnit: string) {
  for (const alias of locationAliases) {
    const aliasIndex = workUnit.indexOf(alias);
    if (aliasIndex < 0) {
      continue;
    }

    const afterAlias = workUnit.slice(aliasIndex + alias.length, aliasIndex + alias.length + 5);
    const beforeAlias = workUnit.slice(Math.max(0, aliasIndex - 1), aliasIndex);
    if (branchSuffixes.some((suffix) => afterAlias.startsWith(suffix)) || beforeAlias === '驻') {
      return alias;
    }
  }
  return '';
}

function earliestAlias(text: string, aliases: Iterable<string>) {
  let match = '';
  let matchIndex = Number.POSITIVE_INFINITY;
  for (const alias of aliases) {
    const index = text.indexOf(alias);
    if (index >= 0 && index < matchIndex) {
      match = alias;
      matchIndex = index;
    }
  }
  return match;
}

function districtForCity(text: string, city: string) {
  return earliestAlias(text, shandongRegions[city] || []);
}

function regionFromAlias(alias: string, source: string): AlumniRegion | null {
  const city = cityAliases.get(alias);
  if (city) {
    return {
      scope: 'shandong',
      region: city,
      district: districtForCity(source, city) || undefined,
    };
  }
  if (alias === '山东' || alias === '山东省') {
    const matchedCityAlias = earliestAlias(source, cityAliases.keys());
    const matchedCity = cityAliases.get(matchedCityAlias);
    if (matchedCity) {
      return {
        scope: 'shandong',
        region: matchedCity,
        district: districtForCity(source, matchedCity) || undefined,
      };
    }

    // Provincial organizations without a city in their unit name are based in Jinan.
    // Their mailing address may be a temporary residence or secondment location.
    return {
      scope: 'shandong',
      region: '济南市',
    };
  }
  const outsideRegion = regionAliases.get(alias);
  return outsideRegion ? { scope: 'outside', region: outsideRegion } : null;
}

function resolveRegion(item: AlumniProfile): AlumniRegion | null {
  const workUnit = normalize(item.work_unit);
  const address = normalize(item.mailing_address);
  const branchAlias = findBranchLocation(workUnit);
  if (branchAlias) {
    return regionFromAlias(branchAlias, workUnit);
  }

  const workAlias = earliestAlias(workUnit, locationAliases);
  const workRegion = regionFromAlias(workAlias, workUnit);
  if (workRegion?.scope === 'outside' || workRegion?.region) {
    return workRegion;
  }

  const addressAlias = earliestAlias(address, locationAliases);
  const addressRegion = regionFromAlias(addressAlias, address);
  if (addressRegion) {
    return addressRegion;
  }

  return null;
}

export function RegionIndustryExplorer({
  expanded,
  onSelectAlumni,
}: RegionIndustryExplorerProps) {
  const [scope, setScope] = useState<RegionScope>('shandong');
  const [selectedRegion, setSelectedRegion] = useState('济南市');
  const [selectedDistrict, setSelectedDistrict] = useState('');
  const [selectedIndustry, setSelectedIndustry] = useState('');
  const [peopleKeyword, setPeopleKeyword] = useState('');
  const [allAlumni, setAllAlumni] = useState<AlumniProfile[]>([]);
  const [loading, setLoading] = useState(false);

  const activeKeyword = selectedDistrict || selectedRegion;

  useEffect(() => {
    let active = true;
    setLoading(true);
    fetchAllAlumni()
      .then((items) => {
        if (active) {
          setAllAlumni(items);
        }
      })
      .catch((error: Error) => {
        if (active) {
          message.error(error.message || '地域数据加载失败');
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    setSelectedIndustry('');
    setPeopleKeyword('');
  }, [activeKeyword]);

  const alumni = useMemo(
    () => allAlumni.filter((item) => {
      const region = resolveRegion(item);
      if (!region || region.scope !== scope || region.region !== selectedRegion) {
        return false;
      }
      return !selectedDistrict || region.district === selectedDistrict;
    }),
    [allAlumni, scope, selectedDistrict, selectedRegion],
  );

  const industries = useMemo(() => {
    const counts = new Map<string, number>();
    alumni.forEach((item) => {
      const industry = item.industry?.trim() || '未填';
      counts.set(industry, (counts.get(industry) || 0) + 1);
    });

    return [...counts.entries()]
      .map(([name, value]) => ({ name, value }))
      .sort((left, right) => right.value - left.value);
  }, [alumni]);

  const industryAlumni = useMemo(
    () =>
      selectedIndustry
        ? alumni.filter((item) => (item.industry?.trim() || '未填') === selectedIndustry)
        : alumni,
    [alumni, selectedIndustry],
  );

  const visibleAlumni = useMemo(() => {
    const keyword = peopleKeyword.trim().toLowerCase();
    if (!keyword) {
      return industryAlumni;
    }

    return industryAlumni.filter((item) =>
      [
        item.name,
        item.grade,
        item.class_name,
        item.cohort,
        item.major,
        item.industry,
        item.work_unit,
        item.position,
        item.mentor,
        item.counselor,
        item.mobile,
        item.mailing_address,
      ].some((value) => value?.toLowerCase().includes(keyword)),
    );
  }, [industryAlumni, peopleKeyword]);

  const districtOptions = scope === 'shandong' ? shandongRegions[selectedRegion] || [] : [];
  const regionOptions = scope === 'shandong' ? Object.keys(shandongRegions) : outsideRegions;
  const maxIndustryValue = industries[0]?.value || 1;

  const changeScope = (value: RegionScope) => {
    setScope(value);
    setSelectedDistrict('');
    setSelectedIndustry('');
    setSelectedRegion(value === 'shandong' ? '济南市' : '北京市');
  };

  return (
    <Spin spinning={loading}>
      <div className={`region-industry-explorer ${expanded ? 'region-industry-expanded' : ''}`}>
        <div className="region-industry-controls">
          <Segmented
            value={scope}
            options={[
              { label: '山东省内', value: 'shandong' },
              { label: '山东省外', value: 'outside' },
            ]}
            onChange={(value) => changeScope(value as RegionScope)}
          />
          <div className="region-chip-list" aria-label={scope === 'shandong' ? '山东城市' : '省外地区'}>
            {regionOptions.map((region) => (
              <button
                type="button"
                key={region}
                className={region === selectedRegion ? 'is-active' : ''}
                onClick={() => {
                  setSelectedRegion(region);
                  setSelectedDistrict('');
                }}
              >
                {region}
              </button>
            ))}
          </div>
          {districtOptions.length ? (
            <div className="region-chip-list region-district-list" aria-label={`${selectedRegion}区县`}>
              <button
                type="button"
                className={!selectedDistrict ? 'is-active' : ''}
                onClick={() => setSelectedDistrict('')}
              >
                全市
              </button>
              {districtOptions.map((district) => (
                <button
                  type="button"
                  key={district}
                  className={district === selectedDistrict ? 'is-active' : ''}
                  onClick={() => setSelectedDistrict(district)}
                >
                  {district}
                </button>
              ))}
            </div>
          ) : null}
        </div>

        <div className="region-industry-summary">
          <div>
            <EnvironmentOutlined />
            <strong>{activeKeyword}</strong>
            <span>匹配 {alumni.length} 人</span>
          </div>
        </div>

        <div className="region-industry-ranks">
          {industries.map((item) => (
            <button
              type="button"
              key={item.name}
              className={selectedIndustry === item.name ? 'is-active' : ''}
              onClick={() => {
                setSelectedIndustry((current) => (current === item.name ? '' : item.name));
                setPeopleKeyword('');
              }}
            >
              <span>{item.name}</span>
              <i>
                <b style={{ width: `${Math.max(8, (item.value / maxIndustryValue) * 100)}%` }} />
              </i>
              <strong>{item.value}</strong>
            </button>
          ))}
        </div>

        <div className="region-alumni-results">
          <div className="region-result-title">
            <strong>
              {selectedIndustry || '全部行业'}人员
              <b>{visibleAlumni.length}</b>
            </strong>
            <span>点击行业条框筛选，点击人员查看完整信息</span>
          </div>
          <div className="region-person-search">
            <Input
              allowClear
              prefix={<SearchOutlined />}
              value={peopleKeyword}
              placeholder="在当前人员中检索姓名、单位、职务、导师等..."
              onChange={(event) => setPeopleKeyword(event.target.value)}
            />
            <span>
              {peopleKeyword.trim()
                ? `匹配 ${visibleAlumni.length} 人`
                : `当前 ${industryAlumni.length} 人`}
            </span>
          </div>
          {visibleAlumni.length ? (
            <div className="region-person-list">
              {visibleAlumni.map((item) => (
                <button
                  type="button"
                  key={item.id}
                  onClick={() => onSelectAlumni(item)}
                >
                  <span>{item.name}</span>
                  <small>{item.industry || '未填'}</small>
                  <em>{item.work_unit || '未填单位'}</em>
                </button>
              ))}
            </div>
          ) : (
            <div className="region-result-empty">当前地域和行业暂无匹配人员</div>
          )}
        </div>
      </div>
    </Spin>
  );
}
