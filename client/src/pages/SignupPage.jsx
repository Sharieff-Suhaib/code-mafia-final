import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';
import '../styles/LoginPage.css';

function SignupPage() {
  const [form, setForm] = useState({
    username: '',
    password: '',
    confirmPassword: '',
    team_name: ''
  });
  const [loading, setLoading] = useState(false);
  const navigate = useNavigate();

  const handleChange = (e) => {
    setForm({ ...form, [e.target.name]: e.target.value });
  };

  const handleSignup = async () => {
    const { username, password, confirmPassword, team_name } = form;

    if (!username || !password || !confirmPassword || !team_name) {
      alert('All fields are required');
      return;
    }

    if (password !== confirmPassword) {
      alert('Passwords do not match');
      return;
    }

    if (password.length < 6) {
      alert('Password must be at least 6 characters long');
      return;
    }

    setLoading(true);
    try {
      const token = localStorage.getItem('token');
      const baseURL = process.env.REACT_APP_SERVER_BASEAPI.replace('/api', '');
      const response = await axios.post(`${baseURL}/auth/signup`, {
        username,
        password,
        confirmPassword,
        team_name
      }, {
        headers: { Authorization: `Bearer ${token}` }
      });

      alert(response.data?.message || 'User created successfully');

      setForm({
        username: '',
        password: '',
        confirmPassword: '',
        team_name: ''
      });

    } catch (err) {
      alert('Signup failed: ' + (err.response?.data?.message || err.message));
    } finally {
      setLoading(false);
    }
  };

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') {
      handleSignup();
    }
  };

  return (
    <div className="login-container">
      <div className="login-box">
        <h1>Create New User</h1>
        <p className="login-subtitle">Admin Panel - User Registration</p>

        <input
          name="username"
          placeholder="Username"
          value={form.username}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <input
          name="password"
          type="password"
          placeholder="Password (min 6 characters)"
          value={form.password}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <input
          name="confirmPassword"
          type="password"
          placeholder="Confirm Password"
          value={form.confirmPassword}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <input
          name="team_name"
          placeholder="Team Name"
          value={form.team_name}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={loading}
        />
        <button onClick={handleSignup} disabled={loading}>
          {loading ? 'Creating User...' : 'Create User'}
        </button>

        <div className="login-links">
          <button
            onClick={() => navigate('/admin/utils')}
            className="link-button"
            disabled={loading}
          >
            Back to Admin Panel
          </button>
        </div>
      </div>
    </div>
  );
}

export default SignupPage;
