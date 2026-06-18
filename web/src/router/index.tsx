import { Navigate, createBrowserRouter } from 'react-router-dom';
import { AppLayout } from '../layouts/AppLayout';
import { RequireAuth } from './guards';
import { ForbiddenPage } from '../pages/common/ForbiddenPage';
import { NotFoundPage } from '../pages/common/NotFoundPage';
import { LoginPage } from '../pages/login/LoginPage';
import { AlumniListPage } from '../pages/alumni/AlumniListPage';
import { AlumniDetailPage } from '../pages/alumni/AlumniDetailPage';
import { ProfilePage } from '../pages/profile/ProfilePage';
import { ChangePasswordPage } from '../pages/profile/ChangePasswordPage';
import { AlumniManagementPage } from '../pages/admin/AlumniManagementPage';
import { DashboardPage } from '../pages/dashboard/DashboardPage';
import { AdminUsersPage } from '../pages/admin/AdminUsersPage';
import { PublicHomePage } from '../pages/common/PublicHomePage';

export const router = createBrowserRouter([
  {
    path: '/',
    element: <PublicHomePage />,
  },
  {
    path: '/login',
    element: <LoginPage />,
  },
  {
    element: <RequireAuth minRole="alumni" />,
    children: [
      {
        element: <AppLayout />,
        children: [
          {
            element: <RequireAuth exactRole="alumni" />,
            children: [
              {
                path: '/profile',
                element: <ProfilePage />,
              },
            ],
          },
          {
            path: '/profile/password',
            element: <ChangePasswordPage />,
          },
        ],
      },
    ],
  },
  {
    element: <RequireAuth minRole="admin" />,
    children: [
      {
        element: <AppLayout />,
        children: [
          {
            path: '/admin',
            element: <Navigate to="/admin/dashboard" replace />,
          },
          {
            path: '/admin/alumni',
            element: <AlumniManagementPage />,
          },
          {
            path: '/admin/dashboard',
            element: <DashboardPage />,
          },
          {
            path: '/alumni',
            element: <AlumniListPage />,
          },
          {
            path: '/alumni/:id',
            element: <AlumniDetailPage />,
          },
        ],
      },
    ],
  },
  {
    element: <RequireAuth minRole="super_admin" />,
    children: [
      {
        element: <AppLayout />,
        children: [
          {
            path: '/admin/users',
            element: <AdminUsersPage />,
          },
        ],
      },
    ],
  },
  {
    path: '/403',
    element: <ForbiddenPage />,
  },
  {
    path: '/404',
    element: <NotFoundPage />,
  },
  {
    path: '*',
    element: <Navigate to="/404" replace />,
  },
]);
