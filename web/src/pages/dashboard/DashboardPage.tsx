import { type ReactNode, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import ReactECharts from 'echarts-for-react';
import {
  ArrowLeftOutlined,
  ArrowsAltOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Button, Input, Modal, Segmented, Spin, message } from 'antd';
import { alumniApi } from '../../api/alumni';
import { dashboardApi } from '../../api/dashboard';
import logoUrl from '../../assets/pspa-logo.png';
import type { AlumniProfile } from '../../types/alumni';
import { AlumniDetailModal } from './AlumniDetailModal';
import { enrichAlumniMailingAddresses, loadAllAlumni } from './dashboardAlumni';
import { DistributionAlumniModal } from './DistributionAlumniModal';
import {
  RegionIndustryExplorer,
  type MapMode,
} from './RegionIndustryExplorer';
import type {
  DashboardDimension,
  DashboardOverview,
  DistributionItem,
} from '../../types/dashboard';

const dimensions: Array<{ label: string; value: DashboardDimension }> = [
  { label: '年级', value: 'grade' },
  { label: '班级', value: 'class_name' },
  { label: '届数', value: 'cohort' },
  { label: '性别', value: 'gender' },
  { label: '专业', value: 'major' },
  { label: '培养方式', value: 'training_mode' },
  { label: '行业', value: 'industry' },
];

const FEED_WINDOW_SIZE = 20;

const dimensionFields: Record<DashboardDimension, keyof AlumniProfile> = {
  grade: 'grade',
  class_name: 'class_name',
  cohort: 'cohort',
  gender: 'gender',
  major: 'major',
  training_mode: 'training_mode',
  industry: 'industry',
};

const emptyOverview: DashboardOverview = {
  total_alumni: 0,
  total_accounts: 0,
  mobile_complete_rate: 0,
  work_unit_complete_rate: 0,
  mentor_complete_rate: 0,
};

const axisColor = 'rgba(195, 224, 255, 0.68)';
const splitLineColor = 'rgba(62, 153, 255, 0.16)';
const chartPalette = [
  '#36d7ff', // 青色
  '#ffcf67', // 金黄
  '#ff645d', // 珊瑚红
  '#31d98b', // 翠绿
  '#9d8cff', // 紫罗兰
  '#ff9f45', // 橙色
  '#5ef5f5', // 亮青
  '#ff85c0', // 粉红
  '#a8e06e', // 黄绿
  '#7eb8ff', // 天蓝
];

interface DataScreenPanelProps {
  title: string;
  subtitle?: string;
  className?: string;
  loading?: boolean;
  expandable?: boolean;
  extra?: ReactNode;
  children: (expanded: boolean) => ReactNode;
}

function toPercent(value: number) {
  return Number((value * 100).toFixed(1));
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('zh-CN').format(value);
}

function formatText(value?: string) {
  return value && value.trim() ? value : '未填';
}

function sortByNumericName(items: DistributionItem[]) {
  return [...items].sort((left, right) => {
    const leftNumber = Number.parseInt(left.name, 10);
    const rightNumber = Number.parseInt(right.name, 10);
    if (Number.isNaN(leftNumber) || Number.isNaN(rightNumber)) {
      return left.name.localeCompare(right.name, 'zh-CN');
    }
    return leftNumber - rightNumber;
  });
}

function DataScreenPanel({
  title,
  subtitle,
  className,
  loading,
  expandable = true,
  extra,
  children,
}: DataScreenPanelProps) {
  const [expanded, setExpanded] = useState(false);
  const [modalReady, setModalReady] = useState(false);

  return (
    <section className={`data-screen-panel ${className ?? ''}`}>
      <div className="data-screen-panel-head">
        <div>
          <h2>{title}</h2>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        <div className="data-screen-panel-actions">
          {extra}
          {expandable ? (
            <button
              type="button"
              className="data-screen-icon-button"
              aria-label={`放大查看${title}`}
              onClick={() => setExpanded(true)}
            >
              <ArrowsAltOutlined />
            </button>
          ) : null}
        </div>
      </div>
      <div className="data-screen-panel-body">
        {loading ? <Spin className="data-screen-spin" /> : children(false)}
      </div>
      {expandable ? <Modal
        centered
        footer={null}
        open={expanded}
        width="min(1180px, 94vw)"
        className="data-screen-modal"
        title={title}
        onCancel={() => setExpanded(false)}
        afterOpenChange={(open) => {
          if (!open) {
            setModalReady(false);
            return;
          }
          window.requestAnimationFrame(() => setModalReady(true));
        }}
        destroyOnHidden
      >
        {modalReady ? <div className="data-screen-expanded-body">{children(true)}</div> : null}
      </Modal> : null}
    </section>
  );
}

function EmptyData({ text = '暂无数据' }: { text?: string }) {
  return <div className="data-screen-empty">{text}</div>;
}

export function DashboardPage() {
  const navigate = useNavigate();
  const [overview, setOverview] = useState<DashboardOverview>(emptyOverview);
  const [dimension, setDimension] = useState<DashboardDimension>('grade');
  const [mainDistribution, setMainDistribution] = useState<DistributionItem[]>([]);
  const [alumniFeed, setAlumniFeed] = useState<AlumniProfile[]>([]);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<AlumniProfile[]>([]);
  const [initialLoading, setInitialLoading] = useState(false);
  const [mainLoading, setMainLoading] = useState(false);
  const [feedLoading, setFeedLoading] = useState(false);
  const [searchLoading, setSearchLoading] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [selectedAlumni, setSelectedAlumni] = useState<AlumniProfile | null>(null);
  const [distributionOpen, setDistributionOpen] = useState(false);
  const [distributionLoading, setDistributionLoading] = useState(false);
  const [distributionTitle, setDistributionTitle] = useState('');
  const [distributionAlumni, setDistributionAlumni] = useState<AlumniProfile[]>([]);
  const [allAlumniCache, setAllAlumniCache] = useState<AlumniProfile[] | null>(null);
  const [regionDataLoading, setRegionDataLoading] = useState(true);
  const [regionMapMode, setRegionMapMode] = useState<MapMode>('shandong');
  const [selectedMapRegion, setSelectedMapRegion] = useState('');
  const [selectedMapDistrict, setSelectedMapDistrict] = useState('');
  const [now, setNow] = useState(() => new Date());
  const [isFullscreen, setIsFullscreen] = useState(() => Boolean(document.fullscreenElement));
  const [viewportWidth, setViewportWidth] = useState(() => window.innerWidth);
  const [feedOffset, setFeedOffset] = useState(0);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(new Date()), 1000);
    return () => window.clearInterval(timer);
  }, []);

  useEffect(() => {
    const onFullscreenChange = () => setIsFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
  }, []);

  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener('resize', onResize);
    return () => window.removeEventListener('resize', onResize);
  }, []);

  useEffect(() => {
    setInitialLoading(true);
    setFeedLoading(true);
    Promise.allSettled([
      dashboardApi.overview(),
      dashboardApi.distribution('grade'),
      loadAllAlumni(),
    ])
      .then(([overviewResult, mainResult, feedResult]) => {
        if (overviewResult.status === 'fulfilled') {
          setOverview(overviewResult.value);
        } else {
          message.error(overviewResult.reason?.message || '概览数据加载失败');
        }

        if (mainResult.status === 'fulfilled') {
          setMainDistribution(sortByNumericName(mainResult.value));
        } else {
          message.error(mainResult.reason?.message || '主图数据加载失败');
        }

        if (feedResult.status === 'fulfilled') {
          setAlumniFeed(feedResult.value);
          setAllAlumniCache(feedResult.value);
          void enrichAlumniMailingAddresses(feedResult.value)
            .then((items) => {
              setAlumniFeed(items);
              setAllAlumniCache(items);
            })
            .catch(() => {
              message.warning('通讯地址加载失败，地域将仅按工作单位判定');
            })
            .finally(() => {
              setRegionDataLoading(false);
            });
        } else {
          setRegionDataLoading(false);
          message.error(feedResult.reason?.message || '校友信息加载失败');
        }
      })
      .finally(() => {
        setInitialLoading(false);
        setFeedLoading(false);
      });
  }, []);

  useEffect(() => {
    if (searchKeyword.trim() || alumniFeed.length <= FEED_WINDOW_SIZE) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      setFeedOffset((current) => (current + 1) % alumniFeed.length);
    }, 1800);
    return () => window.clearInterval(timer);
  }, [alumniFeed.length, searchKeyword]);

  useEffect(() => {
    setMainLoading(true);
    dashboardApi
      .distribution(dimension)
      .then((items) => {
        setMainDistribution(dimension === 'grade' || dimension === 'cohort' ? sortByNumericName(items) : items);
      })
      .catch((error: Error) => message.error(error.message || '分布数据加载失败'))
      .finally(() => setMainLoading(false));
  }, [dimension]);

  const accountRate = overview.total_alumni
    ? toPercent(overview.total_accounts / overview.total_alumni)
    : 0;
  const averageCompletion = toPercent(
    (overview.mobile_complete_rate + overview.work_unit_complete_rate + overview.mentor_complete_rate) / 3,
  );

  const currentTime = useMemo(
    () =>
      new Intl.DateTimeFormat('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      }).format(now),
    [now],
  );
  const isCompactChart = viewportWidth <= 1500;

  const showLineChart = dimension === 'grade' || dimension === 'cohort';

  const mainChartOption = useMemo(
    () => ({
      color: chartPalette,
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        confine: true,
        backgroundColor: 'rgba(5, 20, 46, 0.92)',
        borderColor: '#2bcfff',
        textStyle: { color: '#e8f7ff' },
      },
      grid: {
        top: isCompactChart ? 20 : 26,
        right: isCompactChart ? 6 : 10,
        bottom: mainDistribution.length > 8 ? (isCompactChart ? 38 : 46) : isCompactChart ? 20 : 26,
        left: isCompactChart ? 22 : 30,
        containLabel: true,
      },
      xAxis: {
        type: 'category',
        data: mainDistribution.map((item) => item.name),
        axisLine: { lineStyle: { color: axisColor } },
        axisTick: { show: false },
        axisLabel: {
          color: axisColor,
          interval: 0,
          rotate: mainDistribution.length > 8 ? (isCompactChart ? 34 : 28) : 0,
          fontSize: isCompactChart ? 9 : 11,
        },
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
        splitLine: { lineStyle: { color: splitLineColor } },
        axisLabel: { color: axisColor, fontSize: isCompactChart ? 11 : 12 },
      },
      series: [
        {
          name: '人数',
          type: 'bar',
          data: mainDistribution.map((item) => item.value),
          barMaxWidth: isCompactChart ? 32 : 44,
          itemStyle: {
            borderRadius: [8, 8, 0, 0],
            color: {
              type: 'linear',
              x: 0,
              y: 0,
              x2: 0,
              y2: 1,
              colorStops: [
                { offset: 0, color: '#50e5ff' },
                { offset: 0.55, color: '#1978ff' },
                { offset: 1, color: '#123a9a' },
              ],
            },
          },
        },
        ...(showLineChart
          ? [
              {
                name: '趋势',
                type: 'line' as const,
                data: mainDistribution.map((item) => item.value),
                smooth: true,
                symbolSize: isCompactChart ? 6 : 8,
                lineStyle: { width: isCompactChart ? 2 : 3, color: '#ffcf67' },
                itemStyle: { color: '#ffcf67' },
              },
            ]
          : []),
      ],
    }),
    [isCompactChart, mainDistribution, showLineChart],
  );

  const mainPieOption = useMemo(
    () => ({
      color: chartPalette,
      tooltip: {
        trigger: 'item',
        confine: true,
        backgroundColor: 'rgba(5, 20, 46, 0.92)',
        borderColor: '#2bcfff',
        textStyle: { color: '#e8f7ff' },
      },
      legend: {
        show: false,
      },
      series: [
        {
          name: '占比',
          type: 'pie',
          radius: isCompactChart ? ['22%', '37%'] : ['27%', '44%'],
          center: ['50%', isCompactChart ? '52%' : '49%'],
          avoidLabelOverlap: true,
          label: {
            color: '#eaf7ff',
            formatter: (params: { name: string; percent?: number }) =>
              (params.percent || 0) >= 2 ? `${params.name}\n${params.percent}%` : '',
            fontWeight: 800,
            fontSize: isCompactChart ? 9 : 11,
            distanceToLabelLine: 3,
          },
          labelLine: {
            length: isCompactChart ? 5 : 10,
            length2: isCompactChart ? 4 : 8,
            lineStyle: { color: 'rgba(195, 224, 255, 0.52)' },
          },
          labelLayout: {
            hideOverlap: true,
            moveOverlap: 'shiftY',
          },
          data: mainDistribution,
        },
      ],
    }),
    [isCompactChart, mainDistribution],
  );

  const kpis = [
    { label: '校友总数', value: overview.total_alumni, suffix: '人' },
    { label: '已开通账号', value: overview.total_accounts, suffix: '个' },
    { label: '账号开通率', value: accountRate, suffix: '%' },
    { label: '资料完整度', value: averageCompletion, suffix: '%' },
    { label: '手机号完整率', value: toPercent(overview.mobile_complete_rate), suffix: '%' },
    { label: '单位完整率', value: toPercent(overview.work_unit_complete_rate), suffix: '%' },
    { label: '导师完整率', value: toPercent(overview.mentor_complete_rate), suffix: '%' },
  ];

  const visibleFeed = useMemo(() => {
    if (alumniFeed.length <= FEED_WINDOW_SIZE) {
      return alumniFeed;
    }
    return Array.from(
      { length: FEED_WINDOW_SIZE },
      (_, index) => alumniFeed[(feedOffset + index) % alumniFeed.length],
    );
  }, [alumniFeed, feedOffset]);

  const runSearch = (value = searchKeyword) => {
    const keyword = value.trim();
    if (!keyword) {
      setSearchResults([]);
      return;
    }

    setSearchLoading(true);
    alumniApi
      .list({ page: 1, page_size: 100, keyword })
      .then((result) => {
        setSearchResults(result.items);
      })
      .catch((error: Error) => message.error(error.message || '校友搜索失败'))
      .finally(() => setSearchLoading(false));
  };

  const togglePageFullscreen = () => {
    if (document.fullscreenElement) {
      document
        .exitFullscreen()
        .catch(() => message.warning('退出全屏失败，请重试'));
      return;
    }

    document.documentElement.requestFullscreen().catch(() => message.warning('进入全屏失败，请重试'));
  };

  const openAlumniDetail = (item: AlumniProfile) => {
    setSelectedAlumni(item);
    setDetailOpen(true);
    setDetailLoading(true);
    alumniApi
      .detail(item.id)
      .then(setSelectedAlumni)
      .catch((error: Error) => message.error(error.message || '校友完整信息加载失败'))
      .finally(() => setDetailLoading(false));
  };

  const openDistributionAlumni = async (value: string) => {
    const dimensionLabel =
      dimensions.find((item) => item.value === dimension)?.label || '分布';
    setDistributionTitle(`${dimensionLabel}：${value}`);
    setDistributionOpen(true);
    setDistributionLoading(true);

    try {
      const profiles = allAlumniCache || (await loadAllAlumni());
      if (!allAlumniCache) {
        setAllAlumniCache(profiles);
      }
      const field = dimensionFields[dimension];
      setDistributionAlumni(
        profiles.filter((profile) => formatText(String(profile[field] || '')) === value),
      );
    } catch (error) {
      message.error((error as Error).message || '分布项校友信息加载失败');
      setDistributionAlumni([]);
    } finally {
      setDistributionLoading(false);
    }
  };

  const chartClickEvents = {
    click: (params: { name?: string }) => {
      if (params.name) {
        void openDistributionAlumni(params.name);
      }
    },
  };

  const renderAlumniRows = (items: AlumniProfile[]) =>
    items.map((item) => (
      <tr
        key={item.id}
        className="dashboard-clickable-row"
        tabIndex={0}
        onClick={() => openAlumniDetail(item)}
        onKeyDown={(event) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            openAlumniDetail(item);
          }
        }}
      >
        <td>{item.name}</td>
        <td>{formatText(item.grade)}</td>
        <td>{formatText(item.industry)}</td>
        <td>{formatText(item.work_unit)}</td>
        <td>{formatText(item.mentor)}</td>
      </tr>
    ));

  return (
    <div className="dashboard-screen">
      <header className="dashboard-screen-hero">
        <div className="dashboard-screen-brand">
          <img src={logoUrl} alt="山东大学政治学与公共管理学院" />
        </div>
        <div className="dashboard-screen-title">
          <span>山东大学政治学与公共管理学院</span>
          <h1>MPA 校友数据驾驶舱</h1>
          <p>校友规模、行业去向、年级结构与资料完整性展示</p>
        </div>
        <div className="dashboard-screen-tools">
          <span>{currentTime}</span>
          <Button icon={isFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />} onClick={togglePageFullscreen}>
            {isFullscreen ? '退出全屏' : '全屏展示'}
          </Button>
          <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/admin/alumni')}>
            返回管理面板
          </Button>
        </div>
      </header>

      <section className="dashboard-kpis" aria-label="核心指标">
        {kpis.map((item) => (
          <div className="dashboard-kpi" key={item.label}>
            <span>{item.label}</span>
            <strong>
              {formatNumber(item.value)}
              <em>{item.suffix}</em>
            </strong>
          </div>
        ))}
      </section>

      <main className="dashboard-screen-grid">
        <DataScreenPanel
          title="多维分布主图"
          subtitle="点击柱形、趋势点或环形分区查看对应校友"
          className="dashboard-main-panel"
          loading={mainLoading}
          extra={
            <Segmented
              className="dashboard-dimension-tabs"
              value={dimension}
              options={dimensions}
              onChange={(value) => setDimension(value as DashboardDimension)}
            />
          }
        >
          {(expanded) =>
            mainDistribution.length ? (
              <div className={`main-chart-layout ${expanded ? 'main-chart-layout-expanded' : ''}`}>
                <ReactECharts
                  key={`main-bar-${expanded ? 'expanded' : 'normal'}-${dimension}`}
                  option={mainChartOption}
                  className="dashboard-chart dashboard-chart-main"
                  onEvents={chartClickEvents}
                  notMerge
                />
                <ReactECharts
                  key={`main-pie-${expanded ? 'expanded' : 'normal'}-${dimension}`}
                  option={mainPieOption}
                  className="dashboard-chart dashboard-chart-pie"
                  onEvents={chartClickEvents}
                  notMerge
                />
              </div>
            ) : (
              <EmptyData />
            )
          }
        </DataScreenPanel>

        <DataScreenPanel
          title="校友地域地图"
          subtitle="山东省与全国切换，点击区域联动右侧行业和人员"
          className="dashboard-map-panel"
          expandable={false}
        >
          {(expanded) => (
            <RegionIndustryExplorer
              alumni={allAlumniCache ?? alumniFeed}
              expanded={expanded}
              loading={regionDataLoading}
              view="map"
              mapMode={regionMapMode}
              selectedRegion={selectedMapRegion}
              selectedDistrict={selectedMapDistrict}
              onMapModeChange={(mode) => {
                setRegionMapMode(mode);
                setSelectedMapDistrict('');
              }}
              onRegionChange={(region) => {
                setSelectedMapRegion(region);
                setSelectedMapDistrict('');
              }}
              onDistrictChange={setSelectedMapDistrict}
              onSelectAlumni={openAlumniDetail}
            />
          )}
        </DataScreenPanel>

        <DataScreenPanel
          title="校友信息检索"
          subtitle="按姓名、单位、职务、导师等关键词检索，点击条目查看完整信息"
          loading={feedLoading}
          className="dashboard-feed-panel"
          extra={<SearchOutlined />}
        >
          {() => (
            <div className="dashboard-alumni-search">
              <div className="dashboard-alumni-search-bar">
                <Input.Search
                  value={searchKeyword}
                  loading={searchLoading}
                  onChange={(event) => {
                    const value = event.target.value;
                    setSearchKeyword(value);
                    if (!value.trim()) {
                      setSearchResults([]);
                      setFeedOffset(0);
                    }
                  }}
                  onSearch={runSearch}
                  placeholder="输入姓名、单位、职务、导师等关键词..."
                  enterButton="搜索"
                />
              </div>
              {(searchKeyword.trim() ? searchResults : visibleFeed).length ? (
                <div className="alumni-feed dashboard-alumni-search-results">
                  <table className="dashboard-table">
                    <thead>
                      <tr>
                        <th>姓名</th>
                        <th>年级</th>
                        <th>行业</th>
                        <th>所在单位</th>
                        <th>导师</th>
                      </tr>
                    </thead>
                    <tbody
                      key={searchKeyword.trim() ? 'search-results' : `feed-${feedOffset}`}
                      className={searchKeyword.trim() ? '' : 'dashboard-feed-playing'}
                    >
                      {renderAlumniRows(searchKeyword.trim() ? searchResults : visibleFeed)}
                    </tbody>
                  </table>
                </div>
              ) : (
                <EmptyData text={searchKeyword.trim() ? '暂无匹配结果' : '暂无校友信息'} />
              )}
            </div>
          )}
        </DataScreenPanel>

        <DataScreenPanel
          title="地域与行业分布"
          subtitle="省内外、城市、区县与行业联动"
          className="dashboard-industry-panel"
        >
          {(expanded) => (
            <RegionIndustryExplorer
              alumni={allAlumniCache ?? alumniFeed}
              expanded={expanded}
              loading={regionDataLoading}
              view="industry"
              mapMode={regionMapMode}
              selectedRegion={selectedMapRegion}
              selectedDistrict={selectedMapDistrict}
              onMapModeChange={setRegionMapMode}
              onRegionChange={setSelectedMapRegion}
              onDistrictChange={setSelectedMapDistrict}
              onSelectAlumni={openAlumniDetail}
            />
          )}
        </DataScreenPanel>
      </main>

      <AlumniDetailModal
        open={detailOpen}
        loading={detailLoading}
        profile={selectedAlumni}
        onClose={() => {
          setDetailOpen(false);
          setSelectedAlumni(null);
        }}
      />
      <DistributionAlumniModal
        open={distributionOpen}
        loading={distributionLoading}
        title={distributionTitle}
        items={distributionAlumni}
        onClose={() => setDistributionOpen(false)}
        onSelect={(profile) => {
          setDistributionOpen(false);
          openAlumniDetail(profile);
        }}
      />
    </div>
  );
}
