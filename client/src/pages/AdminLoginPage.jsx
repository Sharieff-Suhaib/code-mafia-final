import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';
import '../styles/LoginPage.css';

function AdminLoginPage({ setIsLoggedIn }) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleLogin = async () => {
    if (!username || !password) {
      alert('Please enter both username and password');
      return;
    }

    setLoading(true);
    try {
      const baseURL = process.env.REACT_APP_SERVER_BASEAPI.replace('/api', '');
      const response = await axios.post(`${baseURL}/auth/login`, {
        username,
        password
      });

      // Store token
      localStorage.setItem('token', response.data.token);

      // Verify if user is admin
      const verifyResponse = await axios.get(`${baseURL}/auth/verify`, {
        headers: { Authorization: `Bearer ${response.data.token}` }
      });

      if (verifyResponse.data.role !== 'admin') {
        localStorage.removeItem('token');
        alert('Access denied. Admin credentials required.');
        return;
      }

      setIsLoggedIn(true);
      alert('Admin login successful');
      navigate('/admin/utils');
    } catch (error) {
      alert(error.response?.data?.message || 'Admin login failed');
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      handleLogin();
    }
  };

  return (
    <div className="login-container">
      <div className="login-box admin-login">
        <h1>Admin Login</h1>
        <p className="login-subtitle">Administrative Access Only</p>
        
        <input
          type="text"
          placeholder="Admin Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <input
          type="password"
          placeholder="Admin Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <button onClick={handleLogin} disabled={loading} className="admin-button">
          {loading ? 'Verifying...' : 'Admin Login'}
        </button>

        <div className="login-links">
          <Link to="/login" className="link">User Login</Link>
        </div>
      </div>
    </div>
  );
}

export default AdminLoginPage;

