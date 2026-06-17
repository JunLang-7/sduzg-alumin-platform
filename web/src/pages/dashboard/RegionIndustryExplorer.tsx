import { useEffect, useMemo, useRef, useState } from 'react';
import {
  ArrowLeftOutlined,
  ArrowsAltOutlined,
  EnvironmentOutlined,
  GlobalOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Input, Modal, Segmented, message } from 'antd';
import * as echarts from 'echarts';
import ReactECharts from 'echarts-for-react';
import administrativeIndex from '../../assets/maps/administrative-index.json';
import chinaGeoJSON from '../../assets/maps/china.json';
import citySeats from '../../assets/maps/city-seats.json';
import shandongGeoJSON from '../../assets/maps/shandong.json';
import type { AlumniProfile } from '../../types/alumni';

interface RegionIndustryExplorerProps {
  alumni: AlumniProfile[];
  expanded: boolean;
  loading?: boolean;
  view?: 'map' | 'industry' | 'combined';
  mapMode?: MapMode;
  selectedRegion?: string;
  selectedDistrict?: string;
  onMapModeChange?: (mode: MapMode) => void;
  onRegionChange?: (region: string) => void;
  onDistrictChange?: (district: string) => void;
  onSelectAlumni: (profile: AlumniProfile) => void;
}

export type MapMode = 'shandong' | 'china';

interface AlumniRegion {
  province: string;
  city?: string;
}

interface AdministrativeRegion {
  name: string;
  province: string;
  adcode: number;
  childrenNum: number;
}

interface MapFeature {
  properties?: {
    name?: string;
    adcode?: number;
    level?: string;
    childrenNum?: number;
  };
  geometry?: {
    type?: string;
    coordinates?: unknown[];
  };
}

interface MapDatum {
  name: string;
  value: number;
  count: number;
  selected?: boolean;
  itemStyle?: {
    areaColor: string;
    shadowBlur: number;
    shadowColor: string;
    opacity: number;
    borderColor: string;
    borderWidth: number;
  };
  label?: {
    offset: [number, number];
  };
}

interface HeatLegendItem {
  color: string;
  label: string;
}

interface DrillMap {
  name: string;
  mapName: string;
  level: 'province' | 'city';
  features: MapFeature[];
  seatName?: string;
}

const CHINA_MAP_NAME = 'alumni-china';
const SHANDONG_MAP_NAME = 'alumni-shandong';
const UNKNOWN_INDUSTRY = '未填';

const shandongCityMapLoaders = {
  济南市: () => import('../../assets/maps/shandong-cities/jinan.json'),
  青岛市: () => import('../../assets/maps/shandong-cities/qingdao.json'),
  淄博市: () => import('../../assets/maps/shandong-cities/zibo.json'),
  枣庄市: () => import('../../assets/maps/shandong-cities/zaozhuang.json'),
  东营市: () => import('../../assets/maps/shandong-cities/dongying.json'),
  烟台市: () => import('../../assets/maps/shandong-cities/yantai.json'),
  潍坊市: () => import('../../assets/maps/shandong-cities/weifang.json'),
  济宁市: () => import('../../assets/maps/shandong-cities/jining.json'),
  泰安市: () => import('../../assets/maps/shandong-cities/taian.json'),
  威海市: () => import('../../assets/maps/shandong-cities/weihai.json'),
  日照市: () => import('../../assets/maps/shandong-cities/rizhao.json'),
  临沂市: () => import('../../assets/maps/shandong-cities/linyi.json'),
  德州市: () => import('../../assets/maps/shandong-cities/dezhou.json'),
  聊城市: () => import('../../assets/maps/shandong-cities/liaocheng.json'),
  滨州市: () => import('../../assets/maps/shandong-cities/binzhou.json'),
  菏泽市: () => import('../../assets/maps/shandong-cities/heze.json'),
} as const;

const provinceMapLoaders = import.meta.glob<{ default: { features: MapFeature[] } }>(
  '../../assets/maps/provinces/*.json',
);
const cityMapLoaders = import.meta.glob<{ default: { features: MapFeature[] } }>(
  '../../assets/maps/cities/*.json',
);

function polygonAreaEstimate(polygon: unknown) {
  if (!Array.isArray(polygon) || !Array.isArray(polygon[0])) return 0;
  const ring = polygon[0] as number[][];
  const xs = ring.map((point) => point[0]);
  const ys = ring.map((point) => point[1]);
  return (Math.max(...xs) - Math.min(...xs)) * (Math.max(...ys) - Math.min(...ys));
}

const chinaProvinceGeoJSON = {
  ...chinaGeoJSON,
  features: chinaGeoJSON.features
    .filter(
      (feature) =>
        Boolean(feature.properties?.name) &&
        feature.properties?.name !== '香港特别行政区',
    )
    .map((feature) => {
      if (feature.geometry.type !== 'MultiPolygon') return feature;
      return {
        ...feature,
        geometry: {
          ...feature.geometry,
          coordinates: feature.geometry.coordinates.filter(
            (polygon) => polygonAreaEstimate(polygon) >= 0.05,
          ),
        },
      };
    }),
};

const provinceLabelNames = new Map<string, string>(
  chinaProvinceGeoJSON.features.map((feature) => {
    const name = feature.properties?.name || '';
    const shortName = name.replace(
      /特别行政区|维吾尔自治区|壮族自治区|回族自治区|自治区|省|市$/u,
      '',
    );
    return [name, shortName || name];
  }),
);

function formatChinaProvinceLabel(params: { name?: string }) {
  const name = params.name || '';
  return provinceLabelNames.get(name) || name;
}

echarts.registerMap(
  CHINA_MAP_NAME,
  chinaProvinceGeoJSON as unknown as Parameters<typeof echarts.registerMap>[1],
);
echarts.registerMap(
  SHANDONG_MAP_NAME,
  shandongGeoJSON as unknown as Parameters<typeof echarts.registerMap>[1],
);

const shandongCities = [
  '济南市',
  '青岛市',
  '淄博市',
  '枣庄市',
  '东营市',
  '烟台市',
  '潍坊市',
  '济宁市',
  '泰安市',
  '威海市',
  '日照市',
  '临沂市',
  '德州市',
  '聊城市',
  '滨州市',
  '菏泽市',
];

const provinceNames = (chinaProvinceGeoJSON.features as MapFeature[])
  .map((feature) => feature.properties?.name || '')
  .filter(Boolean);

const provinceLabelOffsets: Record<string, [number, number]> = {
  河北省: [-12, 10],
  陕西省: [20, 0],
};

const provinceAliases = new Map<string, string>(
  provinceNames.flatMap((province) => {
    const shortName = province.replace(
      /特别行政区|维吾尔自治区|壮族自治区|回族自治区|自治区|省|市$/u,
      '',
    );
    return [
      [province, province],
      [shortName, province],
    ];
  }),
);

provinceAliases.set('内蒙', '内蒙古自治区');
provinceAliases.set('广西', '广西壮族自治区');
provinceAliases.set('宁夏', '宁夏回族自治区');
provinceAliases.set('新疆', '新疆维吾尔自治区');
provinceAliases.set('西藏', '西藏自治区');
provinceAliases.set('香港', '香港特别行政区');
provinceAliases.set('澳门', '澳门特别行政区');

const ethnicGroups =
  '蒙古族|回族|藏族|维吾尔族|哈萨克族|柯尔克孜族|苗族|彝族|壮族|布依族|侗族|瑶族|白族|傣族|傈僳族|佤族|拉祜族|水族|羌族|土家族|朝鲜族|景颇族|哈尼族';

function administrativeNameAliases(name: string) {
  const aliases = new Set([name]);
  const shortName = name
    .replace(new RegExp(`(?:${ethnicGroups})+自治州$`, 'u'), '')
    .replace(/特别行政区|自治州|地区|市|盟|区|县$/u, '');
  if (shortName) {
    aliases.add(shortName);
    if (name.endsWith('自治州')) aliases.add(`${shortName}州`);
  }
  return [...aliases];
}

const cityAliases = new Map<string, AlumniRegion>();
(administrativeIndex as AdministrativeRegion[]).forEach((region) => {
  const isPrefecture = region.childrenNum > 0 || /市|自治州|地区|盟$/u.test(region.name);
  if (!isPrefecture) return;
  administrativeNameAliases(region.name).forEach((alias) => {
    cityAliases.set(alias, {
      province: region.province,
      city: region.name,
    });
  });
});

const locationAliases = [...new Set([
  '山东省',
  '山东',
  ...cityAliases.keys(),
  ...provinceAliases.keys(),
])].sort((left, right) => right.length - left.length);

const branchSuffixes = ['分公司', '分行', '支行', '办事处', '联络处', '营业部', '项目部'];

function normalize(value?: string) {
  return value?.replace(/\s+/gu, '').trim() || '';
}

function findBranchLocation(workUnit: string) {
  for (const alias of locationAliases) {
    const aliasIndex = workUnit.indexOf(alias);
    if (aliasIndex < 0) continue;

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

function regionFromAlias(alias: string, source: string): AlumniRegion | null {
  const city = cityAliases.get(alias);
  if (city) {
    return city;
  }

  if (alias === '山东' || alias === '山东省') {
    const cityAlias = earliestAlias(source, cityAliases.keys());
    return {
      province: '山东省',
      city: cityAliases.get(cityAlias)?.city,
    };
  }

  const province = provinceAliases.get(alias);
  return province ? { province } : null;
}

function resolveRegion(item: AlumniProfile): AlumniRegion | null {
  const workUnit = normalize(item.work_unit).replace(
    /台湾(?:工作办公室|事务办公室|工作办|事务办)/gu,
    '',
  );
  const address = normalize(item.mailing_address);
  const branchAlias = findBranchLocation(workUnit);
  if (branchAlias) {
    const branchRegion = regionFromAlias(branchAlias, workUnit);
    if (branchRegion?.province !== '山东省' || branchRegion.city) {
      return branchRegion;
    }

    const addressCity = cityAliases.get(earliestAlias(address, cityAliases.keys()));
    return {
      ...branchRegion,
      city:
        addressCity?.province === branchRegion.province
          ? addressCity.city
          : branchRegion.city,
    };
  }

  const workAlias = earliestAlias(workUnit, locationAliases);
  const workRegion = regionFromAlias(workAlias, workUnit);
  if (workRegion) {
    if (workRegion.province !== '山东省' || workRegion.city) {
      return workRegion;
    }

    const addressCity = cityAliases.get(earliestAlias(address, cityAliases.keys()));
    return {
      ...workRegion,
      city:
        addressCity?.province === workRegion.province
          ? addressCity.city
          : workRegion.city,
    };
  }

  const addressAlias = earliestAlias(address, locationAliases);
  return regionFromAlias(addressAlias, address);
}

function profileLocationText(item: AlumniProfile) {
  return normalize(`${item.work_unit || ''}${item.mailing_address || ''}`);
}

function locationTextMatches(text: string, location: string) {
  return administrativeNameAliases(location).some((alias) => text.includes(alias));
}

function profileMatchesLocation(item: AlumniProfile, location: string) {
  const workUnit = normalize(item.work_unit).replace(
    /台湾(?:工作办公室|事务办公室|工作办|事务办)/gu,
    '',
  );
  if (locationTextMatches(workUnit, location)) return true;
  return locationTextMatches(normalize(item.mailing_address), location);
}

function administrativeStem(name: string) {
  return name.replace(
    /特别行政区|维吾尔自治区|壮族自治区|回族自治区|自治区|自治州|地区|市|盟|自治县|区|县$/u,
    '',
  );
}

function matchDistrictName(
  text: string,
  districtNames: string[],
  parentCity: string,
) {
  if (!text) return '';
  const orderedNames = [...districtNames].sort(
    (left, right) => right.length - left.length,
  );

  const fullNameMatch = orderedNames.find((name) => text.includes(name));
  if (fullNameMatch) return fullNameMatch;

  const parentStem = administrativeStem(parentCity);
  return (
    orderedNames.find((name) => {
      const shortName = administrativeStem(name);
      return (
        shortName.length >= 2 &&
        shortName !== parentStem &&
        text.includes(shortName)
      );
    }) || ''
  );
}

function assignProfileToDistrict(
  item: AlumniProfile,
  districtNames: string[],
  fallbackDistrict: string,
  parentCity: string,
) {
  const workUnit = normalize(item.work_unit);
  const address = normalize(item.mailing_address);
  return (
    matchDistrictName(workUnit, districtNames, parentCity) ||
    matchDistrictName(address, districtNames, parentCity) ||
    fallbackDistrict
  );
}

function findMapLoader<T>(
  loaders: Record<string, () => Promise<T>>,
  adcode: number,
) {
  const suffix = `/${adcode}.json`;
  return Object.entries(loaders).find(([path]) =>
    path.replace(/\\/gu, '/').endsWith(suffix),
  )?.[1];
}

function createAdaptiveHeatValueMap(counts: number[]) {
  const positiveCounts = [...new Set(counts.filter((count) => count > 0))]
    .sort((left, right) => left - right);
  const maxCount = positiveCounts[positiveCounts.length - 1] || 1;
  const values = new Map<number, number>();

  positiveCounts.forEach((count, index) => {
    const rank =
      positiveCounts.length === 1
        ? 0.72
        : index / (positiveCounts.length - 1);
    const magnitude = Math.log1p(count) / Math.log1p(maxCount);
    values.set(count, 0.1 + (rank * 0.78 + magnitude * 0.22) * 0.9);
  });

  return values;
}

function createAdaptiveHeatValues(counts: number[]) {
  const values = createAdaptiveHeatValueMap(counts);
  return counts.map((count) => (count > 0 ? values.get(count) || 0.1 : -1));
}

const starMapColors = [
  '#18386f',
  '#2455a6',
  '#2478c4',
  '#188fae',
  '#159b8b',
  '#3b9b68',
  '#858f3e',
  '#b47a32',
  '#c65368',
];

function parseHexColor(color: string) {
  return [
    Number.parseInt(color.slice(1, 3), 16),
    Number.parseInt(color.slice(3, 5), 16),
    Number.parseInt(color.slice(5, 7), 16),
  ];
}

function interpolateMapColor(value: number) {
  const position = Math.max(0, Math.min(1, value)) * (starMapColors.length - 1);
  const leftIndex = Math.floor(position);
  const rightIndex = Math.min(starMapColors.length - 1, leftIndex + 1);
  const ratio = position - leftIndex;
  const left = parseHexColor(starMapColors[leftIndex]);
  const right = parseHexColor(starMapColors[rightIndex]);
  return left.map((channel, index) =>
    Math.round(channel + (right[index] - channel) * ratio),
  );
}

function rgbColor(channels: number[], brightness = 1) {
  const values = channels.map((channel) =>
    Math.max(0, Math.min(255, Math.round(channel * brightness))),
  );
  return `rgb(${values[0]}, ${values[1]}, ${values[2]})`;
}

function formatLegendCount(count: number) {
  return count.toLocaleString('zh-CN');
}

function createHeatLegend(counts: number[]): HeatLegendItem[] {
  const positiveCounts = [...new Set(counts.filter((count) => count > 0))]
    .sort((left, right) => left - right);
  const valueMap = createAdaptiveHeatValueMap(counts);
  const items: HeatLegendItem[] = [
    {
      color: '#09275f',
      label: '0 人',
    },
  ];

  if (!positiveCounts.length) return items;

  const binCount = Math.min(5, positiveCounts.length);
  for (let index = 0; index < binCount; index += 1) {
    const startIndex = Math.floor((index * positiveCounts.length) / binCount);
    const endIndex =
      Math.floor(((index + 1) * positiveCounts.length) / binCount) - 1;
    const min = positiveCounts[startIndex];
    const max = positiveCounts[Math.max(startIndex, endIndex)];
    const value = valueMap.get(max) || 0.1;
    items.push({
      color: rgbColor(interpolateMapColor(value)),
      label:
        min === max
          ? `${formatLegendCount(min)} 人`
          : `${formatLegendCount(min)}-${formatLegendCount(max)} 人`,
    });
  }

  return items;
}

function MapHeatLegend({ items }: { items: HeatLegendItem[] }) {
  return (
    <div className="region-map-legend" aria-label="地图人数颜色图例">
      <strong>人数图例</strong>
      {items.map((item) => (
        <span key={`${item.color}-${item.label}`}>
          <i style={{ backgroundColor: item.color }} />
          {item.label}
        </span>
      ))}
    </div>
  );
}

function starGlowStyle(
  value: number,
  pulse: number,
) {
  if (value < 0) {
    return {
      areaColor: '#09275f',
      shadowBlur: 0,
      shadowColor: 'transparent',
      opacity: 1,
      borderColor: 'rgba(82, 154, 210, 0.58)',
      borderWidth: 0.7,
    };
  }
  const easedPulse = Math.pow(pulse, 1.35);
  const intensity = 0.32 + value * 0.68;
  const alpha = (0.2 + easedPulse * 0.72) * intensity;
  const warmGlow = value >= 0.68;
  const baseColor = interpolateMapColor(value);
  const pulseDarkness = 1 - easedPulse * (0.38 + value * 0.12);
  return {
    areaColor: rgbColor(baseColor, pulseDarkness),
    shadowBlur: Math.round(
      3 + value * 12 + easedPulse * (10 + value * 22),
    ),
    shadowColor: warmGlow
      ? `rgba(255, 224, 105, ${alpha})`
      : `rgba(76, 220, 255, ${alpha})`,
    opacity: 1,
    borderColor: warmGlow
      ? `rgba(255, 239, 155, ${0.48 + easedPulse * 0.5})`
      : `rgba(112, 229, 255, ${0.42 + easedPulse * 0.54})`,
    borderWidth: 0.75 + easedPulse * (0.75 + value * 0.55),
  };
}

function updateChartPulse(
  chart: echarts.ECharts | null,
  data: MapDatum[],
  activePulses: Map<number, number>,
) {
  if (!chart || chart.isDisposed()) return;
  chart.setOption(
    {
      series: [
        {
          data: data.map((item, index) => ({
            ...item,
            itemStyle: starGlowStyle(item.value, activePulses.get(index) || 0),
          })),
        },
      ],
    },
    {
      notMerge: false,
      lazyUpdate: true,
      silent: true,
    },
  );
}

export function RegionIndustryExplorer({
  alumni: allAlumni,
  expanded,
  loading = false,
  view = 'combined',
  mapMode: controlledMapMode,
  selectedRegion: controlledSelectedRegion,
  selectedDistrict: controlledSelectedDistrict,
  onMapModeChange,
  onRegionChange,
  onDistrictChange,
  onSelectAlumni,
}: RegionIndustryExplorerProps) {
  const [internalMapMode, setInternalMapMode] = useState<MapMode>('shandong');
  const [internalSelectedRegion, setInternalSelectedRegion] = useState('');
  const [selectedIndustry, setSelectedIndustry] = useState('');
  const [peopleKeyword, setPeopleKeyword] = useState('');
  const [mapModalOpen, setMapModalOpen] = useState(false);
  const [drillMap, setDrillMap] = useState<DrillMap | null>(null);
  const [drillParent, setDrillParent] = useState<DrillMap | null>(null);
  const [drillLoading, setDrillLoading] = useState(false);
  const [internalSelectedDistrict, setInternalSelectedDistrict] = useState('');
  const [reducedMotion, setReducedMotion] = useState(false);
  const mapMode = controlledMapMode ?? internalMapMode;
  const selectedRegion = controlledSelectedRegion ?? internalSelectedRegion;
  const selectedDistrict = controlledSelectedDistrict ?? internalSelectedDistrict;
  const clearMainMapSelectionRef = useRef<() => void>(() => undefined);
  const clearDetailMapSelectionRef = useRef<() => void>(() => undefined);
  const mainChartRef = useRef<echarts.ECharts | null>(null);
  const detailChartRef = useRef<echarts.ECharts | null>(null);
  const mainAreaClickAtRef = useRef(0);
  const detailAreaClickAtRef = useRef(0);
  const mainInteractionUntilRef = useRef(0);
  const detailInteractionUntilRef = useRef(0);

  useEffect(() => {
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    const updatePreference = () => setReducedMotion(mediaQuery.matches);
    updatePreference();
    mediaQuery.addEventListener?.('change', updatePreference);
    return () => mediaQuery.removeEventListener?.('change', updatePreference);
  }, []);

  const updateMapMode = (mode: MapMode) => {
    setInternalMapMode(mode);
    onMapModeChange?.(mode);
  };

  const updateSelectedRegion = (region: string) => {
    setInternalSelectedRegion(region);
    onRegionChange?.(region);
  };

  const updateSelectedDistrict = (district: string) => {
    setInternalSelectedDistrict(district);
    onDistrictChange?.(district);
  };

  clearMainMapSelectionRef.current = () => {
    updateSelectedRegion('');
    updateSelectedDistrict('');
    setSelectedIndustry('');
    setPeopleKeyword('');
  };
  clearDetailMapSelectionRef.current = () => {
    if (drillMap) {
      updateSelectedDistrict('');
    } else {
      updateSelectedRegion('');
      updateSelectedDistrict('');
    }
    setSelectedIndustry('');
    setPeopleKeyword('');
  };

  const bindBlankMapClick = (
    chart: echarts.ECharts,
    callbackRef: { current: () => void },
    areaClickAtRef: { current: number },
    interactionUntilRef: { current: number },
  ) => {
    const renderer = chart.getZr();
    const marker = renderer as typeof renderer & {
      __alumniBlankClickBound?: boolean;
    };
    if (marker.__alumniBlankClickBound) return;
    marker.__alumniBlankClickBound = true;
    renderer.on('mousedown', () => {
      interactionUntilRef.current = Date.now() + 900;
    });
    renderer.on('mouseup', () => {
      interactionUntilRef.current = Math.max(
        interactionUntilRef.current,
        Date.now() + 520,
      );
    });
    renderer.on('click', (event) => {
      if (!event.target) {
        window.setTimeout(() => {
          if (Date.now() - areaClickAtRef.current < 320) return;
          chart.dispatchAction({
            type: 'unselect',
            seriesIndex: 0,
          });
          callbackRef.current();
        }, 0);
      }
    });
  };

  const alumniWithRegion = useMemo(
    () =>
      allAlumni.map((profile) => ({
        profile,
        region: resolveRegion(profile),
      })),
    [allAlumni],
  );

  const regionCounts = useMemo(() => {
    const counts = new Map<string, number>();
    alumniWithRegion.forEach(({ region }) => {
      const name = mapMode === 'shandong'
        ? region?.province === '山东省'
          ? region.city
          : undefined
        : region?.province;
      if (name) {
        counts.set(name, (counts.get(name) || 0) + 1);
      }
    });
    return counts;
  }, [alumniWithRegion, mapMode]);

  const mapRegions = mapMode === 'shandong' ? shandongCities : provinceNames;
  const mapRegionCounts = useMemo(
    () => mapRegions.map((name) => regionCounts.get(name) || 0),
    [mapRegions, regionCounts],
  );
  const mapHeatValues = useMemo(
    () => createAdaptiveHeatValues(mapRegionCounts),
    [mapRegionCounts],
  );
  const mapData = useMemo(
    () =>
      mapRegions.map((name, index): MapDatum => {
        const count = mapRegionCounts[index];
        const value = mapHeatValues[index];
        return {
          name,
          count,
          value,
          selected: name === selectedRegion,
          itemStyle: starGlowStyle(value, 0),
          label:
            mapMode === 'china' && provinceLabelOffsets[name]
              ? { offset: provinceLabelOffsets[name] }
              : undefined,
        };
      }),
    [
      mapHeatValues,
      mapMode,
      mapRegionCounts,
      mapRegions,
      selectedRegion,
    ],
  );

  const baseRegionAlumni = useMemo(
    () => {
      const parentCity = drillMap?.level === 'city' ? drillMap.name : '';
      return alumniWithRegion
        .filter(({ profile, region }) => {
          const inSelectedRegion =
            mapMode === 'shandong'
              ? region?.province === '山东省' && region.city === selectedRegion
              : region?.province === selectedRegion;
          if (!inSelectedRegion) return false;

          if (
            parentCity &&
            region?.city !== parentCity &&
            !profileMatchesLocation(profile, parentCity)
          ) {
            return false;
          }

          return true;
        })
        .map(({ profile }) => profile);
    },
    [
      alumniWithRegion,
      drillMap,
      mapMode,
      selectedRegion,
    ],
  );

  const cityDistrictAssignments = useMemo(() => {
    if (drillMap?.level !== 'city') return new Map<number, string>();
    const districtNames = drillMap.features
      .map((feature) => feature.properties?.name || '')
      .filter(Boolean);
    const fallbackDistrict = drillMap.seatName || districtNames[0] || '';
    return new Map(
      baseRegionAlumni.map((profile) => [
        profile.id,
        assignProfileToDistrict(
          profile,
          districtNames,
          fallbackDistrict,
          drillMap.name,
        ),
      ]),
    );
  }, [baseRegionAlumni, drillMap]);

  const regionAlumni = useMemo(() => {
    if (!selectedDistrict) return baseRegionAlumni;
    if (drillMap?.level === 'city') {
      if (selectedDistrict === drillMap.name) return baseRegionAlumni;
      return baseRegionAlumni.filter(
        (profile) => cityDistrictAssignments.get(profile.id) === selectedDistrict,
      );
    }
    return baseRegionAlumni.filter((profile) =>
      profileMatchesLocation(profile, selectedDistrict),
    );
  }, [
    baseRegionAlumni,
    cityDistrictAssignments,
    drillMap,
    selectedDistrict,
  ]);

  const industries = useMemo(() => {
    const counts = new Map<string, number>();
    regionAlumni.forEach((item) => {
      const industry = item.industry?.trim() || UNKNOWN_INDUSTRY;
      counts.set(industry, (counts.get(industry) || 0) + 1);
    });
    return [...counts.entries()]
      .map(([name, value]) => ({ name, value }))
      .sort((left, right) => right.value - left.value);
  }, [regionAlumni]);

  const industryAlumni = useMemo(
    () =>
      selectedIndustry
        ? regionAlumni.filter(
            (item) => (item.industry?.trim() || UNKNOWN_INDUSTRY) === selectedIndustry,
          )
        : regionAlumni,
    [regionAlumni, selectedIndustry],
  );

  const visibleAlumni = useMemo(() => {
    const keyword = peopleKeyword.trim().toLowerCase();
    if (!keyword) return industryAlumni;

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

  const maxIndustryValue = industries[0]?.value || 1;

  const openCityDetail = async (city: string, adcode?: number, parent?: DrillMap | null) => {
    setDrillLoading(true);
    try {
      const resolvedAdcode =
        adcode ||
        (shandongGeoJSON.features as MapFeature[]).find(
          (feature) => feature.properties?.name === city,
        )?.properties?.adcode;
      const loader = resolvedAdcode
        ? findMapLoader(cityMapLoaders, resolvedAdcode)
        : undefined;
      const fallbackLoader = shandongCityMapLoaders[city as keyof typeof shandongCityMapLoaders];
      const geoJSON = loader
        ? (await loader()).default
        : fallbackLoader
          ? (await fallbackLoader()).default
          : null;
      if (!geoJSON) {
        message.info('该地区暂无区县地图');
        return;
      }
      const mapName = `alumni-city-${resolvedAdcode || city}`;
      echarts.registerMap(
        mapName,
        geoJSON as unknown as Parameters<typeof echarts.registerMap>[1],
      );
      if (!parent || mapMode === 'shandong') {
        updateSelectedRegion(city);
      }
      updateSelectedDistrict('');
      setDrillParent(parent || null);
      setDrillMap({
        name: city,
        mapName,
        level: 'city',
        features: geoJSON.features as MapFeature[],
        seatName:
          (citySeats as Record<string, { city: string; district: string }>)[
            String(resolvedAdcode)
          ]?.district || geoJSON.features[0]?.properties?.name,
      });
      setMapModalOpen(true);
    } catch {
      message.error('该城市地图加载失败，请稍后重试');
    } finally {
      setDrillLoading(false);
    }
  };

  const openProvinceDetail = async (province: string) => {
    const feature = (chinaProvinceGeoJSON.features as MapFeature[]).find(
      (item) => item.properties?.name === province,
    );
    const adcode = feature?.properties?.adcode;
    if (!adcode) return;
    setDrillLoading(true);
    try {
      const loader = findMapLoader(provinceMapLoaders, adcode);
      if (!loader) {
        updateSelectedRegion(province);
        message.info('该地区暂无下一级地图');
        return;
      }
      const geoJSON = (await loader()).default;
      const mapName = `alumni-province-${adcode}`;
      echarts.registerMap(
        mapName,
        geoJSON as unknown as Parameters<typeof echarts.registerMap>[1],
      );
      updateSelectedDistrict('');
      setDrillParent(null);
      setDrillMap({
        name: province,
        mapName,
        level: 'province',
        features: geoJSON.features,
      });
      setMapModalOpen(true);
    } catch {
      message.error('该省地图加载失败，请检查网络后重试');
    } finally {
      setDrillLoading(false);
    }
  };

  const drillNames = useMemo(
    () =>
      drillMap
        ? drillMap.features
            .map((feature) => feature.properties?.name || '')
            .filter(Boolean)
        : [],
    [drillMap],
  );

  const drillCounts = useMemo(
    () =>
      drillNames.map((name) => {
      if (drillMap?.level === 'city') {
        return [...cityDistrictAssignments.values()].filter(
          (district) => district === name,
        ).length;
      }
      return alumniWithRegion.filter(({ profile, region }) => {
        if (drillMap?.level === 'province') {
          return (
            region?.province === selectedRegion &&
            (region.city === name || profileMatchesLocation(profile, name))
          );
        }
        return profileMatchesLocation(profile, name);
      }).length;
      }),
    [
      alumniWithRegion,
      cityDistrictAssignments,
      drillMap,
      drillNames,
      selectedRegion,
    ],
  );

  const drillData = useMemo(() => {
    const heatValues = createAdaptiveHeatValues(drillCounts);
    return drillNames.map((name, index): MapDatum => ({
      name,
      count: drillCounts[index],
      value: heatValues[index],
      itemStyle: starGlowStyle(heatValues[index], 0),
      selected: name === selectedDistrict,
    }));
  }, [drillCounts, drillNames, selectedDistrict]);

  const detailLegendItems = useMemo(
    () => createHeatLegend(drillMap ? drillCounts : mapRegionCounts),
    [drillCounts, drillMap, mapRegionCounts],
  );

  useEffect(() => {
    if (view === 'industry') return undefined;

    const detailAnimationData = drillMap ? drillData : mapData;
    const mainActiveIndexes = mapData
      .map((item, index) => (item.count > 0 ? index : -1))
      .filter((index) => index >= 0);
    const detailActiveIndexes = detailAnimationData
      .map((item, index) => (item.count > 0 ? index : -1))
      .filter((index) => index >= 0);

    if (reducedMotion) {
      updateChartPulse(mainChartRef.current, mapData, new Map());
      updateChartPulse(
        detailChartRef.current,
        detailAnimationData,
        new Map(),
      );
      return undefined;
    }

    const pulseFrames = [0, 0.38, 0.82, 1, 0.7, 0.28, 0];
    const starOffsets = [0, 1, 3, 4, 6];
    const starStrengths = [1, 0.84, 0.72, 0.58, 0.46];
    let frameIndex = 0;
    let mainCursor = 0;
    let detailCursor = 0;

    const createStarPulses = (indexes: number[], cursor: number) => {
      const pulses = new Map<number, number>();
      if (!indexes.length) return pulses;

      starOffsets.forEach((offset, starIndex) => {
        const activeIndex = indexes[(cursor + starIndex) % indexes.length];
        const pulse =
          pulseFrames[(frameIndex + offset) % pulseFrames.length] *
          starStrengths[starIndex];
        pulses.set(activeIndex, Math.max(pulses.get(activeIndex) || 0, pulse));
      });
      return pulses;
    };

    const renderPulse = () => {
      const now = Date.now();
      const mainPulses = createStarPulses(mainActiveIndexes, mainCursor);
      const detailPulses = createStarPulses(
        detailActiveIndexes,
        detailCursor,
      );

      if (now >= mainInteractionUntilRef.current) {
        updateChartPulse(mainChartRef.current, mapData, mainPulses);
      }
      if (now >= detailInteractionUntilRef.current) {
        updateChartPulse(
          detailChartRef.current,
          detailAnimationData,
          detailPulses,
        );
      }

      frameIndex += 1;
      if (frameIndex >= pulseFrames.length) {
        frameIndex = 0;
        mainCursor += 5;
        detailCursor += 5;
      }
    };

    renderPulse();
    const timer = window.setInterval(renderPulse, 360);
    return () => window.clearInterval(timer);
  }, [drillData, drillMap, mapData, reducedMotion, view]);

  useEffect(() => {
    setSelectedIndustry('');
    setPeopleKeyword('');
    updateSelectedDistrict('');
  }, [selectedRegion]);

  const changeMapMode = (value: MapMode) => {
    updateMapMode(value);
    updateSelectedRegion('');
    setSelectedIndustry('');
    setPeopleKeyword('');
    setDrillMap(null);
    setDrillParent(null);
    updateSelectedDistrict('');
  };

  const mapOption = useMemo(
    () => ({
      animationDurationUpdate: 480,
      animationEasingUpdate: 'sinusoidalInOut',
      tooltip: {
        show: false,
      },
      visualMap: {
        min: 0,
        max: 1,
        show: false,
        calculable: false,
        orient: 'horizontal',
        left: 'center',
        bottom: 0,
        itemWidth: expanded ? 90 : 66,
        itemHeight: 7,
        text: ['多', '少'],
        textGap: 6,
        textStyle: {
          color: 'rgba(218, 242, 255, 0.72)',
          fontSize: 10,
        },
        inRange: {
          color: starMapColors,
        },
        outOfRange: {
          color: ['#09275f'],
        },
      },
      series: [
        {
          type: 'map',
          map: mapMode === 'shandong' ? SHANDONG_MAP_NAME : CHINA_MAP_NAME,
          data: mapData,
          selectedMode: 'single',
          roam: expanded,
          scaleLimit: { min: 1, max: 4 },
          layoutCenter: ['50%', mapMode === 'shandong' ? '49%' : '56%'],
          layoutSize: mapMode === 'shandong' ? (view === 'map' ? '106%' : '94%') : '106%',
          label: {
            show: true,
            formatter:
              mapMode === 'china'
                ? formatChinaProvinceLabel
                : undefined,
            color: 'rgba(235, 249, 255, 0.88)',
            fontSize:
              mapMode === 'china'
                ? expanded
                  ? 8
                  : 6
                : expanded
                  ? 10
                  : 8,
            fontWeight: mapMode === 'china' ? 600 : 400,
            align: 'center',
            verticalAlign: 'middle',
            textBorderColor: 'rgba(2, 12, 35, 0.96)',
            textBorderWidth: 3,
            textShadowColor: 'rgba(0, 0, 0, 0.96)',
            textShadowBlur: 5,
          },
          itemStyle: {
            areaColor: '#09275f',
            borderColor: 'rgba(125, 220, 255, 0.92)',
            borderWidth: mapMode === 'shandong' ? 1 : 0.7,
          },
          emphasis: {
            label: {
              show: true,
              color: '#ffffff',
              fontWeight: 800,
              textBorderColor: 'rgba(2, 12, 35, 0.98)',
              textBorderWidth: 4,
              textShadowColor: 'rgba(0, 0, 0, 0.9)',
              textShadowBlur: 6,
            },
            itemStyle: {
              borderColor: '#fff7c2',
              borderWidth: 1.8,
              shadowBlur: 26,
              shadowColor: 'rgba(185, 238, 255, 0.86)',
            },
          },
          select: {
            label: {
              show: true,
              color: '#ffffff',
              fontWeight: 900,
              textBorderColor: 'rgba(2, 12, 35, 0.98)',
              textBorderWidth: 4,
              textShadowColor: 'rgba(0, 0, 0, 0.96)',
              textShadowBlur: 6,
            },
            itemStyle: {
              borderColor: '#fff5b5',
              borderWidth: 2,
              shadowBlur: 28,
              shadowColor: 'rgba(255, 220, 112, 0.88)',
            },
          },
        },
      ],
    }),
    [expanded, mapData, mapMode, view],
  );

  const detailMapOption = useMemo(() => {
    const data = drillMap ? drillData : mapData;
    return {
      ...mapOption,
      visualMap: {
        ...mapOption.visualMap,
        max: 1,
      },
      series: [
        {
          ...mapOption.series[0],
          map: drillMap?.mapName || (mapMode === 'shandong' ? SHANDONG_MAP_NAME : CHINA_MAP_NAME),
          data,
          roam: true,
          layoutCenter: ['50%', drillMap ? '51%' : mapMode === 'china' ? '56%' : '52%'],
          layoutSize:
            drillMap?.level === 'city'
              ? '88%'
              : drillMap
                ? '96%'
                : mapMode === 'china'
                  ? '108%'
                  : '98%',
          label: {
            show: true,
            formatter:
              !drillMap && mapMode === 'china'
                ? formatChinaProvinceLabel
                : undefined,
            color: '#eaf8ff',
            fontSize:
              !drillMap && mapMode === 'china'
                ? 10
                : drillMap?.level === 'province'
                  ? 10
                  : 11,
            fontWeight: 700,
            align: 'center',
            verticalAlign: 'middle',
            textBorderColor: 'rgba(2, 12, 35, 0.96)',
            textBorderWidth: 3,
            textShadowColor: 'rgba(0, 0, 0, 0.96)',
            textShadowBlur: 5,
          },
          itemStyle: {
            ...mapOption.series[0].itemStyle,
            borderWidth: 1,
          },
        },
      ],
    };
  }, [drillData, drillMap, mapData, mapMode, mapOption]);

  const mapEvents = useMemo(
    () => ({
      click: (params: { name?: string }) => {
        if (!params.name || !mapRegions.includes(params.name)) return;
        mainAreaClickAtRef.current = Date.now();
        mainInteractionUntilRef.current = Date.now() + 900;
        updateSelectedRegion(params.name);
      },
      dblclick: async (params: { name?: string }) => {
        if (!params.name || !mapRegions.includes(params.name)) return;
        mainAreaClickAtRef.current = Date.now();
        mainInteractionUntilRef.current = Date.now() + 900;
        updateSelectedRegion(params.name);
        if (mapMode === 'shandong') {
          await openCityDetail(params.name);
        } else {
          await openProvinceDetail(params.name);
        }
      },
    }),
    [mapMode, mapRegions],
  );

  const detailMapEvents = useMemo(
    () => ({
      click: (params: { name?: string }) => {
        if (!params.name) return;
        detailAreaClickAtRef.current = Date.now();
        detailInteractionUntilRef.current = Date.now() + 900;
        if (!drillMap) {
          updateSelectedRegion(params.name);
          return;
        }
        if (drillMap.level === 'province') {
          updateSelectedDistrict(
            selectedDistrict === params.name ? '' : params.name,
          );
          return;
        }
        if (drillMap.level === 'city' && drillNames.includes(params.name)) {
          updateSelectedDistrict(
            selectedDistrict === params.name ? '' : params.name,
          );
        }
      },
      dblclick: async (params: { name?: string }) => {
        if (!params.name) return;
        detailAreaClickAtRef.current = Date.now();
        detailInteractionUntilRef.current = Date.now() + 900;
        if (!drillMap) {
          if (mapMode === 'shandong') {
            await openCityDetail(params.name);
          } else {
            updateSelectedRegion(params.name);
            await openProvinceDetail(params.name);
          }
          return;
        }
        if (drillMap.level !== 'province') return;
        const feature = drillMap.features.find(
          (item) => item.properties?.name === params.name,
        );
        const adcode = feature?.properties?.adcode;
        if (adcode && feature.properties?.childrenNum) {
          await openCityDetail(params.name, adcode, drillMap);
        }
      },
    }),
    [drillMap, drillNames, mapMode, selectedDistrict],
  );

  return (
    <div className={`region-industry-explorer region-view-${view} ${expanded ? 'region-industry-expanded' : ''}`}>
      {view !== 'industry' ? (
        <div className="region-map-toolbar">
          <Segmented
            value={mapMode}
            options={[
              {
                label: (
                  <span className="region-map-mode-option">
                    <EnvironmentOutlined />
                    山东省
                  </span>
                ),
                value: 'shandong',
              },
              {
                label: (
                  <span className="region-map-mode-option">
                    <GlobalOutlined />
                    全国
                  </span>
                ),
                value: 'china',
              },
            ]}
            onChange={(value) => changeMapMode(value as MapMode)}
          />
          <div className="region-map-toolbar-actions">
            <div className="region-industry-summary">
              <EnvironmentOutlined />
              <strong>{selectedRegion || '未选择区域'}</strong>
              <span>{regionAlumni.length} 人</span>
              {loading ? <em>完整数据加载中</em> : null}
            </div>
            <Button
              type="text"
              icon={<ArrowsAltOutlined />}
              className="region-map-detail-button"
              loading={drillLoading}
              aria-label="放大地图"
              title="放大地图"
              onClick={() => {
                setDrillMap(null);
                setDrillParent(null);
                updateSelectedDistrict('');
                setMapModalOpen(true);
              }}
            />
          </div>
        </div>
      ) : null}

      {view !== 'industry' ? (
      <div className={`region-map-insights ${view === 'map' ? 'region-map-only' : ''}`}>
        <div className="region-map-canvas">
          <ReactECharts
            key={`${mapMode}-${expanded}`}
            option={mapOption}
            onEvents={mapEvents}
            onChartReady={(chart) => {
              mainChartRef.current = chart;
              bindBlankMapClick(
                chart,
                clearMainMapSelectionRef,
                mainAreaClickAtRef,
                mainInteractionUntilRef,
              );
            }}
            notMerge
            lazyUpdate
            style={{ width: '100%', height: '100%' }}
          />
        </div>

        {view === 'combined' ? (
        <div className="region-industry-ranks">
          <header>
            <strong>行业分布</strong>
            <span>{selectedIndustry || '全部行业'}</span>
          </header>
          <div>
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
        </div>
        ) : null}
      </div>
      ) : null}

      {view === 'industry' ? (
        <div className="region-industry-ranks">
          <header>
            <strong>{selectedRegion ? `${selectedRegion}行业分布` : '请选择区域'}</strong>
            <span>{selectedIndustry || '全部行业'}</span>
          </header>
          <div>
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
                <i><b style={{ width: `${Math.max(8, (item.value / maxIndustryValue) * 100)}%` }} /></i>
                <strong>{item.value}</strong>
              </button>
            ))}
          </div>
        </div>
      ) : null}

      {view !== 'map' ? (
      <div className="region-alumni-results">
        <div className="region-result-title">
          <strong>
            {selectedDistrict || selectedRegion || '请选择区域'} · {selectedIndustry || '全部行业'}人员
            <b>{visibleAlumni.length}</b>
          </strong>
          <span>点击地图筛选区域，点击人员查看完整信息</span>
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
              <button type="button" key={item.id} onClick={() => onSelectAlumni(item)}>
                <span>{item.name}</span>
                <small>{item.industry || UNKNOWN_INDUSTRY}</small>
                <em>{item.work_unit || '未填单位'}</em>
              </button>
            ))}
          </div>
        ) : (
          <div className="region-result-empty">当前区域和行业暂无匹配人员</div>
        )}
      </div>
      ) : null}

      {view !== 'industry' ? (
        <Modal
          centered
          footer={null}
          open={mapModalOpen}
          width="min(1380px, 96vw)"
          className="region-map-detail-modal"
          title={null}
          onCancel={() => setMapModalOpen(false)}
          destroyOnHidden
        >
          <div className="region-map-detail-head">
            <strong>{drillMap?.name || '地图详情'}</strong>
            <div>
              {drillMap ? (
                <Button
                  icon={<ArrowLeftOutlined />}
                  shape="circle"
                  aria-label="返回上一级"
                  title="返回上一级"
                  onClick={() => {
                    updateSelectedDistrict('');
                    if (drillParent) {
                      setDrillMap(drillParent);
                      setDrillParent(null);
                    } else {
                      setDrillMap(null);
                    }
                  }}
                />
              ) : null}
            </div>
          </div>
          <div className="region-map-detail-layout">
            <div className="region-map-detail-chart">
              <ReactECharts
                key={`detail-${drillMap?.mapName || 'base'}`}
                option={detailMapOption}
                onEvents={detailMapEvents}
                onChartReady={(chart) => {
                  detailChartRef.current = chart;
                  bindBlankMapClick(
                    chart,
                    clearDetailMapSelectionRef,
                    detailAreaClickAtRef,
                    detailInteractionUntilRef,
                  );
                }}
                notMerge
                style={{ width: '100%', height: '100%' }}
              />
              <MapHeatLegend items={detailLegendItems} />
            </div>
            <aside className="region-map-detail-aside">
              <div className="region-industry-ranks">
                <header>
                  <strong>
                    {selectedDistrict || selectedRegion
                      ? `${selectedDistrict || selectedRegion}行业分布`
                      : '请选择区域'}
                  </strong>
                  <span>{selectedIndustry || '全部行业'}</span>
                </header>
                <div>
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
                      <i><b style={{ width: `${Math.max(8, (item.value / maxIndustryValue) * 100)}%` }} /></i>
                      <strong>{item.value}</strong>
                    </button>
                  ))}
                </div>
              </div>
              <div className="region-alumni-results">
                <div className="region-result-title">
                  <strong>
                    {selectedDistrict || selectedRegion || '请选择区域'}人员
                    <b>{visibleAlumni.length}</b>
                  </strong>
                </div>
                <div className="region-person-search">
                  <Input
                    allowClear
                    prefix={<SearchOutlined />}
                    value={peopleKeyword}
                    placeholder="检索当前人员..."
                    onChange={(event) => setPeopleKeyword(event.target.value)}
                  />
                </div>
                {visibleAlumni.length ? (
                  <div className="region-person-list">
                    {visibleAlumni.map((item) => (
                      <button type="button" key={item.id} onClick={() => onSelectAlumni(item)}>
                        <span>{item.name}</span>
                        <small>{item.industry || UNKNOWN_INDUSTRY}</small>
                        <em>{item.work_unit || '未填单位'}</em>
                      </button>
                    ))}
                  </div>
                ) : (
                  <div className="region-result-empty">当前范围暂无匹配人员</div>
                )}
              </div>
            </aside>
          </div>
        </Modal>
      ) : null}
    </div>
  );
}
