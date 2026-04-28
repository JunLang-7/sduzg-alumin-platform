import { Navigate } from 'react-router-dom';
import { useAuthStore } from '../../store/authStore';
import { getDefaultPath } from '../../utils/permissions';

export function HomeRedirect() {
  const user = useAuthStore((state) => state.user);

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return <Navigate to={getDefaultPath(user.role)} replace />;
}
