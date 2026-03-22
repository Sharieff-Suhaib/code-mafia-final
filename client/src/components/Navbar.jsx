import React, { useState, useEffect } from 'react';
import { FaHome, FaCode, FaTrophy, FaSignOutAlt, FaBars, FaTimes } from 'react-icons/fa';
import '../styles/Navbar.css';
import { useNavigate } from 'react-router-dom';
import axios from 'axios';

const Navbar = () => {
  const [isNavOpen, setIsNavOpen] = useState(false);
  const [isTimerActive, setIsTimerActive] = useState(false);
  const [timeRemaining, setTimeRemaining] = useState(""); // State to store remaining time
  const navigate = useNavigate();

  // Function to calculate and set remaining time, including seconds
  const updateRemainingTime = () => {
    const now = new Date();
    const currentHour = now.getHours();
    const currentMinute = now.getMinutes();
    const currentSecond = now.getSeconds();
    const currentTimeInMinutes = currentHour * 60 + currentMinute;
    const currentTimeInSeconds = currentTimeInMinutes * 60 + currentSecond;

    const startTime = 17 * 60 * 60; // 5:00 PM in seconds
    const endTime = 24 * 60 * 60;   // 8:00 PM in seconds

    // If the current time is between 5 PM and 8 PM
    if (currentTimeInSeconds >= startTime && currentTimeInSeconds < endTime) {
      const remainingSeconds = endTime - currentTimeInSeconds;
      const remainingHours = Math.floor(remainingSeconds / 3600);
      const remainingMinutes = Math.floor((remainingSeconds % 3600) / 60);
      const remainingSecs = remainingSeconds % 60;

      setTimeRemaining(`${remainingHours}h ${remainingMinutes}m ${remainingSecs}s`); // Format time with seconds
      setIsTimerActive(true);
    } else {
      setTimeRemaining("00h 00m 00s"); // Ended
      setIsTimerActive(false);
    }
  };
  useEffect(() => {
    // Check timer status and update remaining time when the component mounts
    updateRemainingTime();

    // Optionally, check every second to keep the timer up to date
    const timerInterval = setInterval(updateRemainingTime, 1000);  // Update every second

    // Cleanup the interval when the component unmounts
    return () => clearInterval(timerInterval);
  }, []); // Empty dependency array means this runs only once on mount

  const handleLogout = async () => {
    try {
      const token = localStorage.getItem('token');
      if (!token) {
        window.location.href = '/';
        return;
      }

      const baseURL = process.env.REACT_APP_SERVER_BASEAPI.replace('/api', '');
      await axios.post(`${baseURL}/auth/logout`, {}, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      });

      localStorage.removeItem('token');
      window.location.href = '/';
    } catch (error) {
      console.error('Logout failed:', error);
      localStorage.removeItem('token');
      window.location.href = '/';
    }
  };

  const handleNavigation = (page) => {
    // Allow navigation - the EditorPage will handle game status
    navigate(page);
    setIsNavOpen(false);
  };

  const toggleNav = () => {
    setIsNavOpen((prev) => !prev);
  };

  return (
    <>
      <nav className="navbar">
        <div className="navbar-left" onClick={() => handleNavigation('/')}>
          <img src="assets/cm-logo-2.png" alt="Logo" className="logo" />
          <span className="navbar-title">CodeMafia</span>
        </div>

        <div className="navbar-links">
          <ul>
            <li onClick={() => handleNavigation('/')}>
              <FaHome className="icon" />
              <span>Home</span>
            </li>
            <li onClick={() => handleNavigation('/editor')} style={{ pointerEvents: isTimerActive ? 'auto' : 'none', color: isTimerActive ? 'inherit' : 'gray' }}>
              <FaCode className="icon" />
              <span>Editor</span>
            </li>
            <li onClick={() => handleNavigation('/leader')}>
              <FaTrophy className="icon" />
              <span>Leaderboard</span>
            </li>
          </ul>
        </div>

        {/* Display remaining time in navbar */}
        <div className="timer-display">
          {isTimerActive ? (
            <span>{timeRemaining}</span>
          ) : (
            <span>Timer Ended</span>
          )}
        </div>

        <button className="logout-button" onClick={handleLogout}>
          <FaSignOutAlt className="icon" />
          <span>Logout</span>
        </button>

        <div className="hamburger-icon" onClick={toggleNav}>
          {isNavOpen ? <FaTimes /> : <FaBars />}
        </div>
      </nav>

      {/* Mobile sidebar */}
      <div className={`mobile-sidebar ${isNavOpen ? 'open' : ''}`}>
        <ul>
          <li onClick={() => handleNavigation('/')}>
            <FaHome className="icon" />
            <span>Home</span>
          </li>
          <li onClick={() => handleNavigation('/editor')} style={{ pointerEvents: isTimerActive ? 'auto' : 'none', color: isTimerActive ? 'inherit' : 'gray' }}>
            <FaCode className="icon" />
            <span>Editor</span>
          </li>
          <li onClick={() => handleNavigation('/leader')}>
            <FaTrophy className="icon" />
            <span>Leaderboard</span>
          </li>

          <li className='logout-button' onClick={handleLogout}>
            <FaSignOutAlt className="icon" />
            <span>Logout</span>
          </li>
        </ul>
      </div>
    </>
  );
};

export default Navbar;
