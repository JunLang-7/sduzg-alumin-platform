import { useState } from 'react';
import {
  ArrowRightOutlined,
  BellOutlined,
  SearchOutlined,
  SoundOutlined,
} from '@ant-design/icons';
import { Button, Card, Col, Input, Row, Space, Typography } from 'antd';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { getDefaultPath } from '../../utils/permissions';

const { Title, Paragraph } = Typography;

export function PublicHomePage() {
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const [keyword, setKeyword] = useState('');

  const submitSearch = () => {
    const query = keyword.trim();
    navigate(query ? `/alumni?keyword=${encodeURIComponent(query)}` : '/alumni');
  };

  const openUserEntry = () => {
    navigate(user ? getDefaultPath(user.role) : '/login');
  };

  return (
    <main className="public-page">
      <header className="public-header">
        <div className="public-header-main">
          <button className="public-brand" type="button" onClick={() => navigate('/')}>
            <span className="public-emblem">山</span>
            <span className="public-brand-text">
              <strong>山东大学</strong>
              <span>MPA校友网</span>
            </span>
          </button>
          <div className="public-tools">
            <Button className="public-login" type="primary" onClick={openUserEntry}>
              {user ? '进入平台' : '登录'}
            </Button>
            <Input.Search
              className="public-search"
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              onSearch={submitSearch}
              enterButton="搜索"
              prefix={<SearchOutlined />}
              placeholder="搜索..."
            />
          </div>
        </div>
        <nav className="public-nav">
          <button type="button" onClick={() => navigate('/')}>
            首页
          </button>
          <button type="button" onClick={() => navigate('/admin/dashboard')}>
            数据大屏
          </button>
          <button type="button" onClick={() => navigate('/alumni')}>
            校友服务
          </button>
          <button type="button" disabled>
            AI+
          </button>
          <button type="button" onClick={openUserEntry}>
            用户中心
          </button>
        </nav>
      </header>

      <section className="hero-banner">
        <div className="hero-inner">
          <span className="hero-kicker">山东大学政治学与公共管理学院</span>
          <Title>培新助力 卓越发展</Title>
          <Paragraph>MPA 校友数据管理与查询平台</Paragraph>
          <Space size={12} className="hero-actions">
            <Button type="primary" size="large" onClick={openUserEntry}>
              {user ? '进入校友平台' : '校友登录'}
            </Button>
            <Button size="large" onClick={() => navigate('/alumni')}>
              校友服务
            </Button>
          </Space>
        </div>
        <div className="hero-seal">MPA</div>
        <div className="hero-controls" aria-hidden="true">
          <span>◀</span>
          <span>▶</span>
        </div>
      </section>

      <section className="home-sections">
        <Row gutter={[28, 28]}>
          <Col xs={24} lg={16}>
            <Card className="home-card news-card" bordered={false}>
              <div className="section-title">
                <SoundOutlined />
                <h2>新闻资讯</h2>
              </div>
              <div className="news-list">
                <article>
                  <span>平台建设</span>
                  <h3>MPA 校友平台一期试点启动</h3>
                  <p>面向 MPA 校友提供资料维护、校友查询和管理端数据服务。</p>
                </article>
                <article>
                  <span>校友服务</span>
                  <h3>校友信息维护通道开放</h3>
                  <p>登录后可维护工作单位、职务、通讯地址和联系方式。</p>
                </article>
              </div>
            </Card>
          </Col>
          <Col xs={24} lg={8}>
            <Card className="home-card notice-card" bordered={false}>
              <div className="section-title">
                <BellOutlined />
                <h2>通知公告</h2>
              </div>
              <button type="button" className="notice-link" onClick={openUserEntry}>
                完善本人校友档案
                <ArrowRightOutlined />
              </button>
              <button type="button" className="notice-link" onClick={() => navigate('/alumni')}>
                查询 MPA 校友名录
                <ArrowRightOutlined />
              </button>
              <button type="button" className="notice-link" onClick={() => navigate('/admin')}>
                进入管理员后台
                <ArrowRightOutlined />
              </button>
            </Card>
          </Col>
        </Row>
      </section>
    </main>
  );
}
