import React from 'react';
import ReactDOM from 'react-dom/client';
import { ConfigProvider, App as AntApp } from 'antd';
import zhCN from 'antd/locale/zh_CN';
import { RouterProvider } from 'react-router-dom';
import 'antd/dist/reset.css';
import './styles/global.css';
import { router } from './router';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ConfigProvider
      locale={zhCN}
      theme={{
        token: {
          colorPrimary: '#b20d00',
          borderRadius: 6,
          fontFamily:
            '-apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
        },
        components: {
          Button: {
            primaryShadow: '0 8px 18px rgba(178, 13, 0, 0.18)',
          },
          Layout: {
            bodyBg: '#f7f3f1',
            headerBg: '#a90f05',
          },
          Menu: {
            itemSelectedBg: 'rgba(255, 255, 255, 0.14)',
            itemSelectedColor: '#fff',
            horizontalItemSelectedColor: '#fff',
            horizontalItemHoverColor: '#fff',
          },
        },
      }}
    >
      <AntApp>
        <RouterProvider router={router} />
      </AntApp>
    </ConfigProvider>
  </React.StrictMode>,
);
