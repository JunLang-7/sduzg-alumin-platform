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

const emptyOverview: DashboardOverview = {
  total_alumni: 0,
  total_accounts: 0,
  mobile_complete_rate: 0,
  work_unit_complete_rate: 0,
  mentor_complete_rate: 0,
};

const axisColor = 'rgba(195, 224, 255, 0.68)';
const splitLineColor = 'rgba(62, 153, 255, 0.16)';
const chartPalette = ['#36d7ff', '#ffcf67', '#ff645d', '#31d98b', '#9d8cff', '#ff9f45'];

interface DataScreenPanelProps {
  title: string;
  subtitle?: string;
  className?: string;
  loading?: boolean;
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
          <button
            type="button"
            className="data-screen-icon-button"
            aria-label={`放大查看${title}`}
            onClick={() => setExpanded(true)}
          >
            <ArrowsAltOutlined />
          </button>
        </div>
      </div>
      <div className="data-screen-panel-body">
        {loading ? <Spin className="data-screen-spin" /> : children(false)}
      </div>
      <Modal
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
      </Modal>
    </section>
  );
}

function EmptyData({ text = '暂无数据' }: { text?: string }) {
  return <div className="data-screen-empty">{text}</div>;
}

function IndustryRankList({ items }: { items: DistributionItem[] }) {
  const maxValue = Math.max(...items.map((item) => item.value), 1);

  return (
    <div className="industry-rank-list">
      {items.slice(0, 5).map((item) => (
        <div className="industry-rank-row" key={item.name}>
          <span>{item.name}</span>
          <div className="industry-rank-track">
            <i style={{ width: `${Math.max(8, (item.value / maxValue) * 100)}%` }} />
          </div>
          <strong>{item.value}</strong>
        </div>
      ))}
    </div>
  );
}

export function DashboardPage() {
  const navigate = useNavigate();
  const [overview, setOverview] = useState<DashboardOverview>(emptyOverview);
  const [dimension, setDimension] = useState<DashboardDimension>('grade');
  const [mainDistribution, setMainDistribution] = useState<DistributionItem[]>([]);
  const [industryDistribution, setIndustryDistribution] = useState<DistributionItem[]>([]);
  const [alumniFeed, setAlumniFeed] = useState<AlumniProfile[]>([]);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searchResults, setSearchResults] = useState<AlumniProfile[]>([]);
  const [searchTotal, setSearchTotal] = useState(0);
  const [initialLoading, setInitialLoading] = useState(false);
  const [mainLoading, setMainLoading] = useState(false);
  const [feedLoading, setFeedLoading] = useState(false);
  const [searchLoading, setSearchLoading] = useState(false);
  const [now, setNow] = useState(() => new Date());
  const [isFullscreen, setIsFullscreen] = useState(() => Boolean(document.fullscreenElement));
  const [viewportWidth, setViewportWidth] = useState(() => window.innerWidth);

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
      dashboardApi.distribution('industry'),
      dashboardApi.distribution('grade'),
      alumniApi.list({ page: 1, page_size: 18 }),
    ])
      .then(([overviewResult, industryResult, mainResult, feedResult]) => {
        if (overviewResult.status === 'fulfilled') {
          setOverview(overviewResult.value);
        } else {
          message.error(overviewResult.reason?.message || '概览数据加载失败');
        }

        if (industryResult.status === 'fulfilled') {
          setIndustryDistribution(industryResult.value);
        } else {
          message.error(industryResult.reason?.message || '行业分布加载失败');
        }

        if (mainResult.status === 'fulfilled') {
          setMainDistribution(sortByNumericName(mainResult.value));
        } else {
          message.error(mainResult.reason?.message || '主图数据加载失败');
        }

        if (feedResult.status === 'fulfilled') {
          setAlumniFeed(feedResult.value.items);
        } else {
          message.error(feedResult.reason?.message || '校友信息流加载失败');
        }
      })
      .finally(() => {
        setInitialLoading(false);
        setFeedLoading(false);
      });
  }, []);

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

  const mainChartOption = useMemo(
    () => ({
      color: chartPalette,
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        backgroundColor: 'rgba(5, 20, 46, 0.92)',
        borderColor: '#2bcfff',
        textStyle: { color: '#e8f7ff' },
      },
      grid: {
        top: isCompactChart ? 22 : 30,
        right: isCompactChart ? 12 : 20,
        bottom: mainDistribution.length > 8 ? (isCompactChart ? 58 : 72) : isCompactChart ? 30 : 40,
        left: isCompactChart ? 38 : 48,
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
          rotate: mainDistribution.length > 8 ? (isCompactChart ? 40 : 32) : 0,
          fontSize: isCompactChart ? 11 : 12,
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
          barMaxWidth: isCompactChart ? 30 : 42,
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
        {
          name: '趋势',
          type: 'line',
          data: mainDistribution.map((item) => item.value),
          smooth: true,
          symbolSize: isCompactChart ? 6 : 8,
          lineStyle: { width: isCompactChart ? 2 : 3, color: '#ffcf67' },
          itemStyle: { color: '#ffcf67' },
        },
      ],
    }),
    [isCompactChart, mainDistribution],
  );

  const mainPieOption = useMemo(
    () => ({
      color: chartPalette,
      tooltip: {
        trigger: 'item',
        backgroundColor: 'rgba(5, 20, 46, 0.92)',
        borderColor: '#2bcfff',
        textStyle: { color: '#e8f7ff' },
      },
      legend: {
        type: 'scroll',
        bottom: isCompactChart ? 0 : 4,
        icon: 'circle',
        textStyle: {
          color: axisColor,
          fontWeight: 700,
          fontSize: isCompactChart ? 11 : 12,
        },
        pageIconColor: '#36d7ff',
        pageIconInactiveColor: 'rgba(195, 224, 255, 0.28)',
        pageTextStyle: { color: axisColor },
      },
      series: [
        {
          name: '占比',
          type: 'pie',
          radius: isCompactChart ? ['34%', '52%'] : ['42%', '64%'],
          center: ['50%', isCompactChart ? '35%' : '40%'],
          avoidLabelOverlap: true,
          label: {
            color: '#eaf7ff',
            formatter: '{b}\n{d}%',
            fontWeight: 800,
            fontSize: isCompactChart ? 11 : 12,
          },
          labelLine: {
            lineStyle: { color: 'rgba(195, 224, 255, 0.52)' },
          },
          data: mainDistribution,
        },
      ],
    }),
    [isCompactChart, mainDistribution],
  );

  const industryChartOption = useMemo(
    () => ({
      color: ['#31d98b'],
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
        backgroundColor: 'rgba(5, 20, 46, 0.92)',
        borderColor: '#2bcfff',
        textStyle: { color: '#e8f7ff' },
      },
      grid: { top: 12, right: 28, bottom: 24, left: 82 },
      xAxis: {
        type: 'value',
        minInterval: 1,
        splitLine: { lineStyle: { color: splitLineColor } },
        axisLabel: { color: axisColor },
      },
      yAxis: {
        type: 'category',
        data: industryDistribution.slice(0, 8).map((item) => item.name).reverse(),
        axisLine: { lineStyle: { color: axisColor } },
        axisTick: { show: false },
        axisLabel: { color: axisColor },
      },
      series: [
        {
          name: '人数',
          type: 'bar',
          data: industryDistribution.slice(0, 8).map((item) => item.value).reverse(),
          barMaxWidth: 14,
          itemStyle: {
            borderRadius: 999,
            color: {
              type: 'linear',
              x: 0,
              y: 0,
              x2: 1,
              y2: 0,
              colorStops: [
                { offset: 0, color: '#1160ff' },
                { offset: 1, color: '#2cf5a5' },
              ],
            },
          },
        },
      ],
    }),
    [industryDistribution],
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

  const runSearch = (value = searchKeyword) => {
    const keyword = value.trim();
    if (!keyword) {
      setSearchResults([]);
      setSearchTotal(0);
      return;
    }

    setSearchLoading(true);
    alumniApi
      .list({ page: 1, page_size: 8, keyword })
      .then((result) => {
        setSearchResults(result.items);
        setSearchTotal(result.total);
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

  const renderAlumniRows = (items: AlumniProfile[]) =>
    items.map((item) => (
      <tr key={item.id}>
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
                  notMerge
                />
                <ReactECharts
                  key={`main-pie-${expanded ? 'expanded' : 'normal'}-${dimension}`}
                  option={mainPieOption}
                  className="dashboard-chart dashboard-chart-pie"
                  notMerge
                />
              </div>
            ) : (
              <EmptyData />
            )
          }
        </DataScreenPanel>

        <DataScreenPanel title="行业分布" subtitle="就业与职业方向排行" loading={initialLoading}>
          {(expanded) =>
            industryDistribution.length ? (
              <div className={`industry-layout ${expanded ? 'industry-layout-expanded' : ''}`}>
                {expanded ? (
                  <ReactECharts
                    key="industry-expanded"
                    option={industryChartOption}
                    className="dashboard-chart"
                    notMerge
                  />
                ) : (
                  <IndustryRankList items={industryDistribution} />
                )}
                <div className="industry-tags">
                  {industryDistribution.slice(0, expanded ? 16 : 10).map((item, index) => (
                    <span
                      key={item.name}
                      className={`industry-tag industry-tag-${(index % 5) + 1}`}
                      style={{ fontSize: `${Math.max(13, 22 - index)}px` }}
                    >
                      {item.name}
                    </span>
                  ))}
                </div>
              </div>
            ) : (
              <EmptyData />
            )
          }
        </DataScreenPanel>

        <DataScreenPanel
          title="校友信息流"
          loading={feedLoading}
          className="dashboard-feed-panel"
        >
          {() =>
            alumniFeed.length ? (
              <div className="alumni-feed">
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
                  <tbody>{renderAlumniRows(alumniFeed)}</tbody>
                </table>
              </div>
            ) : (
              <EmptyData />
            )
          }
        </DataScreenPanel>

        <DataScreenPanel
          title="快速检索"
          subtitle="按姓名、单位、职务、导师等关键词查询"
          className="dashboard-search-panel"
          loading={searchLoading}
          extra={<SearchOutlined />}
        >
          {() => (
            <div className="dashboard-search">
              <Input.Search
                value={searchKeyword}
                onChange={(event) => setSearchKeyword(event.target.value)}
                onSearch={runSearch}
                placeholder="输入校友姓名、单位、导师..."
                enterButton="搜索"
              />
              {searchKeyword && searchTotal ? (
                <p className="dashboard-search-count">共匹配 {searchTotal} 条，展示前 8 条</p>
              ) : null}
              {searchResults.length ? (
                <table className="dashboard-table">
                  <thead>
                    <tr>
                      <th>姓名</th>
                      <th>性别</th>
                      <th>年级</th>
                      <th>导师</th>
                      <th>联系电话</th>
                    </tr>
                  </thead>
                  <tbody>
                    {searchResults.map((item) => (
                      <tr key={item.id}>
                        <td>{item.name}</td>
                        <td>{formatText(item.gender)}</td>
                        <td>{formatText(item.grade)}</td>
                        <td>{formatText(item.mentor)}</td>
                        <td>{formatText(item.mobile)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <EmptyData text={searchKeyword ? '暂无匹配结果' : '输入关键词后展示检索结果'} />
              )}
            </div>
          )}
        </DataScreenPanel>
      </main>
    </div>
  );
}
