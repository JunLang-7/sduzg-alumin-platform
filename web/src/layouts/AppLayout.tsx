import { useMemo, useState } from 'react';
import { Outlet, useLocation, useNavigate } from 'react-router-dom';
import {
  BarChartOutlined,
  IdcardOutlined,
  LogoutOutlined,
  SearchOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons';
import { Avatar, Dropdown, Input, Layout, Menu, Space, Typography } from 'antd';
import type { MenuProps } from 'antd';
import logoUrl from '../assets/pspa-logo.png';
import { useAuthStore } from '../store/authStore';
import { getDefaultPath, hasRole } from '../utils/permissions';

const { Content } = Layout;
const { Text } = Typography;

export function AppLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const user = useAuthStore((state) => state.user);
  const logout = useAuthStore((state) => state.logout);
  const [search, setSearch] = useState('');

  const menuItems = useMemo<MenuProps['items']>(() => {
    const items: MenuProps['items'] = [
      {
        key: '/alumni',
        icon: <TeamOutlined />,
        label: '校友服务',
      },
    ];

    if (hasRole(user, 'admin')) {
      items.push(
        {
          key: '/admin/dashboard',
          icon: <BarChartOutlined />,
          label: '数据大屏',
        },
        {
          key: '/admin/alumni',
          icon: <TeamOutlined />,
          label: '校友管理',
        },
      );
    }

    if (user?.role === 'alumni') {
      items.push({
        key: '/profile',
        icon: <IdcardOutlined />,
        label: '用户中心',
      });
    }

    if (hasRole(user, 'super_admin')) {
      items.push({
        key: '/admin/users',
        icon: <UserOutlined />,
        label: '管理员管理',
      });
    }

    return items;
  }, [user]);

  const selectedKeys = useMemo(() => {
    if (location.pathname.startsWith('/admin/users')) {
      return ['/admin/users'];
    }
    if (location.pathname.startsWith('/admin/dashboard')) {
      return ['/admin/dashboard'];
    }
    if (location.pathname.startsWith('/admin/alumni')) {
      return ['/admin/alumni'];
    }
    if (location.pathname.startsWith('/profile')) {
      return ['/profile'];
    }
    return ['/alumni'];
  }, [location.pathname]);

  const userMenu: MenuProps['items'] = [
    {
      key: 'password',
      label: '修改密码',
      onClick: () => navigate('/profile/password'),
    },
    {
      type: 'divider',
    },
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: async () => {
        await logout();
        navigate('/login', { replace: true });
      },
    },
  ];

  const submitSearch = () => {
    const keyword = search.trim();
    navigate(keyword ? `/alumni?keyword=${encodeURIComponent(keyword)}` : '/alumni');
  };

  const openHome = () => {
    navigate(getDefaultPath(user?.role));
  };

  return (
    <Layout className="app-shell">
      <header className="site-header">
        <div className="site-header-main">
          <button className="site-brand" type="button" onClick={openHome}>
            <img className="site-logo" src={logoUrl} alt="山东大学政治学与公共管理学院" />
            <span className="site-title" aria-hidden="true">
              <strong>山东大学</strong>
              <span>MPA校友网</span>
              <em>政管学院校友服务平台</em>
            </span>
          </button>
          <div className="site-actions">
            <Input.Search
              className="site-search"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              onSearch={submitSearch}
              enterButton="搜索"
              prefix={<SearchOutlined />}
              placeholder="搜索..."
            />
            <Dropdown menu={{ items: userMenu }} trigger={['click']}>
              <button className="user-menu" type="button">
                <Space size={8}>
                  <Avatar size={30} icon={<UserOutlined />} />
                  <span className="user-meta">
                    <Text strong>{user?.real_name || user?.account}</Text>
                    <Text>{user?.role}</Text>
                  </span>
                </Space>
              </button>
            </Dropdown>
          </div>
        </div>
        <nav className="site-nav">
          <Menu
            mode="horizontal"
            selectedKeys={selectedKeys}
            items={menuItems}
            onClick={({ key }) => navigate(key)}
          />
        </nav>
      </header>
      <Layout>
        <Content className="app-content">
          <div className="content-panel">
            <Outlet />
          </div>
        </Content>
      </Layout>
    </Layout>
  );
}
