import { useEffect } from 'react';
import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { Spin } from 'antd';
import { useAuthStore } from '../store/authStore';
import type { UserRole } from '../types/auth';
import { hasRole } from '../utils/permissions';

interface RequireAuthProps {
  minRole?: UserRole;
}

export function RequireAuth({ minRole = 'alumni' }: RequireAuthProps) {
  const location = useLocation();
  const user = useAuthStore((state) => state.user);
  const loading = useAuthStore((state) => state.loading);
  const sessionChecked = useAuthStore((state) => state.sessionChecked);
  const ensureCurrentUser = useAuthStore((state) => state.ensureCurrentUser);

  useEffect(() => {
    void ensureCurrentUser();
  }, [ensureCurrentUser]);

  if (loading || (!sessionChecked && !user)) {
    return (
      <div className="route-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace state={{ from: location }} />;
  }

  if (!hasRole(user, minRole)) {
    return <Navigate to="/403" replace />;
  }

  return <Outlet />;
}
