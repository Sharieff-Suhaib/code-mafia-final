import React, { useEffect, useState } from 'react';
import { Navigate } from 'react-router-dom';

// ProtectedRoute - Simply checks if user has a valid token
// The EditorPage itself will handle game status and enable/disable the editor
const ProtectedRoute = ({ children }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const checkAuth = () => {
      const token = localStorage.getItem('token');
      if (token) {
        setIsAuthenticated(true);
      } else {
        setIsAuthenticated(false);
      }
      setLoading(false);
    };

    checkAuth();
  }, []);

  if (loading) {
    return <div style={{
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
      height: '100vh',
      backgroundColor: '#0f1525',
      color: '#ffd400',
      fontFamily: '"Press Start 2P", cursive',
      fontSize: '1.2rem'
    }}>Loading...</div>;
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" />;
  }

  return children;
};

export default ProtectedRoute;
