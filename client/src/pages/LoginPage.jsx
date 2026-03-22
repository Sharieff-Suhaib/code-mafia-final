import React, { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import axios from 'axios';
import '../styles/LoginPage.css';

function LoginPage({ setIsLoggedIn }) {
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
      localStorage.setItem('token', response.data.token);
      setIsLoggedIn(true);
      if (response.data?.message) {
        alert(response.data.message);
      }
      navigate('/');
    } catch (error) {
      alert(error.response?.data?.message || 'Login failed');
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
      <div className="login-box">
        <h1>User Login</h1>
        <p className="login-subtitle">Login to participate in Code Mafia</p>

        <input
          type="text"
          placeholder="Username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <input
          type="password"
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <button onClick={handleLogin} disabled={loading}>
          {loading ? 'Logging in...' : 'Login'}
        </button>

        <div className="login-links">
          <Link to="/admin/login" className="link">Admin Login</Link>
        </div>
      </div>
    </div>
  );
}

export default LoginPage;
