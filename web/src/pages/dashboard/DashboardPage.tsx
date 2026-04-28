import { useEffect, useMemo, useState } from 'react';
import ReactECharts from 'echarts-for-react';
import { Card, Col, Row, Segmented, Statistic, message } from 'antd';
import { dashboardApi } from '../../api/dashboard';
import { PageHeader } from '../../components/PageHeader';
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

function toPercent(value: number) {
  return Number((value * 100).toFixed(1));
}

export function DashboardPage() {
  const [overview, setOverview] = useState<DashboardOverview>(emptyOverview);
  const [dimension, setDimension] = useState<DashboardDimension>('grade');
  const [distribution, setDistribution] = useState<DistributionItem[]>([]);
  const [overviewLoading, setOverviewLoading] = useState(false);
  const [chartLoading, setChartLoading] = useState(false);

  useEffect(() => {
    setOverviewLoading(true);
    dashboardApi
      .overview()
      .then(setOverview)
      .catch((error: Error) => message.error(error.message || '概览数据加载失败'))
      .finally(() => setOverviewLoading(false));
  }, []);

  useEffect(() => {
    setChartLoading(true);
    dashboardApi
      .distribution(dimension)
      .then(setDistribution)
      .catch((error: Error) => message.error(error.message || '分布数据加载失败'))
      .finally(() => setChartLoading(false));
  }, [dimension]);

  const chartOption = useMemo(
    () => ({
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'shadow' },
      },
      grid: {
        top: 32,
        right: 24,
        bottom: 64,
        left: 56,
      },
      xAxis: {
        type: 'category',
        data: distribution.map((item) => item.name),
        axisLabel: {
          interval: 0,
          rotate: distribution.length > 8 ? 30 : 0,
        },
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
      },
      series: [
        {
          type: 'bar',
          data: distribution.map((item) => item.value),
          itemStyle: {
            color: '#b20d00',
          },
          barMaxWidth: 48,
        },
      ],
    }),
    [distribution],
  );

  const pieOption = useMemo(
    () => ({
      tooltip: { trigger: 'item' },
      legend: {
        bottom: 0,
        type: 'scroll',
      },
      series: [
        {
          type: 'pie',
          radius: ['42%', '68%'],
          center: ['50%', '45%'],
          data: distribution,
        },
      ],
    }),
    [distribution],
  );

  return (
    <>
      <PageHeader title="数据大屏" description="MPA 校友统计概览与多维分布" />
      <Row gutter={[16, 16]}>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={overviewLoading} className="metric-card">
            <Statistic title="校友总数" value={overview.total_alumni} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={6}>
          <Card loading={overviewLoading} className="metric-card">
            <Statistic title="已开通账号数" value={overview.total_accounts} />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={4}>
          <Card loading={overviewLoading} className="metric-card">
            <Statistic
              title="手机号完整率"
              value={toPercent(overview.mobile_complete_rate)}
              suffix="%"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={4}>
          <Card loading={overviewLoading} className="metric-card">
            <Statistic
              title="单位完整率"
              value={toPercent(overview.work_unit_complete_rate)}
              suffix="%"
            />
          </Card>
        </Col>
        <Col xs={24} sm={12} xl={4}>
          <Card loading={overviewLoading} className="metric-card">
            <Statistic
              title="导师完整率"
              value={toPercent(overview.mentor_complete_rate)}
              suffix="%"
            />
          </Card>
        </Col>
      </Row>
      <Card
        className="tool-card chart-card"
        title="分布统计"
        extra={
          <Segmented
            value={dimension}
            options={dimensions}
            onChange={(value) => setDimension(value as DashboardDimension)}
          />
        }
      >
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={15}>
            <ReactECharts option={chartOption} showLoading={chartLoading} className="chart" />
          </Col>
          <Col xs={24} xl={9}>
            <ReactECharts option={pieOption} showLoading={chartLoading} className="chart" />
          </Col>
        </Row>
      </Card>
    </>
  );
}
